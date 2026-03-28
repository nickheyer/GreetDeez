package config

import (
	"log/slog"
	"math"
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
	Debug    bool           `toml:"debug"`
	Window   WindowConfig   `toml:"window"`
	Auth     AuthConfig     `toml:"auth"`
	Power    PowerConfig    `toml:"power"`
	Sessions SessionsConfig `toml:"sessions"`
	UI       UIConfig       `toml:"ui"`
}

type UIConfig struct {
	Path  string `toml:"path"`
	Theme string `toml:"theme"`
}

type WindowConfig struct {
	Title  string `toml:"title"`
	Width  int    `toml:"width"`
	Height int    `toml:"height"`
	Scale  int    `toml:"scale"`
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
	w, h, scale := detectDisplay()

	return Config{
		Window: WindowConfig{
			Title:  "GreetDeez",
			Width:  w,
			Height: h,
			Scale:  scale,
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
	if v := os.Getenv("GREETDEEZ_UI_PATH"); v != "" {
		cfg.UI.Path = v
	}
	if v := os.Getenv("GREETDEEZ_UI_THEME"); v != "" {
		cfg.UI.Theme = v
	}
	if v := os.Getenv("GREETDEEZ_WINDOW_SCALE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			cfg.Window.Scale = n
		}
	}
	if v := os.Getenv("GREETDEEZ_DEBUG"); v != "" {
		cfg.Debug = v == "true" || v == "1"
	}
}

// Reads the connected display's native resolution and scale from DRM/EDID
func detectDisplay() (width, height, scale int) {
	matches, _ := filepath.Glob("/sys/class/drm/card*-*/status")
	for _, statusPath := range matches {
		data, err := os.ReadFile(statusPath)
		if err != nil || strings.TrimSpace(string(data)) != "connected" {
			continue
		}
		dir := filepath.Dir(statusPath)

		edid, err := os.ReadFile(filepath.Join(dir, "edid"))
		if err != nil || len(edid) < 72 || !validEDIDHeader(edid) {
			continue
		}

		// Prefer EDID preferred timing, fall back to modes file
		w, h, ok := resolutionFromEDID(edid)
		if !ok {
			w, h, ok = resolutionFromModes(filepath.Join(dir, "modes"))
		}
		if !ok {
			continue
		}

		s := scaleFromEDID(edid, w)

		slog.Debug("detected display", "width", w, "height", h, "scale", s, "source", dir)
		return w, h, s
	}

	return 1920, 1080, 1
}

// Extracts native res from first EDID detailed timing descriptor (bytes 54-71)
func resolutionFromEDID(edid []byte) (int, int, bool) {
	if edid[54] == 0 && edid[55] == 0 {
		return 0, 0, false
	}
	hActive := int(edid[58]>>4)<<8 | int(edid[56])
	vActive := int(edid[61]>>4)<<8 | int(edid[59])
	if hActive <= 0 || vActive <= 0 {
		return 0, 0, false
	}
	return hActive, vActive, true
}

// Reads first line of DRM modes file as fallback when EDID timing is absent
func resolutionFromModes(path string) (int, int, bool) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return 0, 0, false
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
	return parseResolution(line)
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

// Computes whole-number scale from EDID physical size (byte 21 cm, then timing mm)
func scaleFromEDID(edid []byte, hPixels int) int {
	const baseDPI = 96.0

	widthCm := int(edid[21])
	if widthCm > 0 {
		dpi := float64(hPixels) / (float64(widthCm) / 2.54)
		return max(int(math.Round(dpi/baseDPI)), 1)
	}

	// Fallback: physical size in mm from detailed timing descriptor (bytes 66-68)
	hSizeLow := int(edid[66])
	hSizeHigh := int(edid[68]&0xF0) >> 4
	widthMm := (hSizeHigh << 8) | hSizeLow
	if widthMm > 0 {
		dpi := float64(hPixels) / (float64(widthMm) / 25.4)
		return max(int(math.Round(dpi/baseDPI)), 1)
	}

	return 1
}

func validEDIDHeader(edid []byte) bool {
	return len(edid) >= 8 &&
		edid[0] == 0x00 && edid[1] == 0xFF && edid[2] == 0xFF && edid[3] == 0xFF &&
		edid[4] == 0xFF && edid[5] == 0xFF && edid[6] == 0xFF && edid[7] == 0x00
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
