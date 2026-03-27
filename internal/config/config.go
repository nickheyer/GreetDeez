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
	w, h := detectDisplaySize()
	scale := detectDisplayScale()

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

// detectDisplayScale reads EDID physical dimensions and the preferred mode
// from the DRM subsystem to compute a whole-number scale factor.
// Returns 1 when detection fails or the display is standard-DPI.
func detectDisplayScale() int {
	const baseDPI = 96.0

	matches, _ := filepath.Glob("/sys/class/drm/card*-*/status")
	for _, statusPath := range matches {
		data, err := os.ReadFile(statusPath)
		if err != nil || strings.TrimSpace(string(data)) != "connected" {
			continue
		}

		dir := filepath.Dir(statusPath)

		// Read preferred resolution from modes (first line).
		modesData, err := os.ReadFile(filepath.Join(dir, "modes"))
		if err != nil || len(modesData) == 0 {
			continue
		}
		line, _, _ := strings.Cut(strings.TrimSpace(string(modesData)), "\n")
		w, _, ok := parseResolution(line)
		if !ok || w == 0 {
			continue
		}

		// Read physical size from EDID bytes 21-22 (cm).
		edid, err := os.ReadFile(filepath.Join(dir, "edid"))
		if err != nil || len(edid) < 23 {
			continue
		}

		// Validate EDID header: bytes 0-7 must be 00 FF FF FF FF FF FF 00
		edidHeader := []byte{0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00}
		if !bytesEqual(edid[:8], edidHeader) {
			continue
		}

		widthCm := int(edid[21])
		if widthCm == 0 {
			// Some displays use detailed timing descriptors instead.
			if dpi, ok := dpiFromDetailedTiming(edid, w); ok && dpi > 0 {
				scale := int(math.Round(dpi / baseDPI))
				if scale < 1 {
					scale = 1
				}
				slog.Debug("detected display scale from EDID detailed timing", "dpi", int(dpi), "scale", scale, "source", dir)
				return scale
			}
			continue
		}

		dpi := float64(w) / (float64(widthCm) / 2.54)
		scale := int(math.Round(dpi / baseDPI))
		if scale < 1 {
			scale = 1
		}
		slog.Debug("detected display scale from EDID", "dpi", int(dpi), "widthCm", widthCm, "scale", scale, "source", dir)
		return scale
	}

	return 1
}

// dpiFromDetailedTiming extracts physical width in mm from the first EDID
// detailed timing descriptor (bytes 54-71) when the base EDID image size
// fields are zero.
func dpiFromDetailedTiming(edid []byte, hPixels int) (float64, bool) {
	if len(edid) < 72 {
		return 0, false
	}
	// Detailed timing descriptor starts at byte 54.
	// Bytes 12-13 (absolute 66-67) encode horizontal/vertical image size in mm.
	// Byte 14 (absolute 68) has upper nibbles for each.
	hSizeLow := int(edid[66])
	hSizeHigh := int(edid[68]&0xF0) >> 4
	widthMm := (hSizeHigh << 8) | hSizeLow
	if widthMm == 0 {
		return 0, false
	}
	dpi := float64(hPixels) / (float64(widthMm) / 25.4)
	return dpi, true
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
