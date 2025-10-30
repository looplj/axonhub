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

type Services struct {
	fx.In

	TraceService  *biz.TraceService
	ThreadService *biz.ThreadService
	AuthService   *biz.AuthService
}

func SetupRoutes(server *Server, handlers Handlers, client *ent.Client, services Services) {
	// Serve static frontend files
	server.NoRoute(static.Handler())

	server.Use(middleware.WithEntClient(client))
	server.Use(middleware.WithLoggingTracing(server.Config.Trace))
	server.Use(middleware.WithMetrics())

	publicGroup := server.Group("", middleware.WithTimeout(server.Config.RequestTimeout))
	{
		// Favicon API - DO NOT AUTH
		publicGroup.GET("/favicon", handlers.System.GetFavicon)
		// Health check endpoint - no authentication required
		publicGroup.GET("/health", handlers.System.Health)
	}

	unSecureAdminGroup := server.Group("/admin",
		middleware.WithTimeout(server.Config.RequestTimeout),
	)
	{
		// System Status and Initialize - DO NOT AUTH
		unSecureAdminGroup.GET("/system/status", handlers.System.GetSystemStatus)
		unSecureAdminGroup.POST("/system/initialize", handlers.System.InitializeSystem)
		// User Login - DO NOT AUTH
		unSecureAdminGroup.POST("/auth/signin", handlers.Auth.SignIn)
	}

	adminGroup := server.Group("/admin", middleware.WithJWTAuth(services.AuthService), middleware.WithProjectID())
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

	apiGroup := server.Group("/",
		middleware.WithTimeout(server.Config.LLMRequestTimeout),
		middleware.WithAPIKeyAuth(services.AuthService),
		middleware.WithSource(request.SourceAPI),
		middleware.WithThread(server.Config.Trace, services.ThreadService),
		middleware.WithTrace(server.Config.Trace, services.TraceService),
	)
	{
		apiGroup.POST("/chat/completions", handlers.OpenAI.ChatCompletion)
		apiGroup.GET("/models", handlers.OpenAI.ListModels)
	}

	openaiGroup := apiGroup.Group("/v1")
	{
		openaiGroup.POST("/chat/completions", handlers.OpenAI.ChatCompletion)
		openaiGroup.GET("/models", handlers.OpenAI.ListModels)
	}

	anthropicGroup := apiGroup.Group("/anthropic/v1")
	{
		anthropicGroup.POST("/messages", handlers.Anthropic.CreateMessage)
		anthropicGroup.GET("/models", handlers.Anthropic.ListModels)
	}
}
