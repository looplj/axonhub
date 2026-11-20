package static

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
	"/codex",
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

func Handler(basePath string) gin.HandlerFunc {
	index := loadSPAIndex(basePath)
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

		serveSPAIndex(c, index)
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

func loadSPAIndex(basePath string) []byte {
	file, err := staticFS.Open("/index.html")
	if err != nil {
		return nil
	}
	defer file.Close()

	index, err := io.ReadAll(file)
	if err != nil {
		return nil
	}

	// Vite emits relative asset URLs so lazy chunks resolve from their modules.
	// The entry document must use absolute URLs because it also serves deep links.
	assetPath := basePath + "/assets/"
	replacer := strings.NewReplacer(
		`src="./assets/`, `src="`+assetPath,
		`href="./assets/`, `href="`+assetPath,
		`src="/assets/`, `src="`+assetPath,
		`href="/assets/`, `href="`+assetPath,
		`href="/favicon"`, `href="`+basePath+`/favicon"`,
	)
	html := replacer.Replace(string(index))
	if basePath != "" {
		meta := `<meta name="axonhub-base-path" content="` + basePath + `">`
		html = strings.Replace(html, "<head>", "<head>\n    "+meta, 1)
	}
	return []byte(html)
}

func serveSPAIndex(c *gin.Context, index []byte) {
	// SPA routes should always reload the latest index.html.
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	if index == nil {
		c.FileFromFS("/", staticFS)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(c.Writer, c.Request, "index.html", time.Time{}, bytes.NewReader(index))
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
