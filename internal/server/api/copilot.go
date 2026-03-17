package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	// GitHub Copilot OAuth Device Flow endpoints
	githubDeviceCodeURL  = "https://github.com/login/device/code"
	githubAccessTokenURL = "https://github.com/login/oauth/access_token" //nolint:gosec

	//nolint:gosec // This is a public OAuth client identifier, not a secret
	// defaultGithubCopilotClientID is the VS Code public client ID, used as fallback
	defaultGithubCopilotClientID = "Iv1.b507a08c87ecfe98"

	// OAuth scopes for GitHub Copilot
	githubCopilotScope = "read:user"

	// Grant type for device flow
	deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

	// Cache key prefix
	copilotOAuthCacheKeyPrefix = "copilot:oauth"

	// Default cache expiration for device flow (15 minutes)
	deviceFlowCacheExpiration = 15 * time.Minute
)

// getGithubCopilotClientID returns the GitHub Copilot OAuth client ID.
// It checks the GITHUB_COPILOT_CLIENT_ID environment variable first,
// then falls back to the default VS Code client ID.
func getGithubCopilotClientID() string {
	if clientID := os.Getenv("GITHUB_COPILOT_CLIENT_ID"); clientID != "" {
		return clientID
	}
	return defaultGithubCopilotClientID
}

// CopilotHandlersParams contains the dependencies for CopilotHandlers.
type CopilotHandlersParams struct {
	fx.In

	CacheConfig xcache.Config
	HttpClient  *httpclient.HttpClient
	Clock       Clock `optional:"true"`
}

// Clock provides time-related functions for testability.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// realClock implements Clock with real time functions.
type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// CopilotHandlers provides HTTP handlers for GitHub Copilot OAuth device flow.
type CopilotHandlers struct {
	deviceCodeCache xcache.Cache[copilotDeviceFlowState]
	httpClient      *httpclient.HttpClient
	clock           Clock
}

// copilotDeviceFlowState stores the state of a device flow authorization.
type copilotDeviceFlowState struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	CreatedAt       int64  `json:"created_at"`
}

// deviceCodeResponse represents the response from GitHub's device code endpoint.
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// accessTokenResponse represents the response from GitHub's access token endpoint.
type accessTokenResponse struct {
	Token     string `json:"access_token"` //nolint:gosec
	TokenType string `json:"token_type"`
	Scope     string `json:"scope"`
	Error     string `json:"error"`
	ErrorDesc string `json:"error_description"`
}

func NewCopilotHandlers(params CopilotHandlersParams) *CopilotHandlers {
	clock := params.Clock
	if clock == nil {
		clock = realClock{}
	}
	return &CopilotHandlers{
		deviceCodeCache: xcache.NewFromConfig[copilotDeviceFlowState](params.CacheConfig),
		httpClient:      params.HttpClient,
		clock:           clock,
	}
}

// generateCopilotSessionID generates a unique session ID for the device flow.
func generateCopilotSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// copilotOAuthCacheKey generates a cache key for the OAuth session.
func copilotOAuthCacheKey(sessionID string) string {
	return fmt.Sprintf("%s:%s", copilotOAuthCacheKeyPrefix, sessionID)
}

// StartCopilotOAuthRequest represents the request body for starting OAuth device flow.
type StartCopilotOAuthRequest struct {
	Proxy *httpclient.ProxyConfig `json:"proxy,omitempty"`
}

// StartCopilotOAuthResponse represents the response for starting OAuth device flow.
type StartCopilotOAuthResponse struct {
	SessionID       string `json:"session_id"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// StartOAuth initiates the GitHub OAuth device flow.
// POST /admin/copilot/oauth/start
func (h *CopilotHandlers) StartOAuth(c *gin.Context) {
	ctx := c.Request.Context()

	var req StartCopilotOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body for requests with optional fields
		// Only skip error if it's exactly "EOF" (empty body), not other EOF-related errors
		if err.Error() != "EOF" {
			JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
			return
		}
	}

	sessionID, err := generateCopilotSessionID()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to generate session ID: %w", err))
		return
	}

	// Create HTTP client with proxy if provided
	httpClient := h.httpClient
	if req.Proxy != nil && req.Proxy.Type == httpclient.ProxyTypeURL && req.Proxy.URL != "" {
		httpClient = h.httpClient.WithProxy(req.Proxy)
	}

	// Step 1: Request device code from GitHub
	deviceCodeResp, err := h.requestDeviceCode(ctx, httpClient)
	if err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("failed to request device code: %w", err))
		return
	}

	// Store device flow state in cache
	state := copilotDeviceFlowState{
		DeviceCode:      deviceCodeResp.DeviceCode,
		UserCode:        deviceCodeResp.UserCode,
		VerificationURI: deviceCodeResp.VerificationURI,
		ExpiresIn:       deviceCodeResp.ExpiresIn,
		Interval:        deviceCodeResp.Interval,
		CreatedAt:       h.clock.Now().Unix(),
	}

	cacheKey := copilotOAuthCacheKey(sessionID)
	expiration := time.Duration(deviceCodeResp.ExpiresIn) * time.Second
	if expiration > deviceFlowCacheExpiration {
		expiration = deviceFlowCacheExpiration
	}

	if err := h.deviceCodeCache.Set(ctx, cacheKey, state, xcache.WithExpiration(expiration)); err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to save device flow state: %w", err))
		return
	}

	c.JSON(http.StatusOK, StartCopilotOAuthResponse{
		SessionID:       sessionID,
		UserCode:        deviceCodeResp.UserCode,
		VerificationURI: deviceCodeResp.VerificationURI,
		ExpiresIn:       deviceCodeResp.ExpiresIn,
		Interval:        deviceCodeResp.Interval,
	})
}

// requestDeviceCode requests a device code from GitHub's device flow endpoint.
func (h *CopilotHandlers) requestDeviceCode(ctx context.Context, httpClient *httpclient.HttpClient) (*deviceCodeResponse, error) {
	formData := url.Values{}
	formData.Set("client_id", getGithubCopilotClientID())
	formData.Set("scope", githubCopilotScope)

	req := &httpclient.Request{
		Method:      http.MethodPost,
		URL:         githubDeviceCodeURL,
		Headers:     http.Header{"Accept": []string{"application/json"}},
		ContentType: "application/x-www-form-urlencoded",
		Body:        []byte(formData.Encode()),
	}

	resp, err := httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}

	// Check for non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("device code request failed with status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var deviceResp deviceCodeResponse
	if err := json.Unmarshal(resp.Body, &deviceResp); err != nil {
		return nil, fmt.Errorf("failed to parse device code response: %w", err)
	}

	if deviceResp.DeviceCode == "" {
		return nil, errors.New("device code not received from GitHub")
	}

	return &deviceResp, nil
}

// PollCopilotOAuthRequest represents the request body for polling OAuth token.
type PollCopilotOAuthRequest struct {
	SessionID string                  `json:"session_id" binding:"required"`
	Proxy     *httpclient.ProxyConfig `json:"proxy,omitempty"`
}

// PollCopilotOAuthResponse represents the response for polling OAuth token.
type PollCopilotOAuthResponse struct {
	Token   string `json:"access_token,omitempty"` //nolint:gosec
	Type    string `json:"token_type,omitempty"`
	Scope   string `json:"scope,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// PollOAuth polls for the OAuth access token using the device flow.
// POST /admin/copilot/oauth/poll
func (h *CopilotHandlers) PollOAuth(c *gin.Context) {
	ctx := c.Request.Context()

	var req PollCopilotOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	cacheKey := copilotOAuthCacheKey(req.SessionID)

	state, err := h.deviceCodeCache.Get(ctx, cacheKey)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid or expired session"))
		return
	}

	// Check if device code has expired
	if h.clock.Now().Unix() > state.CreatedAt+int64(state.ExpiresIn) {
		_ = h.deviceCodeCache.Delete(ctx, cacheKey)
		JSONError(c, http.StatusBadRequest, errors.New("device code expired"))
		return
	}

	// Create HTTP client with proxy if provided
	httpClient := h.httpClient
	if req.Proxy != nil && req.Proxy.Type == httpclient.ProxyTypeURL && req.Proxy.URL != "" {
		httpClient = h.httpClient.WithProxy(req.Proxy)
	}

	// Step 2: Poll for access token
	tokenResp, err := h.pollAccessToken(ctx, httpClient, state.DeviceCode)
	if err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("token poll failed: %w", err))
		return
	}

	// Handle error responses from GitHub
	if tokenResp.Error != "" {
		switch tokenResp.Error {
		case "authorization_pending":
			c.JSON(http.StatusOK, PollCopilotOAuthResponse{
				Status:  "pending",
				Message: "Authorization pending. User has not yet authorized the device.",
			})
			return
		case "slow_down":
			// Increase the polling interval
			c.JSON(http.StatusOK, PollCopilotOAuthResponse{
				Status:  "slow_down",
				Message: "Polling too fast. Please slow down.",
			})
			return
		case "expired_token":
			_ = h.deviceCodeCache.Delete(ctx, cacheKey)
			JSONError(c, http.StatusBadRequest, errors.New("device code expired"))
			return
		case "access_denied":
			_ = h.deviceCodeCache.Delete(ctx, cacheKey)
			JSONError(c, http.StatusBadRequest, errors.New("access denied by user"))
			return
		default:
			JSONError(c, http.StatusBadGateway, fmt.Errorf("OAuth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc))
			return
		}
	}

	// Success - access token received
	if tokenResp.Token != "" {
		// Clean up the cache entry
		if err := h.deviceCodeCache.Delete(ctx, cacheKey); err != nil {
			log.Warn(ctx, "failed to delete used oauth state from cache", log.String("session_id", req.SessionID), log.Cause(err))
		}

		c.JSON(http.StatusOK, PollCopilotOAuthResponse{
			Token:   tokenResp.Token,
			Type:    tokenResp.TokenType,
			Scope:   tokenResp.Scope,
			Status:  "complete",
			Message: "Authorization complete. Access token received.",
		})
		return
	}

	// Unexpected response
	JSONError(c, http.StatusInternalServerError, errors.New("unexpected response from GitHub"))
}

// pollAccessToken polls for the access token from GitHub's OAuth endpoint.
func (h *CopilotHandlers) pollAccessToken(ctx context.Context, httpClient *httpclient.HttpClient, deviceCode string) (*accessTokenResponse, error) {
	formData := url.Values{}
	formData.Set("client_id", getGithubCopilotClientID())
	formData.Set("device_code", deviceCode)
	formData.Set("grant_type", deviceGrantType)

	req := &httpclient.Request{
		Method:      http.MethodPost,
		URL:         githubAccessTokenURL,
		Headers:     http.Header{"Accept": []string{"application/json"}},
		ContentType: "application/x-www-form-urlencoded",
		Body:        []byte(formData.Encode()),
	}

	resp, err := httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("access token request failed: %w", err)
	}

	// Check for non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("access token request failed with status %d", resp.StatusCode)
	}

	// GitHub returns form-encoded response or JSON depending on Accept header
	// Try to parse as JSON first, then fall back to form-encoded
	contentType := resp.Headers.Get("Content-Type")

	var tokenResp accessTokenResponse
	if strings.Contains(contentType, "application/json") {
		if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
			return nil, fmt.Errorf("failed to parse access token JSON response: %w", err)
		}
	} else {
		// Parse form-encoded response
		values, err := url.ParseQuery(string(resp.Body))
		if err != nil {
			return nil, fmt.Errorf("failed to parse access token form response: %w", err)
		}

		tokenResp.Token = values.Get("access_token")
		tokenResp.TokenType = values.Get("token_type")
		tokenResp.Scope = values.Get("scope")
		tokenResp.Error = values.Get("error")
		tokenResp.ErrorDesc = values.Get("error_description")
	}

	return &tokenResp, nil
}
