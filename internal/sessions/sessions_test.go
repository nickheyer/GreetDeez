package sessions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nickheyer/greetdeez/internal/config"
)

func TestParseExecString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"simple", "sway", []string{"sway"}},
		{"with args", "sway --config /etc/sway", []string{"sway", "--config", "/etc/sway"}},
		{"quoted args", `/usr/bin/session --arg "hello world" --flag`, []string{"/usr/bin/session", "--arg", "hello world", "--flag"}},
		{"escaped quote", `cmd "say \"hi\""`, []string{"cmd", `say "hi"`}},
		{"excess whitespace", "  sway   --debug  ", []string{"sway", "--debug"}},
		{"empty string", "", nil},
		{"only spaces", "   ", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseExecString(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("parseExecString(%q) = %v (len %d), want %v (len %d)", tt.in, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseExecString(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseDesktopEntry(t *testing.T) {
	root := filepath.Join(findRoot(t), "testdata", "sessions")

	t.Run("valid wayland session", func(t *testing.T) {
		s, err := parseDesktopEntry(filepath.Join(root, "wayland", "sway.desktop"), "wayland")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Name != "Sway" {
			t.Errorf("Name = %q, want %q", s.Name, "Sway")
		}
		if len(s.Cmd) != 1 || s.Cmd[0] != "sway" {
			t.Errorf("Cmd = %v, want [sway]", s.Cmd)
		}
		if s.Type != "wayland" {
			t.Errorf("Type = %q, want %q", s.Type, "wayland")
		}
	})

	t.Run("missing Exec", func(t *testing.T) {
		_, err := parseDesktopEntry(filepath.Join(root, "edge", "no-exec.desktop"), "wayland")
		if err == nil {
			t.Fatal("expected error for missing Exec, got nil")
		}
	})

	t.Run("Hidden=true", func(t *testing.T) {
		_, err := parseDesktopEntry(filepath.Join(root, "edge", "hidden.desktop"), "wayland")
		if err == nil {
			t.Fatal("expected error for Hidden=true, got nil")
		}
	})

	t.Run("NoDisplay=true", func(t *testing.T) {
		_, err := parseDesktopEntry(filepath.Join(root, "edge", "nodisplay.desktop"), "wayland")
		if err == nil {
			t.Fatal("expected error for NoDisplay=true, got nil")
		}
	})

	t.Run("quoted exec", func(t *testing.T) {
		s, err := parseDesktopEntry(filepath.Join(root, "edge", "quoted-exec.desktop"), "wayland")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Name != "QuotedExec" {
			t.Errorf("Name = %q, want %q", s.Name, "QuotedExec")
		}
		want := []string{"/usr/bin/session", "--arg", "hello world", "--flag"}
		if len(s.Cmd) != len(want) {
			t.Fatalf("Cmd = %v, want %v", s.Cmd, want)
		}
		for i := range s.Cmd {
			if s.Cmd[i] != want[i] {
				t.Errorf("Cmd[%d] = %q, want %q", i, s.Cmd[i], want[i])
			}
		}
	})
}

func TestList(t *testing.T) {
	root := filepath.Join(findRoot(t), "testdata", "sessions")

	t.Run("multiple dirs with wayland-first sort", func(t *testing.T) {
		dirs := []config.SessionDir{
			{Path: filepath.Join(root, "wayland"), Type: "wayland"},
			{Path: filepath.Join(root, "x11"), Type: "x11"},
		}
		got := List(dirs)
		if len(got) != 3 {
			t.Fatalf("got %d sessions, want 3", len(got))
		}
		// Wayland sessions should come first, alphabetically
		if got[0].Name != "Hyprland" || got[0].Type != "wayland" {
			t.Errorf("got[0] = %+v, want Hyprland/wayland", got[0])
		}
		if got[1].Name != "Sway" || got[1].Type != "wayland" {
			t.Errorf("got[1] = %+v, want Sway/wayland", got[1])
		}
		// X11 sessions after
		if got[2].Name != "i3" || got[2].Type != "x11" {
			t.Errorf("got[2] = %+v, want i3/x11", got[2])
		}
	})

	t.Run("empty dir", func(t *testing.T) {
		tmp := t.TempDir()
		dirs := []config.SessionDir{{Path: tmp, Type: "wayland"}}
		got := List(dirs)
		if len(got) != 0 {
			t.Fatalf("got %d sessions from empty dir, want 0", len(got))
		}
	})

	t.Run("missing dir", func(t *testing.T) {
		dirs := []config.SessionDir{{Path: "/nonexistent/path", Type: "wayland"}}
		got := List(dirs)
		if len(got) != 0 {
			t.Fatalf("got %d sessions from missing dir, want 0", len(got))
		}
	})

	t.Run("edge cases filtered out", func(t *testing.T) {
		dirs := []config.SessionDir{{Path: filepath.Join(root, "edge"), Type: "wayland"}}
		got := List(dirs)
		// Only quoted-exec.desktop should survive — hidden, nodisplay, no-exec are all filtered
		if len(got) != 1 {
			t.Fatalf("got %d sessions from edge dir, want 1: %+v", len(got), got)
		}
		if got[0].Name != "QuotedExec" {
			t.Errorf("got[0].Name = %q, want %q", got[0].Name, "QuotedExec")
		}
	})
}

// findRoot walks up from the working directory to find the project root (where go.mod lives).
func findRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod)")
		}
		dir = parent
	}
}
