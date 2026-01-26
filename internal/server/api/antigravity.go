package api

import (
	"context"
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

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
)

type AntigravityHandlersParams struct {
	fx.In

	CacheConfig xcache.Config
	HttpClient  *httpclient.HttpClient
}

type AntigravityHandlers struct {
	stateCache xcache.Cache[antigravityOAuthState]
	httpClient *httpclient.HttpClient
}

func NewAntigravityHandlers(params AntigravityHandlersParams) *AntigravityHandlers {
	return &AntigravityHandlers{
		stateCache: xcache.NewFromConfig[antigravityOAuthState](params.CacheConfig),
		httpClient: params.HttpClient,
	}
}

type StartAntigravityOAuthRequest struct {
	ProjectID string `json:"project_id"`
}

type StartAntigravityOAuthResponse struct {
	SessionID string `json:"session_id"`
	AuthURL   string `json:"auth_url"`
}

type antigravityOAuthState struct {
	CodeVerifier string `json:"code_verifier"`
	ProjectID    string `json:"project_id"`
	CreatedAt    int64  `json:"created_at"`
}

func generateAntigravityCodeVerifier() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

func generateAntigravityCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
}

func generateAntigravityState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

func antigravityOAuthCacheKey(sessionID string) string {
	return fmt.Sprintf("antigravity:oauth:%s", sessionID)
}

// StartOAuth creates a PKCE session and returns the authorize URL.
// POST /admin/antigravity/oauth/start.
func (h *AntigravityHandlers) StartOAuth(c *gin.Context) {
	ctx := c.Request.Context()

	var req StartAntigravityOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	state, err := generateAntigravityState()
	if err != nil {
		log.Error(ctx, "failed to generate oauth state", log.Cause(err))
		JSONError(c, http.StatusInternalServerError, errors.New("internal server error"))
		return
	}

	codeVerifier, err := generateAntigravityCodeVerifier()
	if err != nil {
		log.Error(ctx, "failed to generate code verifier", log.Cause(err))
		JSONError(c, http.StatusInternalServerError, errors.New("internal server error"))
		return
	}

	codeChallenge := generateAntigravityCodeChallenge(codeVerifier)

	cacheKey := antigravityOAuthCacheKey(state)
	if err := h.stateCache.Set(ctx, cacheKey, antigravityOAuthState{
		CodeVerifier: codeVerifier,
		ProjectID:    req.ProjectID,
		CreatedAt:    time.Now().Unix(),
	}, xcache.WithExpiration(10*time.Minute)); err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to save oauth state: %w", err))
		return
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", antigravity.ClientID)
	params.Set("redirect_uri", antigravity.RedirectURI)
	params.Set("scope", antigravity.ScopesString)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")

	authURL := fmt.Sprintf("%s?%s", antigravity.AuthorizeURL, params.Encode())

	c.JSON(http.StatusOK, StartAntigravityOAuthResponse{SessionID: state, AuthURL: authURL})
}

type ExchangeAntigravityOAuthRequest struct {
	SessionID   string `json:"session_id" binding:"required"`
	CallbackURL string `json:"callback_url" binding:"required"`
}

type ExchangeAntigravityOAuthResponse struct {
	Credentials string `json:"credentials"`
}

func parseAntigravityCallbackURL(callbackURL string) (string, string, error) {
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

// Exchange exchanges callback URL for OAuth credentials.
// POST /admin/antigravity/oauth/exchange.
func (h *AntigravityHandlers) Exchange(c *gin.Context) {
	ctx := c.Request.Context()

	var req ExchangeAntigravityOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	cacheKey := antigravityOAuthCacheKey(req.SessionID)

	state, err := h.stateCache.Get(ctx, cacheKey)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid or expired oauth session"))
		return
	}

	code, callbackState, err := parseAntigravityCallbackURL(req.CallbackURL)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	if callbackState != req.SessionID {
		JSONError(c, http.StatusBadRequest, errors.New("oauth state mismatch"))
		return
	}

	// Delete state after validation succeeds
	if err := h.stateCache.Delete(ctx, cacheKey); err != nil {
		log.Warn(ctx, "failed to delete used oauth state from cache", log.String("session_id", req.SessionID), log.Cause(err))
	}

	tokenProvider := antigravity.NewTokenProvider(oauth.TokenProviderParams{
		HTTPClient: h.httpClient,
	})

	creds, err := tokenProvider.Exchange(ctx, oauth.ExchangeParams{
		Code:         code,
		CodeVerifier: state.CodeVerifier,
		ClientID:     antigravity.ClientID,
		RedirectURI:  antigravity.RedirectURI,
	})
	if err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("token exchange failed: %w", err))
		return
	}

	projectID := state.ProjectID
	if projectID == "" {
		projectID, err = h.resolveProjectID(ctx, creds.AccessToken)
		if err != nil {
			log.Warn(ctx, "failed to resolve project id", log.Cause(err))
			JSONError(c, http.StatusBadGateway, fmt.Errorf("failed to resolve project id and none provided: %w", err))

			return
		}
	}

	// Format: refreshToken|projectId
	output := fmt.Sprintf("%s|%s", creds.RefreshToken, projectID)

	c.JSON(http.StatusOK, ExchangeAntigravityOAuthResponse{Credentials: output})
}

func (h *AntigravityHandlers) resolveProjectID(ctx context.Context, accessToken string) (string, error) {
	if len(antigravity.LoadEndpoints) == 0 {
		return "", errors.New("no load endpoints configured")
	}

	// Try each load endpoint
	var lastErr error
	var defaultTierID string = "FREE"

	for _, baseEndpoint := range antigravity.LoadEndpoints {
		url := fmt.Sprintf("%s/v1internal:loadCodeAssist", baseEndpoint)
		reqBody := map[string]any{
			"metadata": map[string]string{
				"ideType":    "IDE_UNSPECIFIED",
				"platform":   "PLATFORM_UNSPECIFIED",
				"pluginType": "GEMINI",
			},
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			lastErr = fmt.Errorf("failed to marshal request body: %w", err)
			continue
		}

		req := &httpclient.Request{
			Method: http.MethodPost,
			URL:    url,
			Headers: http.Header{
				"Authorization":     []string{fmt.Sprintf("Bearer %s", accessToken)},
				"Content-Type":      []string{"application/json"},
				"User-Agent":        []string{antigravity.UserAgent},
				"X-Goog-Api-Client": []string{"google-cloud-sdk vscode_cloudshelleditor/0.1"},
				"Client-Metadata":   []string{`{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`},
			},
			Body: bodyBytes,
		}

		resp, err := h.httpClient.Do(ctx, req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			continue
		}

		var data struct {
			CloudAICompanionProject any `json:"cloudaicompanionProject"`
			AllowedTiers            []struct {
				ID        string `json:"id"`
				IsDefault bool   `json:"isDefault"`
			} `json:"allowedTiers"`
		}
		if err := json.Unmarshal(resp.Body, &data); err != nil {
			lastErr = err
			continue
		}

		// Check for project ID
		var projectID string
		if s, ok := data.CloudAICompanionProject.(string); ok && s != "" {
			projectID = s
		} else if m, ok := data.CloudAICompanionProject.(map[string]any); ok {
			if id, ok := m["id"].(string); ok && id != "" {
				projectID = id
			}
		}

		if projectID != "" {
			return projectID, nil
		}

		// If no project ID, try to determine default tier for onboarding
		if len(data.AllowedTiers) > 0 {
			defaultTierID = data.AllowedTiers[0].ID
			for _, tier := range data.AllowedTiers {
				if tier.IsDefault {
					defaultTierID = tier.ID
					break
				}
			}
		}

		// Try onboarding since we didn't get a project ID
		projectID, err = h.onboardUser(ctx, accessToken, defaultTierID)
		if err == nil && projectID != "" {
			return projectID, nil
		}
		if err != nil {
			log.Warn(ctx, "failed to onboard user", log.Cause(err))
			lastErr = err
		}
	}

	return "", lastErr
}

func (h *AntigravityHandlers) onboardUser(ctx context.Context, accessToken, tierID string) (string, error) {
	// Try endpoints for onboarding
	for _, baseEndpoint := range antigravity.LoadEndpoints {
		url := fmt.Sprintf("%s/v1internal:onboardUser", baseEndpoint)
		reqBody := map[string]any{
			"tierId": tierID,
			"metadata": map[string]string{
				"ideType":    "IDE_UNSPECIFIED",
				"platform":   "PLATFORM_UNSPECIFIED",
				"pluginType": "GEMINI",
			},
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := &httpclient.Request{
			Method: http.MethodPost,
			URL:    url,
			Headers: http.Header{
				"Authorization":     []string{fmt.Sprintf("Bearer %s", accessToken)},
				"Content-Type":      []string{"application/json"},
				"User-Agent":        []string{antigravity.UserAgent},
				"X-Goog-Api-Client": []string{"google-cloud-sdk vscode_cloudshelleditor/0.1"},
				"Client-Metadata":   []string{`{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`},
			},
			Body: bodyBytes,
		}

		// Try up to 3 times with delay
		for i := 0; i < 3; i++ {
			resp, err := h.httpClient.Do(ctx, req)
			if err != nil || resp.StatusCode != http.StatusOK {
				time.Sleep(1 * time.Second)
				continue
			}

			var data struct {
				Done     bool `json:"done"`
				Response struct {
					CloudAICompanionProject struct {
						ID string `json:"id"`
					} `json:"cloudaicompanionProject"`
				} `json:"response"`
			}

			if err := json.Unmarshal(resp.Body, &data); err != nil {
				continue
			}

			if data.Done && data.Response.CloudAICompanionProject.ID != "" {
				return data.Response.CloudAICompanionProject.ID, nil
			}

			time.Sleep(2 * time.Second)
		}
	}

	return "", errors.New("failed to onboard user after retries")
}
