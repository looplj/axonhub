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
	"github.com/looplj/axonhub/internal/server/static"
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
	// Serve static frontend files
	server.NoRoute(static.Handler())

	server.Use(middleware.WithEntClient(client))
	server.Use(middleware.WithTracing(server.Config.Trace))
	server.Use(middleware.WithMetrics())

	publicGroup := server.Group("", middleware.WithTimeout(server.Config.RequestTimeout))
	{
		// Favicon API - DO NOT AUTH
		publicGroup.GET("/favicon", handlers.System.GetFavicon)
		publicGroup.GET("/favicon.ico", handlers.System.GetFavicon)
	}

	unSecureAdminGroup := server.Group("/admin", middleware.WithTimeout(server.Config.RequestTimeout))
	{
		// System Status and Initialize - DO NOT AUTH
		unSecureAdminGroup.GET("/system/status", handlers.System.GetSystemStatus)
		unSecureAdminGroup.POST("/system/initialize", handlers.System.InitializeSystem)
		// User Login - DO NOT AUTH
		unSecureAdminGroup.POST("/auth/signin", handlers.Auth.SignIn)
	}

	// Health check endpoint - no authentication required
	server.GET("/health", handlers.System.Health)

	adminGroup := server.Group("/admin", middleware.WithJWTAuth(auth))
	// 管理员路由 - 使用 JWT 认证
	{
		adminGroup.GET("/playground", middleware.WithTimeout(server.Config.RequestTimeout), func(c *gin.Context) {
			handlers.Graphql.Playground.ServeHTTP(c.Writer, c.Request)
		})
		adminGroup.POST("/graphql", middleware.WithTimeout(server.Config.RequestTimeout), func(c *gin.Context) {
			handlers.Graphql.Graphql.ServeHTTP(c.Writer, c.Request)
		})

		// Playground API with channel specification support
		adminGroup.POST(
			"/playground/chat",
			middleware.WithTimeout(server.Config.LLMRequestTimeout),
			middleware.WithSource(request.SourcePlayground),
			handlers.Playground.ChatCompletion,
		)
	}

	apiGroup := server.Group("/v1", middleware.WithTimeout(server.Config.LLMRequestTimeout))
	apiGroup.Use(middleware.WithAPIKeyAuth(auth))
	apiGroup.Use(middleware.WithSource(request.SourceAPI))
	{
		apiGroup.POST("/chat/completions", handlers.OpenAI.ChatCompletion)
	}

	anthropicGroup := server.Group("/anthropic/v1", middleware.WithTimeout(server.Config.LLMRequestTimeout))
	anthropicGroup.Use(middleware.WithAPIKeyAuth(auth))
	anthropicGroup.Use(middleware.WithSource(request.SourceAPI))
	{
		anthropicGroup.POST("/messages", handlers.Anthropic.CreateMessage)
	}
}
