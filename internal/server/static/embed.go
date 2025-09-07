package static

import (
	"embed"
	"strings"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

//go:embed all:dist/*
var dist embed.FS

var staticFS static.ServeFileSystem

func init() {
	var err error

	staticFS, err = static.EmbedFolder(dist, "dist")
	if err != nil {
		panic(err)
	}
}

func Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if the request is for an API route or static file
		if shouldServeStatic(c.Request.URL.Path) {
			static.Serve("/", staticFS)(c)
		} else {
			// For SPA routes, serve the index.html
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
			c.FileFromFS("/", staticFS)
		}
	}
}

// shouldServeStatic determines if a path should be served as a static file.
func shouldServeStatic(path string) bool {
	// Always serve static assets
	if strings.HasPrefix(path, "/assets/") ||
		strings.HasPrefix(path, "/images/") ||
		path == "/favicon.ico" ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".jpeg") ||
		strings.HasSuffix(path, ".gif") ||
		strings.HasSuffix(path, ".svg") ||
		strings.HasSuffix(path, ".webp") {
		return true
	}

	// Serve root path
	if path == "/" {
		return true
	}

	// Everything else is an SPA route
	return false
}
