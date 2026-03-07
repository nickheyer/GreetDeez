package ui

import (
	_ "embed"
	embedfs "embed"
	"fmt"
	"io/fs"
)

//go:embed all:default/build
var defaultFiles embedfs.FS

//go:embed all:minimal/build
var minimalFiles embedfs.FS

// BuildFS returns the embedded filesystem for the given theme.
func BuildFS(theme string) (fs.FS, error) {
	switch theme {
	case "minimal":
		return fs.Sub(minimalFiles, "minimal/build")
	case "", "default":
		return fs.Sub(defaultFiles, "default/build")
	default:
		return nil, fmt.Errorf("unknown theme %q", theme)
	}
}
