// Package ui embeds the built web UI.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// FS returns the UI build output. It only contains a placeholder until the
// web UI is built with `make ui`.
func FS() fs.FS {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
