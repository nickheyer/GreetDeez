package display

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os/exec"

	"github.com/nickheyer/greetdeez/pkg/binds"
	"github.com/nickheyer/greetdeez/pkg/webview"
)

type wlrOutput struct {
	Name         string      `json:"name"`
	Enabled      bool        `json:"enabled"`
	PhysicalSize wlrPhysSize `json:"physical_size"`
	Modes        []wlrMode   `json:"modes"`
}

type wlrPhysSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type wlrMode struct {
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Refresh   float64 `json:"refresh"`
	Preferred bool    `json:"preferred"`
	Current   bool    `json:"current"`
}

// Use wlr-randr to detect physical dislay info and set scale on the compositor
func ConfigureOutputScale() {
	out, err := exec.Command("wlr-randr", "--json").Output()
	if err != nil {
		slog.Warn("wlr-randr unavailable, skipping output scale configuration", "error", err)
		return
	}

	var outputs []wlrOutput
	if err := json.Unmarshal(out, &outputs); err != nil {
		slog.Error("failed to parse wlr-randr output", "error", err)
		return
	}

	for _, o := range outputs {
		if !o.Enabled || o.PhysicalSize.Width == 0 {
			continue
		}

		// Find the current (or preferred) mode's pixel width.
		var pixelWidth int
		for _, m := range o.Modes {
			if m.Current {
				pixelWidth = m.Width
				break
			}
			if m.Preferred {
				pixelWidth = m.Width
			}
		}
		if pixelWidth == 0 {
			continue
		}

		dpi := float64(pixelWidth) / (float64(o.PhysicalSize.Width) / 25.4)
		ratio := dpi / 96.0

		// Round to nearest 0.25, clamp to [1, 3].
		scale := math.Round(ratio*4) / 4
		if scale < 1 {
			scale = 1
		}
		if scale > 3 {
			scale = 3
		}

		slog.Info("output scale detection",
			"output", o.Name, "pixels", pixelWidth,
			"physical_mm", o.PhysicalSize.Width,
			"dpi", fmt.Sprintf("%.0f", dpi),
			"scale", fmt.Sprintf("%.2f", scale))

		if scale <= 1 {
			continue
		}

		cmd := exec.Command("wlr-randr", "--output", o.Name,
			"--scale", fmt.Sprintf("%g", scale))
		if err := cmd.Run(); err != nil {
			slog.Error("failed to set output scale", "output", o.Name, "error", err)
		}
	}
}

// Queries GDK for active monitors and fullscreens the webview on the best one
func Setup(w webview.WebView) {
	monitors := binds.EnumerateMonitors()

	for _, m := range monitors {
		slog.Info("GDK: monitor", "idx", m.Index, "connector", m.Connector,
			"resolution", fmt.Sprintf("%dx%d", m.Width, m.Height),
			"physical_mm", fmt.Sprintf("%dx%d", m.WidthMM, m.HeightMM))
	}

	target := selectBestMonitor(monitors)
	if target == nil {
		slog.Warn("GDK: no monitors found, falling back to default fullscreen")
		binds.Fullscreen(w.Window())
		return
	}

	slog.Info("GDK: targeting monitor", "connector", target.Connector,
		"resolution", fmt.Sprintf("%dx%d", target.Width, target.Height))
	binds.FullscreenOnMonitor(w.Window(), target.Index)
}

// Harden disables dev extras and injects user-select restrictions.
func Harden(w webview.WebView) {
	binds.HardenWebView(w.Widget())
	w.DisableContextMenu()
}

func selectBestMonitor(monitors []binds.MonitorInfo) *binds.MonitorInfo {
	if len(monitors) == 0 {
		return nil
	}

	best := &monitors[0]
	for i := 1; i < len(monitors); i++ {
		if monitors[i].Width*monitors[i].Height > best.Width*best.Height {
			best = &monitors[i]
		}
	}
	return best
}
