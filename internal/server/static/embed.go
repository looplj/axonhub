package static

import (
	"embed"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/objects"
)

//go:embed all:dist/*
var dist embed.FS

var staticFS static.ServeFileSystem

var apiPrefixes = []string{
	"/admin",
	"/anthropic",
	"/doubao",
	"/gemini",
	"/jina",
	"/openapi",
	"/v1",
	"/v1beta",
}

func init() {
	var err error

	staticFS, err = static.EmbedFolder(dist, "dist")
	if err != nil {
		panic(err)
	}
}

func Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if isAPIPath(path) {
			serveAPINotFound(c, path)
			return
		}

		if isStaticAssetPath(path) {
			static.Serve("/", staticFS)(c)
			return
		}

		serveSPAIndex(c)
	}
}

func serveAPINotFound(c *gin.Context, path string) {
	err := fmt.Errorf("path not found: %s", path)
	_ = c.Error(err)
	c.AbortWithStatusJSON(http.StatusNotFound, objects.ErrorResponse{
		Error: objects.Error{
			Type:    http.StatusText(http.StatusNotFound),
			Message: err.Error(),
		},
	})
}

func serveSPAIndex(c *gin.Context) {
	// SPA routes should always reload the latest index.html.
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.FileFromFS("/", staticFS)
}

func isAPIPath(path string) bool {
	for _, prefix := range apiPrefixes {
		if hasPathPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// isStaticAssetPath determines if a path should be served as a static asset.
func isStaticAssetPath(path string) bool {
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

	return false
}

func hasPathPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
