package config

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	appName          = "greetdeez"
	systemConfigPath = "/etc/greetd/greetdeez.conf"
)

type Config struct {
	Debug    bool           `toml:"debug"`
	Window   WindowConfig   `toml:"window"`
	Display  DisplayConfig  `toml:"display"`
	Auth     AuthConfig     `toml:"auth"`
	Power    PowerConfig    `toml:"power"`
	Sessions SessionsConfig `toml:"sessions"`
	UI       UIConfig       `toml:"ui"`
}

type DisplayConfig struct {
	// Output is the connector to show the greeter on ("DP-1", "HDMI-A-1",
	// "eDP-1", ...). Empty means auto: most pixels, DPI breaking ties.
	Output string `toml:"output"`
}

type UIConfig struct {
	Path  string `toml:"path"`
	Theme string `toml:"theme"`
}

type WindowConfig struct {
	Title  string  `toml:"title"`
	Width  int     `toml:"width"`
	Height int     `toml:"height"`
	Scale  float64 `toml:"scale"`
}

type AuthConfig struct {
	TimeoutSeconds int `toml:"timeout_seconds"`
}

func (a AuthConfig) Timeout() time.Duration {
	return time.Duration(a.TimeoutSeconds) * time.Second
}

type PowerConfig struct {
	Enabled     bool     `toml:"enabled"`
	PoweroffCmd []string `toml:"poweroff_cmd"`
	RebootCmd   []string `toml:"reboot_cmd"`
	SuspendCmd  []string `toml:"suspend_cmd"`
}

type SessionDir struct {
	Path string `toml:"path"`
	Type string `toml:"type"`
}

type SessionsConfig struct {
	Dirs       []SessionDir `toml:"dirs"`
	X11Wrapper []string     `toml:"x11_wrapper"`
}

// Returns the first config path that exists
func DefaultConfigPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		p := filepath.Join(dir, appName, "greetdeez.conf")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return systemConfigPath
}

// Load layers env over file over detected defaults
func Load(path string) (Config, error) {
	cfg := detectDefaults()

	// file overrides detected defaults
	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
		slog.Info("loaded config", "path", path)
	} else if os.IsNotExist(err) {
		slog.Debug("no config file found, using detected defaults", "path", path)
	} else {
		slog.Warn("failed to read config file, using detected defaults", "path", path, "error", err)
	}

	// env wins over everything
	applyEnvOverrides(&cfg)

	return cfg, nil
}

func detectDefaults() Config {
	return Config{
		Window: WindowConfig{
			Title: "GreetDeez",
			Scale: 1.5,
		},
		Auth: AuthConfig{
			TimeoutSeconds: 30,
		},
		Power: PowerConfig{
			Enabled:     true,
			PoweroffCmd: []string{"shutdown", "-h", "now"},
			RebootCmd:   []string{"shutdown", "-r", "now"},
			SuspendCmd:  detectStandbyCmd(),
		},
		Sessions: SessionsConfig{
			Dirs:       detectSessionDirs(),
			X11Wrapper: []string{"startx", "/usr/bin/env"},
		},
	}
}

// maps GREETDEEZ_* env vars onto config
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GREETDEEZ_WINDOW_TITLE"); v != "" {
		cfg.Window.Title = v
	}
	if v := os.Getenv("GREETDEEZ_WINDOW_WIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Window.Width = n
		}
	}
	if v := os.Getenv("GREETDEEZ_WINDOW_HEIGHT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Window.Height = n
		}
	}
	if v := os.Getenv("GREETDEEZ_SCALE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Window.Scale = f
		}
	}
	if v := os.Getenv("GREETDEEZ_DISPLAY_OUTPUT"); v != "" {
		cfg.Display.Output = v
	}
	if v := os.Getenv("GREETDEEZ_AUTH_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Auth.TimeoutSeconds = n
		}
	}
	if v := os.Getenv("GREETDEEZ_POWER_ENABLED"); v != "" {
		cfg.Power.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("GREETDEEZ_UI_PATH"); v != "" {
		cfg.UI.Path = v
	}
	if v := os.Getenv("GREETDEEZ_UI_THEME"); v != "" {
		cfg.UI.Theme = v
	}
	if v := os.Getenv("GREETDEEZ_DEBUG"); v != "" {
		cfg.Debug = v == "true" || v == "1"
	}
}

// Probes the host for the available power commands
func detectStandbyCmd() []string {

	if hasCmd("systemctl") {
		return []string{"systemctl", "suspend"}
	}
	if hasCmd("loginctl") {
		return []string{"loginctl", "suspend"}
	}
	return nil
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Builds session directories from $XDG_DATA_DIRS
func detectSessionDirs() []SessionDir {
	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}

	sessTypes := [][2]string{
		{"wayland-sessions", "wayland"},
		{"xsessions", "x11"},
	}

	var dirs []SessionDir
	for _, base := range strings.Split(dataDirs, ":") {
		if base == "" {
			continue
		}
		for _, st := range sessTypes {
			dirs = append(dirs, SessionDir{
				Path: filepath.Join(base, st[0]),
				Type: st[1],
			})
		}
	}
	return dirs
}
