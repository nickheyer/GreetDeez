package ui

import (
	"os"
	"strings"
	"time"

	"github.com/nickheyer/greetdeez/ui/metal"
	"github.com/nickheyer/greetdeez/ui/metal/gtkwin"
)

// NativeTheme is a theme that renders the login screen itself instead
// of shipping HTML for the webview. Native themes are ordinary clients
// of the greetdeez protocol: the backend serves the rpc socket and the
// theme talks to it exactly like an external front end would.
type NativeTheme struct {
	Name string
	// Run draws the greeter until login succeeds or shutdown. output is
	// the configured connector name to render on, empty for auto. In dev
	// mode auth is unavailable and the theme should offer a quit key.
	Run func(socketPath string, timeout time.Duration, dev bool, output string) error
}

// the manifest of built-in native themes, next to the embedded webview
// themes in embed.go — the backend never references these packages
var natives = map[string]NativeTheme{
	"metal": {Name: "metal", Run: runMetal},
}

func runMetal(socketPath string, timeout time.Duration, dev bool, output string) error {
	if os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") != "" {
		return gtkwin.Run(socketPath, timeout, dev, output)
	}
	return metal.Run(socketPath, timeout, dev, output)
}

// Native returns the built-in native theme with the given name.
func Native(name string) (NativeTheme, bool) {
	t, ok := natives[strings.ToLower(name)]
	return t, ok
}
