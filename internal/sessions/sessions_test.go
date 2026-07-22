package sessions

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nickheyer/greetdeez/internal/config"
)

func TestParseExecString(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"sway", []string{"sway"}},
		{"env FOO=bar sway --unsupported-gpu", []string{"env", "FOO=bar", "sway", "--unsupported-gpu"}},
		{`"quoted arg" second`, []string{"quoted arg", "second"}},
		{"gnome-session --session=gnome %U", []string{"gnome-session", "--session=gnome"}},
		{"foo %f bar", []string{"foo", "bar"}},
		{"foo %%lit", []string{"foo", "%lit"}},
	}
	for _, c := range cases {
		if got := parseExecString(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseExecString(%q) = %v want %v", c.in, got, c.want)
		}
	}
}

func TestCleanSessionName(t *testing.T) {
	cases := map[string]string{
		"Plasma (Wayland)": "Plasma",
		"plasma (X11)":     "plasma",
		"GNOME on Xorg":    "GNOME",
		"Sway":             "Sway",
	}
	for in, want := range cases {
		if got := cleanSessionName(in); got != want {
			t.Errorf("cleanSessionName(%q) = %q want %q", in, got, want)
		}
	}
}

func writeDesktop(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestList(t *testing.T) {
	wayland := t.TempDir()
	x11 := t.TempDir()

	writeDesktop(t, wayland, "sway.desktop", "[Desktop Entry]\nName=Sway\nExec=sway\n")
	writeDesktop(t, wayland, "hidden.desktop", "[Desktop Entry]\nName=Ghost\nExec=ghost\nHidden=true\n")
	writeDesktop(t, wayland, "nodisplay.desktop", "[Desktop Entry]\nName=Shy\nExec=shy\nNoDisplay=true\n")
	writeDesktop(t, wayland, "missing.desktop", "[Desktop Entry]\nName=Gone\nExec=gone\nTryExec=this-binary-does-not-exist-xyz\n")
	writeDesktop(t, x11, "sway.desktop", "[Desktop Entry]\nName=Sway\nExec=sway\n")

	out := List([]config.SessionDir{
		{Path: wayland, Type: "wayland"},
		{Path: x11, Type: "x11"},
	})

	// sway twice tty appended hidden nodisplay tryexec dropped
	if len(out) != 3 {
		t.Fatalf("want 3 sessions got %+v", out)
	}
	if out[0].Name != "Sway" || out[0].Type != "wayland" {
		t.Fatalf("wayland should sort first got %+v", out[0])
	}
	if out[1].Name != "Sway" || out[1].Type != "x11" {
		t.Fatalf("want x11 sway second got %+v", out[1])
	}
	if out[2].Type != "tty" {
		t.Fatalf("want tty last got %+v", out[2])
	}
}
