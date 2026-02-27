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

const DefaultConfigPath = "/etc/greetd/greetdeez.conf"

type Config struct {
	Window   WindowConfig   `toml:"window"   json:"window"`
	Auth     AuthConfig     `toml:"auth"     json:"auth"`
	Power    PowerConfig    `toml:"power"    json:"power"`
	Sessions SessionsConfig `toml:"sessions" json:"sessions"`
	Theme    ThemeConfig    `toml:"theme"    json:"theme"`
}

type WindowConfig struct {
	Title  string `toml:"title"  json:"title"`
	Width  int    `toml:"width"  json:"width"`
	Height int    `toml:"height" json:"height"`
}

type AuthConfig struct {
	TimeoutSeconds int `toml:"timeout_seconds" json:"timeout_seconds"`
}

func (a AuthConfig) Timeout() time.Duration {
	return time.Duration(a.TimeoutSeconds) * time.Second
}

type PowerConfig struct {
	Enabled     bool     `toml:"enabled"      json:"enabled"`
	PoweroffCmd []string `toml:"poweroff_cmd" json:"poweroff_cmd"`
	RebootCmd   []string `toml:"reboot_cmd"   json:"reboot_cmd"`
	SuspendCmd  []string `toml:"suspend_cmd"  json:"suspend_cmd"`
}

type SessionDir struct {
	Path string `toml:"path" json:"path"`
	Type string `toml:"type" json:"type"`
}

type SessionsConfig struct {
	Dirs []SessionDir `toml:"dirs" json:"dirs"`
}

type ThemeConfig struct {
	AccentColor string  `toml:"accent_color" json:"accent_color"`
	AuroraSpeed float64 `toml:"aurora_speed"  json:"aurora_speed"`
}

// Load reads configuration with the following priority (highest wins):
//
//	environment variables (GREETDEEZ_*) > config file > detected host defaults
func Load(path string) (Config, error) {
	cfg := detectDefaults()

	// Layer 2: config file overrides detected defaults
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

	// Layer 3: env vars override everything
	applyEnvOverrides(&cfg)

	return cfg, nil
}

func detectDefaults() Config {
	w, h := detectDisplaySize()
	poweroff, reboot, suspend := detectPowerCmds()

	return Config{
		Window: WindowConfig{
			Title:  "GreetDeez",
			Width:  w,
			Height: h,
		},
		Auth: AuthConfig{
			TimeoutSeconds: 30,
		},
		Power: PowerConfig{
			Enabled:     true,
			PoweroffCmd: poweroff,
			RebootCmd:   reboot,
			SuspendCmd:  suspend,
		},
		Sessions: SessionsConfig{
			Dirs: []SessionDir{
				{Path: "/usr/share/wayland-sessions", Type: "wayland"},
				{Path: "/usr/share/xsessions", Type: "x11"},
			},
		},
		Theme: ThemeConfig{
			AccentColor: "",
			AuroraSpeed: 1.0,
		},
	}
}

// applyEnvOverrides maps GREETDEEZ_* environment variables onto the config.
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
	if v := os.Getenv("GREETDEEZ_AUTH_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Auth.TimeoutSeconds = n
		}
	}
	if v := os.Getenv("GREETDEEZ_POWER_ENABLED"); v != "" {
		cfg.Power.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("GREETDEEZ_THEME_ACCENT_COLOR"); v != "" {
		cfg.Theme.AccentColor = v
	}
	if v := os.Getenv("GREETDEEZ_THEME_AURORA_SPEED"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Theme.AuroraSpeed = f
		}
	}
}

// detectDisplaySize reads the preferred mode from the DRM subsystem.
// Falls back to 1920x1080 if detection fails.
func detectDisplaySize() (int, int) {
	matches, _ := filepath.Glob("/sys/class/drm/card*-*/modes")
	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil || len(data) == 0 {
			continue
		}
		line, _, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
		if w, h, ok := parseResolution(line); ok {
			slog.Debug("detected display size from DRM", "width", w, "height", h, "source", p)
			return w, h
		}
	}
	return 1920, 1080
}

func parseResolution(s string) (int, int, bool) {
	ws, hs, ok := strings.Cut(s, "x")
	if !ok {
		return 0, 0, false
	}
	w, err := strconv.Atoi(ws)
	if err != nil || w <= 0 {
		return 0, 0, false
	}
	h, err := strconv.Atoi(hs)
	if err != nil || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// detectPowerCmds probes the host for the available session/init manager
// and returns appropriate power commands.
func detectPowerCmds() (poweroff, reboot, suspend []string) {
	if hasCmd("loginctl") {
		return []string{"loginctl", "poweroff"},
			[]string{"loginctl", "reboot"},
			[]string{"loginctl", "suspend"}
	}
	if hasCmd("systemctl") {
		return []string{"systemctl", "poweroff"},
			[]string{"systemctl", "reboot"},
			[]string{"systemctl", "suspend"}
	}
	// Generic POSIX — suspend has no universal equivalent.
	return []string{"poweroff"}, []string{"reboot"}, nil
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
