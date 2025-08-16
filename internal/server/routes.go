package server

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/api"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/gql"
	"github.com/looplj/axonhub/internal/server/middleware"
	"go.uber.org/fx"
)

type Handlers struct {
	fx.In

	Graphql   *gql.GraphqlHandler
	OpenAI    *api.OpenAIHandlers
	Anthropic *api.AnthropicHandlers
	AiSDK     *api.AiSDKHandlers
	System    *api.SystemHandlers
	Auth      *api.AuthHandlers
}

func SetupRoutes(server *Server, handlers Handlers, auth *biz.AuthService, client *ent.Client) {
	server.Use(middleware.WithEntClient(client))
	unAuthGroup := server.Group("/v1", cors.Default())
	{
		// Favicon API - 不需要认证
		unAuthGroup.GET("/favicon", handlers.System.GetFavicon)

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
		middleware.WithJWTAuth(auth),
	)
	// 管理员路由 - 使用 JWT 认证
	{
		adminGroup.OPTIONS("*any", cors.Default())
		adminGroup.GET("/playground", func(c *gin.Context) {
			handlers.Graphql.Playground.ServeHTTP(c.Writer, c.Request)
		})
		adminGroup.POST("/graphql", func(c *gin.Context) {
			handlers.Graphql.Graphql.ServeHTTP(c.Writer, c.Request)
		})
		adminGroup.POST("/v1/chat", handlers.AiSDK.ChatCompletion)
	}

	// API 路由 - 需要 API key 认证
	apiGroup := server.Group("/v1")
	apiGroup.Use(middleware.WithAPIKeyAuth(auth))
	{
		apiGroup.POST("/messages", handlers.Anthropic.CreateMessage)
		apiGroup.POST("/chat/completions", handlers.OpenAI.ChatCompletion)
	}
}
