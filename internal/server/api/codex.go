package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

const (
	codexAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	//nolint:gosec // false alert.
	codexTokenURL    = "https://auth.openai.com/oauth/token"
	codexClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexRedirectURI = "http://localhost:1455/auth/callback"
	codexScopes      = "openid profile email offline_access"

	codexUserAgent = "codex_cli_rs/0.38.0 (Ubuntu 22.04.0; x86_64) WindowsTerminal"
)

type CodexHandlersParams struct {
	fx.In

	CacheConfig xcache.Config
	HttpClient  *httpclient.HttpClient
}

type CodexHandlers struct {
	stateCache xcache.Cache[codexOAuthState]
	httpClient *httpclient.HttpClient
}

func NewCodexHandlers(params CodexHandlersParams) *CodexHandlers {
	return &CodexHandlers{
		stateCache: xcache.NewFromConfig[codexOAuthState](params.CacheConfig),
		httpClient: params.HttpClient,
	}
}

type StartCodexOAuthRequest struct{}

type StartCodexOAuthResponse struct {
	SessionID string `json:"session_id"`
	AuthURL   string `json:"auth_url"`
}

type codexOAuthState struct {
	CodeVerifier string `json:"code_verifier"`
	CreatedAt    int64  `json:"created_at"`
}

func generateCodexCodeVerifier() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

func generateCodexCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
}

func generateCodexState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

func codexOAuthCacheKey(projectID int, sessionID string) string {
	return fmt.Sprintf("codex:oauth:%d:%s", projectID, sessionID)
}

// StartOAuth creates a PKCE session and returns the authorize URL.
// POST /admin/codex/oauth/start.
func (h *CodexHandlers) StartOAuth(c *gin.Context) {
	ctx := c.Request.Context()

	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		JSONError(c, http.StatusBadRequest, errors.New("project id not found; set X-Project-ID header"))
		return
	}

	var req StartCodexOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	state, err := generateCodexState()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to generate oauth state: %w", err))
		return
	}

	codeVerifier, err := generateCodexCodeVerifier()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to generate code verifier: %w", err))
		return
	}

	codeChallenge := generateCodexCodeChallenge(codeVerifier)

	cacheKey := codexOAuthCacheKey(projectID, state)
	if err := h.stateCache.Set(ctx, cacheKey, codexOAuthState{CodeVerifier: codeVerifier, CreatedAt: time.Now().Unix()}, xcache.WithExpiration(10*time.Minute)); err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to save oauth state: %w", err))
		return
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", codexClientID)
	params.Set("redirect_uri", codexRedirectURI)
	params.Set("scope", codexScopes)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")

	authURL := fmt.Sprintf("%s?%s", codexAuthorizeURL, params.Encode())

	c.JSON(http.StatusOK, StartCodexOAuthResponse{SessionID: state, AuthURL: authURL})
}

type ExchangeCodexOAuthRequest struct {
	SessionID   string `json:"session_id" binding:"required"`
	CallbackURL string `json:"callback_url" binding:"required"`
}

type ExchangeCodexOAuthResponse struct {
	Credentials string `json:"credentials"`
}

type codexTokenResponse struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
}

type codexOAuth2Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ClientID     string    `json:"client_id,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
}

func parseCodexCallbackURL(callbackURL string) (string, string, error) {
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

	state := q.Get("state")
	if state == "" {
		return "", "", fmt.Errorf("state parameter not found in callback_url")
	}

	return code, state, nil
}

// Exchange exchanges callback URL for OAuth credentials JSON.
// POST /admin/codex/oauth/exchange.
func (h *CodexHandlers) Exchange(c *gin.Context) {
	ctx := c.Request.Context()

	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		JSONError(c, http.StatusBadRequest, errors.New("project id not found; set X-Project-ID header"))
		return
	}

	var req ExchangeCodexOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	if req.SessionID == "" || req.CallbackURL == "" {
		JSONError(c, http.StatusBadRequest, errors.New("session_id and callback_url are required"))
		return
	}

	cacheKey := codexOAuthCacheKey(projectID, req.SessionID)

	state, err := h.stateCache.Get(ctx, cacheKey)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid or expired oauth session"))
		return
	}

	if err := h.stateCache.Delete(ctx, cacheKey); err != nil {
		log.Warn(ctx, "failed to delete used oauth state from cache", log.Int("project_id", projectID), log.String("session_id", req.SessionID), log.Cause(err))
	}

	code, callbackState, err := parseCodexCallbackURL(req.CallbackURL)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	if callbackState != req.SessionID {
		JSONError(c, http.StatusBadRequest, errors.New("oauth state mismatch"))
		return
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", codexClientID)
	form.Set("code", code)
	form.Set("redirect_uri", codexRedirectURI)
	form.Set("code_verifier", state.CodeVerifier)

	hreq := &httpclient.Request{
		Method: http.MethodPost,
		URL:    codexTokenURL,
		Headers: http.Header{
			"Content-Type": []string{"application/x-www-form-urlencoded"},
			"User-Agent":   []string{codexUserAgent},
			"Accept":       []string{"application/json"},
		},
		Body: []byte(form.Encode()),
	}

	resp, err := h.httpClient.Do(ctx, hreq)
	if err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("token exchange failed: %w", err))
		return
	}

	var tokenResp codexTokenResponse
	if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("failed to decode token response: %w", err))
		return
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		JSONError(c, http.StatusBadGateway, errors.New("token response missing required fields"))
		return
	}

	accountID := ""
	if tokenResp.IDToken != "" {
		accountID = codex.ExtractChatGPTAccountIDFromJWT(tokenResp.IDToken)
	}

	if accountID == "" {
		accountID = codex.ExtractChatGPTAccountIDFromJWT(tokenResp.AccessToken)
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	creds := codexOAuth2Credentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     codexClientID,
		AccountID:    accountID,
		ExpiresAt:    expiresAt,
		TokenType:    tokenResp.TokenType,
	}

	if tokenResp.Scope != "" {
		creds.Scopes = strings.Fields(tokenResp.Scope)
	}

	raw, err := json.Marshal(creds)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to encode credentials: %w", err))
		return
	}

	c.JSON(http.StatusOK, ExchangeCodexOAuthResponse{Credentials: string(raw)})
}
