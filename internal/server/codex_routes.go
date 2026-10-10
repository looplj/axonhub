package server

import (
	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/server/middleware"
)

func registerCodexRoutes(server *Server, handlers Handlers, services Services) {
	common := []gin.HandlerFunc{
		middleware.WithIPBlocklist(services.SystemService),
		handlers.CodexCompatibility.Enabled,
		middleware.WithStrictAPIKeyConfig(services.AuthService, nil),
		middleware.WithSource(request.SourceAPI),
		middleware.WithResponseHeaders(services.RequestService),
		middleware.WithThread(server.Config.Trace, services.ThreadService),
		middleware.WithTrace(server.Config.Trace, services.TraceService),
	}
	httpGroup := server.Group("/codex", append(common, middleware.WithTimeout(server.Config.LLMRequestTimeout))...)
	httpGroup.GET("/v1/user-auth-credential/whoami", handlers.CodexCompatibility.Whoami)
	httpGroup.GET("/models", handlers.CodexCompatibility.ListModels)
	httpGroup.POST("/responses", handlers.OpenAI.CreateResponse)
	websocketGroup := server.Group("/codex", common...)
	websocketGroup.GET("/responses", handlers.OpenAI.CreateResponseWebSocket(server.Config.LLMRequestTimeout))
}
