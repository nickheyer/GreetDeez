package display

import (
	"fmt"
	"log/slog"

	"github.com/nickheyer/greetdeez/pkg/binds"
	"github.com/nickheyer/greetdeez/pkg/webview"
)

// Setup queries GDK for active monitors and fullscreens the webview on the best one.
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
