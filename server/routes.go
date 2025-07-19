package server

import (
	"go.uber.org/fx"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/server/api"
	"github.com/looplj/axonhub/server/middleware"
)

type Handlers struct {
	fx.In

	Graphql   *GraphqlHandler
	OpenAI    *api.OpenAIHandlers
	Anthropic *api.AnthropicHandlers
}

func SetupRoutes(server *Server, handlers Handlers, deps Dependencies) {
	// 管理员路由 - 不需要 API key 认证
	server.GET("/admin/playground", func(c *gin.Context) {
		handlers.Graphql.Playground.ServeHTTP(c.Writer, c.Request)
	})
	server.POST("/admin/graphql", func(ctx *gin.Context) {
		handlers.Graphql.Graphql.ServeHTTP(ctx.Writer, ctx.Request)
	})

	// API 路由 - 需要 API key 认证
	apiGroup := server.Group("/v1")
	apiGroup.Use(middleware.WithAPIKey(deps.Client))
	{
		apiGroup.POST("/messages", handlers.Anthropic.CreateMessage)
		apiGroup.POST("/chat/completions", handlers.OpenAI.ChatCompletion)
	}
}
