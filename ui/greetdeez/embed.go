package embed

import (
	_ "embed"
	embedfs "embed"
	"io/fs"
)

// Embed the built SvelteKit application
//
//go:embed all:build
var files embedfs.FS

//go:embed splash.html
var SplashHTML string

// BuildFS returns the embedded filesystem containing the built frontend
func BuildFS() (fs.FS, error) {
	return fs.Sub(files, "build")
}
