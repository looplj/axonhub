package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/server/api"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/gql"
	"github.com/looplj/axonhub/internal/server/gql/openapi"
	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/looplj/axonhub/internal/server/plugin"
	"github.com/looplj/axonhub/internal/server/static"
)

type Handlers struct {
	fx.In

	Graphql            *gql.GraphqlHandler
	OpenAPIGraphql     *openapi.GraphqlHandler
	OpenAI             *api.OpenAIHandlers
	Doubao             *api.DoubaoHandlers
	Anthropic          *api.AnthropicHandlers
	Gemini             *api.GeminiHandlers
	AiSDK              *api.AiSDKHandlers
	Playground         *api.PlaygroundHandlers
	System             *api.SystemHandlers
	Auth               *api.AuthHandlers
	Invitation         *api.InvitationHandlers
	Jina               *api.JinaHandlers
	TypeSafe           *api.TypeSafeHandlers
	Codex              *api.CodexHandlers
	CodexCompatibility *api.CodexCompatibilityHandlers
	XAI                *api.XAIHandlers
	ClaudeCode         *api.ClaudeCodeHandlers
	Antigravity        *api.AntigravityHandlers
	Copilot            *api.CopilotHandlers
	RequestContent     *api.RequestContentHandlers
	OIDC               *api.OIDCHandlers
	RequestPreview     *api.RequestPreviewHandlers
}

type Services struct {
	fx.In

	TraceService   *biz.TraceService
	ThreadService  *biz.ThreadService
	AuthService    *biz.AuthService
	SystemService  *biz.SystemService
	RequestService *biz.RequestService
	PluginManager  *plugin.Manager
}

func SetupRoutes(server *Server, handlers Handlers, client *ent.Client, services Services, ipAccessControl *middleware.IPAccessControlConfig) {
	// IP 访问控制 - 全局最优先，拦截所有请求包括静态文件
	server.Use(middleware.WithIPAccessControl(ipAccessControl))

	// Serve static frontend files
	server.NoRoute(static.Handler())

	server.Use(middleware.AccessLog())
	server.Use(middleware.WithEntClient(client))
	server.Use(middleware.WithLoggingTracing(server.Config.Trace))
	server.Use(middleware.WithMetrics())

	// Setup CORS middleware at server level if enabled
	if server.Config.CORS.Enabled {
		corsConfig := cors.DefaultConfig()
		corsConfig.AllowOrigins = server.Config.CORS.AllowedOrigins
		corsConfig.AllowMethods = server.Config.CORS.AllowedMethods
		corsConfig.AllowHeaders = server.Config.CORS.AllowedHeaders
		corsConfig.ExposeHeaders = server.Config.CORS.ExposedHeaders
		corsConfig.AllowCredentials = server.Config.CORS.AllowCredentials
		corsConfig.MaxAge = server.Config.CORS.MaxAge

		corsHandler := cors.New(corsConfig)
		// 插件 UI 使用自己的匿名 CORS，原因见 skipCredentialedCORS。
		credentialedCORS := skipCredentialedCORS(corsHandler)
		server.Use(credentialedCORS)
		server.OPTIONS("*any", credentialedCORS)
	}

	publicGroup := server.Group("", middleware.WithTimeout(server.Config.RequestTimeout))
	{
		// Favicon API - DO NOT AUTH
		publicGroup.GET("/favicon", handlers.System.GetFavicon)
		// Health check endpoint - no authentication required
		publicGroup.GET("/health", handlers.System.Health)
		publicGroup.GET("/auth/invitations/:token", handlers.Invitation.Get)
		publicGroup.POST("/auth/invitations/:token/register", handlers.Invitation.Register)
	}

	unSecureAdminGroup := server.Group(adminGroupPath, middleware.WithTimeout(server.Config.RequestTimeout))
	{
		// System Status and Initialize - DO NOT AUTH
		unSecureAdminGroup.GET("/system/status", handlers.System.GetSystemStatus)
		unSecureAdminGroup.POST("/system/initialize", handlers.System.InitializeSystem)
		// User Login - DO NOT AUTH
		unSecureAdminGroup.POST("/auth/signin", handlers.Auth.SignIn)
		unSecureAdminGroup.POST("/auth/refresh", handlers.Auth.Refresh)
		// Desktop browser handoff. The one-time code is the credential.
		// The owner JWT is written into the page and never put in this URL.
		registerDesktopLoginRoute(unSecureAdminGroup)
		// The UI token is the credential and only GET/HEAD resources are routed.
		// Other methods fall through to the default 404 handler. OPTIONS is not
		// registered here: a root-level OPTIONS("*any") catch-all exists when
		// CORS is enabled, and Gin panics if a second wildcard is added to that
		// tree. That catch-all skips this path; see skipCredentialedCORS.
		registerPluginUIRoutes(unSecureAdminGroup, services.PluginManager, services.SystemService)
	}

	oauthGroup := server.Group("/oauth", middleware.WithTimeout(server.Config.RequestTimeout))
	{
		handlers.OIDC.RegisterRoutes(oauthGroup)
	}

	adminGroup := server.Group(adminGroupPath, middleware.WithJWTAuth(services.AuthService), middleware.WithProjectID())
	// 管理员路由 - 使用 JWT 认证
	{
		adminGroup.GET("/playground", middleware.WithTimeout(server.Config.RequestTimeout), func(c *gin.Context) {
			handlers.Graphql.Playground.ServeHTTP(c.Writer, c.Request)
		})
		adminGroup.POST("/invitations", handlers.Invitation.Create)

		adminGroup.POST("/codex/oauth/start", handlers.Codex.StartOAuth)
		adminGroup.POST("/codex/oauth/exchange", handlers.Codex.Exchange)
		adminGroup.POST("/codex/auth/decode", handlers.Codex.DecodeAuthJSON)
		adminGroup.POST("/xai/oauth/start", handlers.XAI.StartOAuth)
		adminGroup.POST("/xai/oauth/exchange", handlers.XAI.Exchange)
		adminGroup.POST("/xai/oauth/sso", handlers.XAI.DecodeSSO)

		adminGroup.POST("/claudecode/oauth/start", handlers.ClaudeCode.StartOAuth)
		adminGroup.POST("/claudecode/oauth/exchange", handlers.ClaudeCode.Exchange)

		adminGroup.POST("/antigravity/oauth/start", handlers.Antigravity.StartOAuth)
		adminGroup.POST("/antigravity/oauth/exchange", handlers.Antigravity.Exchange)

		adminGroup.POST("/copilot/oauth/start", handlers.Copilot.StartOAuth)
		adminGroup.POST("/copilot/oauth/poll", handlers.Copilot.PollOAuth)

		// OIDC Manual Linking
		adminGroup.GET("/oidc/link/:provider", handlers.OIDC.GetLinkAuthorizeURL)

		// Playground API with channel specification support
		adminGroup.POST(
			"/playground/chat",
			middleware.WithTimeout(server.Config.LLMRequestTimeout),
			middleware.WithSource(request.SourcePlayground),
			middleware.WithResponseHeaders(services.RequestService),
			handlers.Playground.ChatCompletion,
		)

		adminGroup.GET(
			"/requests/:request_id/content",
			middleware.WithTimeout(server.Config.RequestTimeout),
			handlers.RequestContent.DownloadRequestContent,
		)
		adminGroup.GET(
			"/requests/:request_id/preview",
			middleware.WithTimeout(server.Config.RequestTimeout),
			handlers.RequestPreview.PreviewRequest,
		)
	}

	// Admin GraphQL accepts both the admin UI's JWT and service_account API keys.
	// Service account principals are read-only; see middleware.WithAdminGraphqlAuth
	// and gql.apiKeyReadOnly.
	adminGraphqlGroup := server.Group(
		"/admin",
		middleware.WithAdminGraphqlAuth(services.AuthService),
		middleware.WithProjectID(),
	)
	{
		adminGraphqlGroup.POST("/graphql", middleware.WithTimeout(server.Config.RequestTimeout), func(c *gin.Context) {
			handlers.Graphql.Graphql.ServeHTTP(c.Writer, c.Request)
		})
	}

	// Page routes accept a user JWT only. Service-account keys stay on
	// /admin/graphql. GraphQL keeps its 10s budget. Fetch uses the 15s
	// bridge budget. Neither route has a rate limiter.
	pluginGroup := server.Group(
		"/admin/plugins",
		middleware.WithJWTAuth(services.AuthService),
		middleware.WithProjectID(),
	)
	{
		pluginGroup.POST("/:pluginId/graphql", middleware.WithTimeout(gql.PluginGraphqlTimeout), func(c *gin.Context) {
			handlers.Graphql.PluginGraphql.ServeHTTP(c.Writer, c.Request)
		})
		pluginGroup.POST("/:pluginId/fetch", middleware.WithTimeout(gql.PluginFetchTimeout), func(c *gin.Context) {
			handlers.Graphql.PluginFetch.ServeHTTP(c.Writer, c.Request)
		})
	}

	openAPIGroup := server.Group(
		"/openapi",
		middleware.WithIPBlocklist(services.SystemService),
		middleware.WithOpenAPIAuth(services.AuthService),
		middleware.WithTimeout(server.Config.RequestTimeout),
	)
	{
		openAPIGroup.POST("/v1/graphql", func(c *gin.Context) {
			handlers.OpenAPIGraphql.Graphql.ServeHTTP(c.Writer, c.Request)
		})
		openAPIGroup.GET("/v1/playground", func(c *gin.Context) {
			handlers.OpenAPIGraphql.Playground.ServeHTTP(c.Writer, c.Request)
		})

		openAPIGroup.POST("/webhook/echo", handlers.System.WebhookEcho)
	}

	registerCodexRoutes(server, handlers, services)

	apiMiddlewares := []gin.HandlerFunc{
		middleware.WithIPBlocklist(services.SystemService),
		middleware.WithAPIKeyConfig(services.AuthService, nil),
		middleware.WithSource(request.SourceAPI),
		middleware.WithResponseHeaders(services.RequestService),
		middleware.WithThread(server.Config.Trace, services.ThreadService),
		middleware.WithTrace(server.Config.Trace, services.TraceService),
	}
	apiGroup := server.Group("/", append([]gin.HandlerFunc{
		middleware.WithTimeout(server.Config.LLMRequestTimeout),
	}, apiMiddlewares...)...)

	// WebSocket mode owns a long-lived connection and applies its processing
	// timeout per response.create event, so it must not inherit the ordinary
	// single-request timeout from apiGroup.
	responsesWebSocketGroup := server.Group("/", apiMiddlewares...)
	responsesWebSocketGroup.GET("/v1/responses", handlers.OpenAI.CreateResponseWebSocket(server.Config.LLMRequestTimeout))

	{
		openaiGroup := apiGroup.Group("/v1")
		openaiGroup.POST("/chat/completions", handlers.OpenAI.ChatCompletion)
		openaiGroup.POST("/completions", handlers.OpenAI.Completion)
		openaiGroup.POST("/responses/compact", handlers.OpenAI.CompactResponse)
		openaiGroup.POST("/responses", handlers.OpenAI.CreateResponse)
		openaiGroup.GET("/models", handlers.OpenAI.ListModels)
		openaiGroup.GET("/models/*model", handlers.OpenAI.RetrieveModel)
		openaiGroup.POST("/embeddings", handlers.OpenAI.CreateEmbedding)
		openaiGroup.POST("/moderations", handlers.OpenAI.CreateModeration)
		openaiGroup.POST("/alpha/search", handlers.OpenAI.CreateAlphaSearch)
		openaiGroup.POST("/decisions", handlers.OpenAI.CreateDecisions)
		openaiGroup.POST("/images/generations", handlers.OpenAI.CreateImage)
		openaiGroup.POST("/images/edits", handlers.OpenAI.CreateImageEdit)
		openaiGroup.POST("/videos", handlers.OpenAI.CreateVideo)
		openaiGroup.GET("/videos/:id", handlers.OpenAI.GetVideo)
		openaiGroup.DELETE("/videos/:id", handlers.OpenAI.DeleteVideo)
		openaiGroup.POST("/audio/speech", handlers.OpenAI.CreateSpeech)
		openaiGroup.POST("/audio/transcriptions", handlers.OpenAI.CreateTranscription)
		openaiGroup.POST("/audio/translations", handlers.OpenAI.CreateTranslation)
		// DO NOT SUPPORT IMAGE VARIATION
		// openaiGroup.POST("/images/variations", handlers.OpenAI.CreateImageVariation)

		// OpenAI-compatible Anthropic endpoint
		openaiGroup.POST("/messages", handlers.Anthropic.CreateMessage)

		// Compatible with OpenAI API
		openaiGroup.POST("/rerank", handlers.Jina.Rerank)

		// Native System One endpoint
		openaiGroup.POST("/systemone", handlers.TypeSafe.SystemOne)
	}

	{
		jinaGroup := apiGroup.Group("/jina/v1")
		jinaGroup.POST("/embeddings", handlers.Jina.CreateEmbedding)
		jinaGroup.POST("/rerank", handlers.Jina.Rerank)
	}

	{
		typesafeGroup := apiGroup.Group("/typesafe/v1")
		typesafeGroup.POST("/systemone", handlers.TypeSafe.SystemOne)
	}

	{
		anthropicGroup := apiGroup.Group("/anthropic/v1")
		anthropicGroup.POST("/messages", handlers.Anthropic.CreateMessage)
		anthropicGroup.GET("/models", handlers.Anthropic.ListModels)
	}

	{
		doubaoGroup := apiGroup.Group("/doubao/v3")
		doubaoGroup.POST("/contents/generations/tasks", handlers.Doubao.CreateTask)
		doubaoGroup.GET("/contents/generations/tasks/:id", handlers.Doubao.GetTask)
		doubaoGroup.DELETE("/contents/generations/tasks/:id", handlers.Doubao.DeleteTask)
	}

	{
		registerGeminiRoutes := func(group *gin.RouterGroup) {
			group.POST("/models/*action", handlers.Gemini.GenerateContent)
			group.GET("/models", handlers.Gemini.ListModels)
		}

		geminiGroup := server.Group("/gemini/:gemini-api-version",
			middleware.WithTimeout(server.Config.LLMRequestTimeout),
			middleware.WithIPBlocklist(services.SystemService),
			middleware.WithGeminiKeyAuth(services.AuthService),
			middleware.WithSource(request.SourceAPI),
			middleware.WithResponseHeaders(services.RequestService),
			middleware.WithThread(server.Config.Trace, services.ThreadService),
			middleware.WithTrace(server.Config.Trace, services.TraceService),
		)

		registerGeminiRoutes(geminiGroup)

		// Alias for Gemini API
		geminiAliasGroup := server.Group("/v1beta",
			middleware.WithTimeout(server.Config.LLMRequestTimeout),
			middleware.WithIPBlocklist(services.SystemService),
			middleware.WithGeminiKeyAuth(services.AuthService),
			middleware.WithSource(request.SourceAPI),
			middleware.WithResponseHeaders(services.RequestService),
			middleware.WithThread(server.Config.Trace, services.ThreadService),
			middleware.WithTrace(server.Config.Trace, services.TraceService),
		)

		registerGeminiRoutes(geminiAliasGroup)
	}
}

// registerPluginUIRoutes serves token-scoped plugin UI files without a user JWT.
// A nil manager does not panic here; the handler answers 503.
func registerPluginUIRoutes(group *gin.RouterGroup, manager *plugin.Manager, system *biz.SystemService) {
	handler := func(c *gin.Context) {
		request := plugin.PluginUIRequest{
			Manager: manager,
			Token:   c.Param("token"),
			Path:    c.Param("path"),
		}
		if system == nil {
			request.SecretErr = errors.New("system secret is unavailable")
		} else {
			secret, err := authz.RunWithSystemBypass(c.Request.Context(), "plugin-ui-secret", func(ctx context.Context) (string, error) {
				return system.SecretKey(ctx)
			})
			if err != nil {
				request.SecretErr = err
			} else {
				request.Secret = secret
			}
		}
		plugin.ServePluginUI(c.Writer, c.Request, request)
	}
	// Only GET/HEAD are routed: other methods must not be registered here,
	// because a root-level OPTIONS("*any") catch-all exists when CORS is
	// enabled and Gin panics when a second wildcard is added to that tree.
	group.Handle(http.MethodGet, pluginUIRoutePath, handler)
	group.Handle(http.MethodHead, pluginUIRoutePath, handler)
}

const (
	adminGroupPath    = "/admin"
	pluginUIPath      = "/plugin-ui/"
	pluginUIRoutePath = pluginUIPath + ":token/*path"
)

// skipCredentialedCORS 让插件 UI 不走带凭证的全局 CORS。
// 插件页面在不带 allow-same-origin 的 sandbox iframe 里，来源是不透明的 null。
// 模块脚本会发送 Origin: null。gin-contrib/cors 对白名单外的来源会在处理函数之前
// 直接 403，插件路由自己的 Access-Control-Allow-Origin: * 就写不上去。
// null 不能加进这份白名单：中间件只接受 http(s) 来源，而且放开 null 会让任意沙箱页面读取带凭证的接口。
// 这条路径上的 OPTIONS 因此直接返回，响应是空的 200，没有 CORS 头。模块脚本和普通 GET 不会预检。
// 以后插件如果发送自定义请求头，预检会失败。
func skipCredentialedCORS(corsHandler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPluginUIResource(c.Request.URL.Path) {
			return
		}
		corsHandler(c)
	}
}

func isPluginUIResource(path string) bool {
	return strings.HasPrefix(path, adminGroupPath+pluginUIPath)
}
