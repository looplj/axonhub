package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
)

type ClaudeCodeHandlersParams struct {
	fx.In

	CacheConfig xcache.Config
	HttpClient  *httpclient.HttpClient
}

type ClaudeCodeHandlers struct {
	stateCache xcache.Cache[claudeCodeOAuthState]
	httpClient *httpclient.HttpClient
}

func NewClaudeCodeHandlers(params ClaudeCodeHandlersParams) *ClaudeCodeHandlers {
	return &ClaudeCodeHandlers{
		stateCache: xcache.NewFromConfig[claudeCodeOAuthState](params.CacheConfig),
		httpClient: params.HttpClient,
	}
}

type StartClaudeCodeOAuthRequest struct{}

type StartClaudeCodeOAuthResponse struct {
	SessionID string `json:"session_id"`
	AuthURL   string `json:"auth_url"`
}

type claudeCodeOAuthState struct {
	CodeVerifier string `json:"code_verifier"`
	CreatedAt    int64  `json:"created_at"`
}

func generateClaudeCodeCodeVerifier() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

func generateClaudeCodeCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
}

func generateClaudeCodeState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

func claudeCodeOAuthCacheKey(sessionID string) string {
	return fmt.Sprintf("claudecode:oauth:%s", sessionID)
}

// StartOAuth creates a PKCE session and returns the authorize URL.
// POST /admin/claudecode/oauth/start.
func (h *ClaudeCodeHandlers) StartOAuth(c *gin.Context) {
	ctx := c.Request.Context()

	var req StartClaudeCodeOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	state, err := generateClaudeCodeState()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to generate oauth state: %w", err))
		return
	}

	codeVerifier, err := generateClaudeCodeCodeVerifier()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to generate code verifier: %w", err))
		return
	}

	codeChallenge := generateClaudeCodeCodeChallenge(codeVerifier)

	cacheKey := claudeCodeOAuthCacheKey(state)
	if err := h.stateCache.Set(ctx, cacheKey, claudeCodeOAuthState{CodeVerifier: codeVerifier, CreatedAt: time.Now().Unix()}, xcache.WithExpiration(10*time.Minute)); err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to save oauth state: %w", err))
		return
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", claudecode.ClientID)
	params.Set("redirect_uri", claudecode.RedirectURI)
	params.Set("scope", claudecode.Scopes)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)

	authURL := fmt.Sprintf("%s?%s", claudecode.AuthorizeURL, params.Encode())

	c.JSON(http.StatusOK, StartClaudeCodeOAuthResponse{SessionID: state, AuthURL: authURL})
}

type ExchangeClaudeCodeOAuthRequest struct {
	SessionID   string                  `json:"session_id" binding:"required"`
	CallbackURL string                  `json:"callback_url" binding:"required"`
	Proxy       *httpclient.ProxyConfig `json:"proxy,omitempty"`
}

type ExchangeClaudeCodeOAuthResponse struct {
	Credentials string `json:"credentials"`
}

func parseClaudeCodeCallbackURL(callbackURL string) (string, string, error) {
	trimmed := strings.TrimSpace(callbackURL)
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return "", "", fmt.Errorf("callback_url must be a full URL")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("invalid callback_url: %w", err)
	}

	q := u.Query()

	code := q.Get("code")
	if code == "" {
		return "", "", fmt.Errorf("code parameter not found in callback_url")
	}

	// Claude puts state in URL fragment (after #), not in query params
	// Format: http://localhost:54545/callback?code=xxx#state
	state := u.Fragment

	// Also check query params as fallback
	if state == "" {
		state = q.Get("state")
	}

	if state == "" {
		return "", "", fmt.Errorf("state parameter not found in callback_url (should be after # or in query)")
	}

	return code, state, nil
}

// Exchange exchanges callback URL for OAuth credentials JSON.
// POST /admin/claudecode/oauth/exchange.
func (h *ClaudeCodeHandlers) Exchange(c *gin.Context) {
	ctx := c.Request.Context()

	var req ExchangeClaudeCodeOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	cacheKey := claudeCodeOAuthCacheKey(req.SessionID)

	state, err := h.stateCache.Get(ctx, cacheKey)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid or expired oauth session"))
		return
	}

	if err := h.stateCache.Delete(ctx, cacheKey); err != nil {
		log.Warn(ctx, "failed to delete used oauth state from cache", log.String("session_id", req.SessionID), log.Cause(err))
	}

	code, callbackState, err := parseClaudeCodeCallbackURL(req.CallbackURL)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	if callbackState != req.SessionID {
		JSONError(c, http.StatusBadRequest, errors.New("oauth state mismatch"))
		return
	}

	// Create HTTP client with proxy if provided
	httpClient := h.httpClient
	if req.Proxy != nil && req.Proxy.Type == httpclient.ProxyTypeURL && req.Proxy.URL != "" {
		httpClient = httpclient.NewHttpClientWithProxy(req.Proxy)
	}

	tokenProvider := claudecode.NewTokenProvider(oauth.TokenProviderParams{
		HTTPClient: httpClient,
	})

	creds, err := tokenProvider.Exchange(ctx, oauth.ExchangeParams{
		Code:         code,
		State:        callbackState,
		CodeVerifier: state.CodeVerifier,
		ClientID:     claudecode.ClientID,
		RedirectURI:  claudecode.RedirectURI,
	})
	if err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("token exchange failed: %w", err))
		return
	}

	output, err := creds.ToJSON()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to encode credentials: %w", err))
		return
	}

	c.JSON(http.StatusOK, ExchangeClaudeCodeOAuthResponse{Credentials: output})
}
