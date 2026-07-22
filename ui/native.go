package ui

import (
	"strings"
	"time"

	"github.com/nickheyer/greetdeez/ui/metal"
)

// NativeTheme is a theme that renders the login screen itself instead
// of shipping HTML for the webview. Native themes are ordinary clients
// of the greetdeez protocol: the backend serves the rpc socket and the
// theme talks to it exactly like an external front end would.
type NativeTheme struct {
	Name string
	// Run draws the greeter until login succeeds or shutdown.
	Run func(socketPath string, timeout time.Duration) error
	// Snapshot renders representative frames as PNGs into dir with
	// stub data — no hardware needed. Optional.
	Snapshot func(dir string, w, h int) error
}

// the manifest of built-in native themes, next to the embedded webview
// themes in embed.go — the backend never references these packages
var natives = map[string]NativeTheme{
	"metal": {Name: "metal", Run: metal.Run, Snapshot: metal.Snapshot},
}

// Native returns the built-in native theme with the given name.
func Native(name string) (NativeTheme, bool) {
	t, ok := natives[strings.ToLower(name)]
	return t, ok
}
