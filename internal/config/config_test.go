package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Load with a nonexistent config file — should use detected defaults
	cfg, err := Load("/nonexistent/greetdeez.conf")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Window.Title != "GreetDeez" {
		t.Errorf("Window.Title = %q, want %q", cfg.Window.Title, "GreetDeez")
	}
	if cfg.Auth.TimeoutSeconds != 30 {
		t.Errorf("Auth.TimeoutSeconds = %d, want 30", cfg.Auth.TimeoutSeconds)
	}
	if !cfg.Power.Enabled {
		t.Error("Power.Enabled = false, want true")
	}
	if len(cfg.Sessions.Dirs) != 2 {
		t.Errorf("Sessions.Dirs length = %d, want 2", len(cfg.Sessions.Dirs))
	}
	if cfg.Theme.AuroraSpeed != 1.0 {
		t.Errorf("Theme.AuroraSpeed = %f, want 1.0", cfg.Theme.AuroraSpeed)
	}
}

func TestLoad_FileOverride(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "greetdeez.conf")

	content := `
[window]
title = "MyGreeter"

[auth]
timeout_seconds = 10

[theme]
accent_color = "#ff0000"
aurora_speed = 2.5
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Window.Title != "MyGreeter" {
		t.Errorf("Window.Title = %q, want %q", cfg.Window.Title, "MyGreeter")
	}
	if cfg.Auth.TimeoutSeconds != 10 {
		t.Errorf("Auth.TimeoutSeconds = %d, want 10", cfg.Auth.TimeoutSeconds)
	}
	if cfg.Theme.AccentColor != "#ff0000" {
		t.Errorf("Theme.AccentColor = %q, want %q", cfg.Theme.AccentColor, "#ff0000")
	}
	if cfg.Theme.AuroraSpeed != 2.5 {
		t.Errorf("Theme.AuroraSpeed = %f, want 2.5", cfg.Theme.AuroraSpeed)
	}
	// Defaults should still be populated
	if !cfg.Power.Enabled {
		t.Error("Power.Enabled = false, want true (default)")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("GREETDEEZ_WINDOW_TITLE", "EnvTitle")
	t.Setenv("GREETDEEZ_AUTH_TIMEOUT_SECONDS", "5")

	cfg, err := Load("/nonexistent/greetdeez.conf")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Window.Title != "EnvTitle" {
		t.Errorf("Window.Title = %q, want %q", cfg.Window.Title, "EnvTitle")
	}
	if cfg.Auth.TimeoutSeconds != 5 {
		t.Errorf("Auth.TimeoutSeconds = %d, want 5", cfg.Auth.TimeoutSeconds)
	}
}

func TestParseResolution(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		w, h    int
		wantOK  bool
	}{
		{"valid", "1920x1080", 1920, 1080, true},
		{"small", "800x600", 800, 600, true},
		{"bad string", "bad", 0, 0, false},
		{"zero width", "0x100", 0, 0, false},
		{"just x", "x", 0, 0, false},
		{"empty", "", 0, 0, false},
		{"negative", "-1x100", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h, ok := parseResolution(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("parseResolution(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}
			if ok && (w != tt.w || h != tt.h) {
				t.Errorf("parseResolution(%q) = (%d, %d), want (%d, %d)", tt.in, w, h, tt.w, tt.h)
			}
		})
	}
}
