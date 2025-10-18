package gateway

import (
	"embed"
	"net/http"
)

// StaticFiles is exported so it can be set from main package
// This will be embedded in the main package
var StaticFiles embed.FS

// GetStaticFS returns the static file system
func GetStaticFS() http.FileSystem {
	return http.FS(StaticFiles)
}
