package sharer

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/dist
var frontendEmbed embed.FS

// FrontendFS returns the embedded frontend filesystem.
func FrontendFS() (fs.FS, error) {
	return fs.Sub(frontendEmbed, "frontend/dist")
}
