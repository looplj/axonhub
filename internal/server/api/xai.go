package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/xai/subscription"
)

type XAIHandlersParams struct {
	fx.In

	CacheConfig xcache.Config
	HttpClient  *httpclient.HttpClient
}

type XAIHandlers struct {
	stateCache xcache.Cache[xaiOAuthState]
	httpClient *httpclient.HttpClient
}

type xaiOAuthState struct {
	CodeVerifier string `json:"code_verifier"`
	UserID       int    `json:"user_id"`
}

type StartXAIOAuthResponse struct {
	SessionID string `json:"session_id"`
	AuthURL   string `json:"auth_url"`
}

type ExchangeXAIOAuthRequest struct {
	SessionID   string                  `json:"session_id" binding:"required"`
	CallbackURL string                  `json:"callback_url" binding:"required"`
	Proxy       *httpclient.ProxyConfig `json:"proxy,omitempty"`
}

type ExchangeXAIOAuthResponse struct {
	Credentials string `json:"credentials"`
}

type DecodeXAISSORequest struct {
	SSOToken string                  `json:"sso_token" binding:"required"`
	Proxy    *httpclient.ProxyConfig `json:"proxy,omitempty"`
}

type DecodeXAISSOResponse struct {
	Credentials string `json:"credentials"`
}

func NewXAIHandlers(params XAIHandlersParams) *XAIHandlers {
	return &XAIHandlers{
		stateCache: xcache.NewFromConfig[xaiOAuthState](params.CacheConfig),
		httpClient: params.HttpClient,
	}
}

func xaiOAuthCacheKey(sessionID string) string {
	return fmt.Sprintf("xai:oauth:%s", sessionID)
}

// StartOAuth creates a PKCE session for the official xAI subscription channel.
func (handler *XAIHandlers) StartOAuth(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := contexts.GetUser(ctx)
	if !ok {
		JSONError(c, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}

	state, err := oauth.GenerateState(32)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("generate xAI oauth state: %w", err))
		return
	}
	verifier, err := oauth.GenerateCodeVerifier(32)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("generate xAI code verifier: %w", err))
		return
	}
	nonce, err := oauth.GenerateState(16)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("generate xAI nonce: %w", err))
		return
	}
	if err := handler.stateCache.Set(ctx, xaiOAuthCacheKey(state), xaiOAuthState{
		CodeVerifier: verifier,
		UserID:       user.ID,
	}, xcache.WithExpiration(30*time.Minute)); err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("save xAI oauth state: %w", err))
		return
	}

	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", subscription.ClientID)
	query.Set("redirect_uri", subscription.RedirectURI)
	query.Set("scope", subscription.Scopes)
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", oauth.GenerateCodeChallenge(verifier))
	query.Set("code_challenge_method", "S256")
	query.Set("plan", "generic")
	query.Set("referrer", "axonhub")

	c.JSON(http.StatusOK, StartXAIOAuthResponse{
		SessionID: state,
		AuthURL:   subscription.AuthorizeURL + "?" + query.Encode(),
	})
}

// Exchange exchanges an xAI OAuth callback URL for normalized credentials JSON.
func (handler *XAIHandlers) Exchange(c *gin.Context) {
	ctx := c.Request.Context()
	var request ExchangeXAIOAuthRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	state, err := handler.stateCache.Get(ctx, xaiOAuthCacheKey(request.SessionID))
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid or expired oauth session"))
		return
	}
	user, ok := contexts.GetUser(ctx)
	if !ok {
		JSONError(c, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}
	if state.UserID != user.ID {
		JSONError(c, http.StatusBadRequest, errors.New("invalid or expired oauth session"))
		return
	}
	code, callbackState, err := parseXAICallbackURL(request.CallbackURL)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}
	if callbackState != request.SessionID {
		JSONError(c, http.StatusBadRequest, errors.New("oauth state mismatch"))
		return
	}
	if err := handler.stateCache.Delete(ctx, xaiOAuthCacheKey(request.SessionID)); err != nil {
		log.Warn(ctx, "failed to delete used xAI oauth state", log.String("session_id", request.SessionID), log.Cause(err))
	}

	client := handler.httpClient
	if request.Proxy != nil && request.Proxy.Type == httpclient.ProxyTypeURL && request.Proxy.URL != "" {
		client = client.WithProxy(request.Proxy)
	}
	credentials, err := subscription.NewTokenProvider(subscription.TokenProviderParams{HTTPClient: client}).Exchange(ctx, oauth.ExchangeParams{
		Code: code, CodeVerifier: state.CodeVerifier, ClientID: subscription.ClientID, RedirectURI: subscription.RedirectURI,
	})
	if err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("xAI token exchange failed: %w", err))
		return
	}
	output, err := credentials.ToJSON()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("encode xAI credentials: %w", err))
		return
	}
	c.JSON(http.StatusOK, ExchangeXAIOAuthResponse{Credentials: output})
}

// DecodeSSO converts a Grok Web SSO cookie to Build OAuth credentials.
func (handler *XAIHandlers) DecodeSSO(c *gin.Context) {
	ctx := c.Request.Context()
	var request DecodeXAISSORequest
	if err := c.ShouldBindJSON(&request); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	client := handler.httpClient
	if request.Proxy != nil && request.Proxy.Type == httpclient.ProxyTypeURL && request.Proxy.URL != "" {
		client = client.WithProxy(request.Proxy)
	}
	credentials, err := subscription.ConvertSSOToBuild(ctx, request.SSOToken, subscription.SSODeviceOptions{HTTPClient: client.GetNativeClient()})
	if err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("xAI SSO conversion failed: %w", err))
		return
	}
	output, err := credentials.ToJSON()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("encode xAI credentials: %w", err))
		return
	}
	c.JSON(http.StatusOK, DecodeXAISSOResponse{Credentials: output})
}

func parseXAICallbackURL(callbackURL string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(callbackURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", errors.New("callback_url must be a full URL")
	}
	code := parsed.Query().Get("code")
	state := parsed.Query().Get("state")
	if code == "" || state == "" {
		return "", "", errors.New("callback_url must contain code and state")
	}
	return code, state, nil
}
