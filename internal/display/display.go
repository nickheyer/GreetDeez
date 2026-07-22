package display

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/nickheyer/greetdeez/pkg/binds"
	"github.com/nickheyer/greetdeez/pkg/outputs"
	"github.com/nickheyer/greetdeez/pkg/webview"
)

// Setup queries GDK for active monitors and fullscreens the webview on the
// one selected by the shared output policy. want is the configured connector
// name, empty for auto.
func Setup(w webview.WebView, want string) {
	monitors := binds.EnumerateMonitors()

	outs := make([]outputs.Output, len(monitors))
	for i, m := range monitors {
		outs[i] = outputs.Output{
			Name: m.Connector, Width: m.Width, Height: m.Height,
			WidthMM: m.WidthMM, HeightMM: m.HeightMM,
		}
		slog.Info("GDK: monitor", "idx", m.Index, "connector", m.Connector,
			"resolution", fmt.Sprintf("%dx%d", m.Width, m.Height),
			"physical_mm", fmt.Sprintf("%dx%d", m.WidthMM, m.HeightMM))
	}

	idx := outputs.Pick(outs, want)
	if idx < 0 {
		slog.Warn("GDK: no monitors found, falling back to default fullscreen")
		binds.Fullscreen(w.Window())
		return
	}
	target := monitors[idx]
	if want != "" && !strings.EqualFold(target.Connector, want) {
		slog.Warn("GDK: configured output not connected, using auto",
			"want", want, "using", target.Connector)
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
