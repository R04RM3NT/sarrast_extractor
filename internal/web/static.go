package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticEmbed embed.FS

// staticFS returns the embedded frontend bundle (Vite build output) rooted at
// "static". It exists whenever the bundle has been built; the SPA handler falls
// back to a placeholder page otherwise.
func staticFS() (fs.FS, error) {
	return fs.Sub(staticEmbed, "static")
}