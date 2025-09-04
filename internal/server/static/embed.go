package static

import (
	"embed"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

//go:embed dist/*
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
	return static.Serve("/", staticFS)
}
