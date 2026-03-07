package ui

//go:generate sh -c "cd minimal && npm ci && npm run build"
//go:generate sh -c "cd cyber && npm ci && npm run build"

import (
	_ "embed"
	embedfs "embed"
	"fmt"
	"io/fs"
)

//go:embed all:minimal/build
var minimalFiles embedfs.FS

//go:embed all:cyber/build
var cyberFiles embedfs.FS

// BuildFS returns the embedded filesystem for the given theme.
func BuildFS(theme string) (fs.FS, error) {
	switch theme {
	case "", "cyber", "default":
		return fs.Sub(cyberFiles, "cyber/build")
	case "minimal":
		return fs.Sub(minimalFiles, "minimal/build")
	default:
		return nil, fmt.Errorf("unknown theme %q", theme)
	}
}
