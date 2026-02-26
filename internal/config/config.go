package config

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const DefaultConfigPath = "/etc/greetd/greetdeez.conf"

type Config struct {
	Window   WindowConfig   `mapstructure:"window"   json:"window"`
	Auth     AuthConfig     `mapstructure:"auth"     json:"auth"`
	Power    PowerConfig    `mapstructure:"power"    json:"power"`
	Sessions SessionsConfig `mapstructure:"sessions" json:"sessions"`
	Theme    ThemeConfig    `mapstructure:"theme"    json:"theme"`
}

type WindowConfig struct {
	Title  string `mapstructure:"title"  json:"title"`
	Width  int    `mapstructure:"width"  json:"width"`
	Height int    `mapstructure:"height" json:"height"`
}

type AuthConfig struct {
	TimeoutSeconds int `mapstructure:"timeout_seconds" json:"timeout_seconds"`
}

func (a AuthConfig) Timeout() time.Duration {
	return time.Duration(a.TimeoutSeconds) * time.Second
}

type PowerConfig struct {
	Enabled     bool     `mapstructure:"enabled"      json:"enabled"`
	PoweroffCmd []string `mapstructure:"poweroff_cmd" json:"poweroff_cmd"`
	RebootCmd   []string `mapstructure:"reboot_cmd"   json:"reboot_cmd"`
	SuspendCmd  []string `mapstructure:"suspend_cmd"  json:"suspend_cmd"`
}

type SessionDir struct {
	Path string `mapstructure:"path" json:"path"`
	Type string `mapstructure:"type" json:"type"`
}

type SessionsConfig struct {
	Dirs []SessionDir `mapstructure:"dirs" json:"dirs"`
}

type ThemeConfig struct {
	AccentColor string  `mapstructure:"accent_color" json:"accent_color"`
	AuroraSpeed float64 `mapstructure:"aurora_speed"  json:"aurora_speed"`
}

// Load reads configuration with the following priority (highest wins):
//
//	environment variables (GREETDEEZ_*) > config file > detected host defaults
func Load(path string) (Config, error) {
	v := viper.New()

	setDetectedDefaults(v)

	v.SetConfigFile(path)
	v.SetConfigType("toml")

	v.SetEnvPrefix("GREETDEEZ")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Debug("no config file found, using detected defaults", "path", path)
		} else if os.IsNotExist(err) {
			slog.Debug("no config file found, using detected defaults", "path", path)
		} else {
			slog.Warn("failed to read config file, using detected defaults", "path", path, "error", err)
		}
	} else {
		slog.Info("loaded config", "path", v.ConfigFileUsed())
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func setDetectedDefaults(v *viper.Viper) {
	v.SetDefault("window.title", "GreetDeez")
	w, h := detectDisplaySize()
	v.SetDefault("window.width", w)
	v.SetDefault("window.height", h)

	v.SetDefault("auth.timeout_seconds", 30)

	v.SetDefault("power.enabled", true)
	poweroff, reboot, suspend := detectPowerCmds()
	v.SetDefault("power.poweroff_cmd", poweroff)
	v.SetDefault("power.reboot_cmd", reboot)
	v.SetDefault("power.suspend_cmd", suspend)

	v.SetDefault("sessions.dirs", []SessionDir{
		{Path: "/usr/share/wayland-sessions", Type: "wayland"},
		{Path: "/usr/share/xsessions", Type: "x11"},
	})

	v.SetDefault("theme.accent_color", "")
	v.SetDefault("theme.aurora_speed", 1.0)
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
