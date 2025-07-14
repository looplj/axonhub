package server

import (
	"go.uber.org/fx"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/server/api"
)

type Handlers struct {
	fx.In

	Graphql *GraphqlHandler
	OpenAI  *api.ChatCompletionHandlers
}

func SetupRoutes(server *Server, handlers Handlers) {
	server.GET("/playground", func(c *gin.Context) {
		handlers.Graphql.Playground.ServeHTTP(c.Writer, c.Request)
	})
	server.POST("/graphql", func(ctx *gin.Context) {
		handlers.Graphql.Graphql.ServeHTTP(ctx.Writer, ctx.Request)
	})

	server.POST("/v1/chat/completions", handlers.OpenAI.ChatCompletion)
}
