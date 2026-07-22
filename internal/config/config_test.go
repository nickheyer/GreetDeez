package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultsWhenNoFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.conf"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Window.Scale != 1.5 {
		t.Errorf("scale default = %v want 1.5", cfg.Window.Scale)
	}
	if cfg.Auth.Timeout() != 30*time.Second {
		t.Errorf("timeout default = %v want 30s", cfg.Auth.Timeout())
	}
	if !cfg.Power.Enabled || len(cfg.Power.PoweroffCmd) == 0 {
		t.Errorf("power defaults broken: %+v", cfg.Power)
	}
}

func TestFileOverridesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greetdeez.conf")
	conf := "[ui]\ntheme = \"cyber\"\n[window]\nscale = 2.0\n[auth]\ntimeout_seconds = 5\n"
	if err := os.WriteFile(path, []byte(conf), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UI.Theme != "cyber" || cfg.Window.Scale != 2.0 || cfg.Auth.TimeoutSeconds != 5 {
		t.Errorf("file values not applied: %+v", cfg)
	}
}

func TestEnvBeatsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greetdeez.conf")
	if err := os.WriteFile(path, []byte("[ui]\ntheme = \"cyber\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GREETDEEZ_UI_THEME", "metal")
	t.Setenv("GREETDEEZ_SCALE", "1.25")
	t.Setenv("GREETDEEZ_AUTH_TIMEOUT_SECONDS", "7")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UI.Theme != "metal" || cfg.Window.Scale != 1.25 || cfg.Auth.TimeoutSeconds != 7 {
		t.Errorf("env overrides not applied: %+v", cfg)
	}
}
