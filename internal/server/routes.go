package server

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/server/api"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/gql"
	"github.com/looplj/axonhub/internal/server/middleware"
)

type Handlers struct {
	fx.In

	Graphql    *gql.GraphqlHandler
	OpenAI     *api.OpenAIHandlers
	Anthropic  *api.AnthropicHandlers
	AiSDK      *api.AiSDKHandlers
	Playground *api.PlaygroundHandlers
	System     *api.SystemHandlers
	Auth       *api.AuthHandlers
}

func SetupRoutes(server *Server, handlers Handlers, auth *biz.AuthService, client *ent.Client) {
	server.Use(middleware.WithEntClient(client))
	unAuthGroup := server.Group("/v1", middleware.WithTimeout(server.config.RequestTimeout))
	{
		// Favicon API - 不需要认证
		unAuthGroup.GET("/favicon", handlers.System.GetFavicon)

		// 系统状态和初始化 API - 不需要认证
		unAuthGroup.GET("/system/status", handlers.System.GetSystemStatus)
		unAuthGroup.POST("/system/initialize", handlers.System.InitializeSystem)
		// 用户登录 API - 不需要认证
		unAuthGroup.POST("/auth/signin", handlers.Auth.SignIn)
	}

	adminGroup := server.Group("/admin", middleware.WithJWTAuth(auth))
	// 管理员路由 - 使用 JWT 认证
	{
		adminGroup.GET("/playground", middleware.WithTimeout(server.config.RequestTimeout), func(c *gin.Context) {
			handlers.Graphql.Playground.ServeHTTP(c.Writer, c.Request)
		})
		adminGroup.POST("/graphql", middleware.WithTimeout(server.config.RequestTimeout), func(c *gin.Context) {
			handlers.Graphql.Graphql.ServeHTTP(c.Writer, c.Request)
		})

		adminGroup.POST("/v1/chat", middleware.WithTimeout(server.config.LLMRequestTimeout), middleware.WithSource(request.SourcePlayground), handlers.AiSDK.ChatCompletion)
		// Playground API with channel specification support
		adminGroup.POST("/v1/playground/chat", middleware.WithTimeout(server.config.LLMRequestTimeout), middleware.WithSource(request.SourcePlayground), handlers.Playground.ChatCompletion)
	}

	apiGroup := server.Group("/v1", middleware.WithTimeout(server.config.LLMRequestTimeout))
	apiGroup.Use(middleware.WithAPIKeyAuth(auth))
	apiGroup.Use(middleware.WithSource(request.SourceAPI))
	{
		apiGroup.POST("/messages", handlers.Anthropic.CreateMessage)
		apiGroup.POST("/chat/completions", handlers.OpenAI.ChatCompletion)
	}
}
