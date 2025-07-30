package server

import (
	"go.uber.org/fx"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/server/api"
	"github.com/looplj/axonhub/server/middleware"
)

type Handlers struct {
	fx.In

	Graphql   *GraphqlHandler
	OpenAI    *api.OpenAIHandlers
	Anthropic *api.AnthropicHandlers
	AiSDK     *api.AiSDKHandlers
	System    *api.SystemHandlers
	Auth      *api.AuthHandlers
}

func SetupRoutes(server *Server, handlers Handlers, deps Dependencies) {
	unAuthGroup := server.Group("/v1", cors.Default())
	{
		unAuthGroup.OPTIONS("*any", cors.Default())
		// 系统状态和初始化 API - 不需要认证
		unAuthGroup.GET("/system/status", handlers.System.GetSystemStatus)
		unAuthGroup.POST("/system/initialize", handlers.System.InitializeSystem)
		// 用户登录 API - 不需要认证
		unAuthGroup.POST("/auth/signin", handlers.Auth.SignIn)
	}
	adminCorsConfig := cors.DefaultConfig()
	adminCorsConfig.AllowAllOrigins = true
	adminCorsConfig.AddAllowHeaders("Authorization")
	adminGroup := server.Group("/admin",
		cors.New(adminCorsConfig),
		middleware.WithJWTAuth(deps.AuthService),
	)
	// 管理员路由 - 不需要 API key 认证
	{
		adminGroup.OPTIONS("*any", cors.Default())
		adminGroup.GET("/playground", func(c *gin.Context) {
			handlers.Graphql.Playground.ServeHTTP(c.Writer, c.Request)
		})
		adminGroup.POST("/graphql", func(ctx *gin.Context) {
			handlers.Graphql.Graphql.ServeHTTP(ctx.Writer, ctx.Request)
		})
		// OpenAI 兼容 API for admin playground
		adminGroup.POST("/v1/chat", handlers.AiSDK.ChatCompletion)
	}

	// API 路由 - 需要 API key 认证
	apiGroup := server.Group("/v1")
	apiGroup.Use(middleware.WithAPIKeyAuth(deps.Client))
	{
		apiGroup.POST("/messages", handlers.Anthropic.CreateMessage)
		apiGroup.POST("/chat/completions", handlers.OpenAI.ChatCompletion)
	}
}
