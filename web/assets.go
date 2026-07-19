// Package webassets exposes the production dashboard bundled into the Go
// executable. The dist placeholder keeps ordinary Go builds valid on a clean
// checkout; release builds populate the directory with Vite first.
package webassets

import (
	"embed"
	"io/fs"
)

// files contains web/dist as it existed when the Go binary was built.
//
//go:embed all:dist
var files embed.FS

// Dist returns a filesystem rooted at the Vite output directory.
func Dist() fs.FS {
	dist, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return dist
}
