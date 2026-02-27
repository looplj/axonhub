package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zhenzou/executors"
	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/llm/httpclient"
)

// DeviceFlowState stores the state of a device flow authorization.
// This is used internally to track the device flow progress.
type DeviceFlowState struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	CreatedAt       int64  `json:"created_at"`
}

// DeviceFlowResponse represents the response from the device authorization endpoint.
// This is returned to the user to guide them through the device flow.
type DeviceFlowResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	CompleteURI     string `json:"verification_uri_complete,omitempty"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceFlowConfig contains the configuration for OAuth2 Device Flow.
type DeviceFlowConfig struct {
	// DeviceAuthURL is the URL for the device authorization endpoint
	DeviceAuthURL string
	// TokenURL is the URL for the token endpoint
	TokenURL string
	// ClientID is the OAuth2 client ID
	ClientID string
	// Scopes are the OAuth2 scopes to request
	Scopes []string
	// UserAgent is the User-Agent header to use for requests
	UserAgent string
}

// TokenExchanger defines the interface for exchanging OAuth access tokens
// for provider-specific tokens (e.g., Copilot token exchange).
// This is optional - if not set, GetToken returns the access_token directly.
type TokenExchanger interface {
	// Exchange exchanges an OAuth access token for a provider-specific token.
	// Returns the exchanged token, expiration timestamp, and any error.
	Exchange(ctx context.Context, accessToken string) (token string, expiresAt int64, err error)

	// ExchangeWithClient exchanges an OAuth access token using the provided HTTP client.
	// This allows customization of the HTTP client (e.g., with proxy settings).
	ExchangeWithClient(ctx context.Context, httpClient *httpclient.HttpClient, accessToken string) (token string, expiresAt int64, err error)
}

// DeviceFlowProvider manages OAuth2 Device Authorization Grant (RFC 8628).
// It handles the device flow lifecycle including:
// - Starting the device flow (getting device_code, user_code, verification_uri)
// - Polling for access token
// - Token refresh (if refresh_token is available)
// - Optional token exchange (for two-step flows like Copilot)
// - Auto-refresh and OnRefreshed callbacks for persistence
type DeviceFlowProvider struct {
	config     DeviceFlowConfig
	httpClient *httpclient.HttpClient

	// Optional token exchanger for two-step flows (e.g., Copilot)
	// If nil, GetToken returns the access_token directly
	tokenExchanger TokenExchanger

	// OAuth credentials (access_token, refresh_token, etc.)
	mu    sync.RWMutex
	creds *OAuthCredentials

	// Singleflight for deduplicating concurrent token requests
	sf singleflight.Group

	// Callback fired when credentials are refreshed (for persistence)
	onRefreshed func(ctx context.Context, refreshed *OAuthCredentials) error

	// Auto-refresh state
	autoMu         sync.Mutex
	autoCancel     context.CancelFunc
	autoExecutor   executors.ScheduledExecutor
	autoTaskCancel executors.CancelFunc
}

// DeviceFlowProviderParams contains the parameters for creating a new DeviceFlowProvider.
type DeviceFlowProviderParams struct {
	Config     DeviceFlowConfig
	HTTPClient *httpclient.HttpClient
	// Credentials are optional - can be set after device flow completes
	Credentials *OAuthCredentials
	// TokenExchanger is optional - for two-step token flows
	TokenExchanger TokenExchanger
	// OnRefreshed is called when credentials are refreshed (for persistence)
	OnRefreshed func(ctx context.Context, refreshed *OAuthCredentials) error
}

// NewDeviceFlowProvider creates a new DeviceFlowProvider instance.
func NewDeviceFlowProvider(params DeviceFlowProviderParams) *DeviceFlowProvider {
	return &DeviceFlowProvider{
		config:         params.Config,
		httpClient:     params.HTTPClient,
		creds:          params.Credentials,
		tokenExchanger: params.TokenExchanger,
		onRefreshed:    params.OnRefreshed,
	}
}

// Start initiates the OAuth2 Device Authorization Grant flow.
// It sends a request to the device authorization endpoint and returns
// the device_code, user_code, verification_uri, etc.
// The user must visit the verification_uri and enter the user_code.
func (p *DeviceFlowProvider) Start(ctx context.Context) (*DeviceFlowResponse, error) {
	if p.httpClient == nil {
		return nil, errors.New("http client is nil")
	}

	if p.config.DeviceAuthURL == "" {
		return nil, errors.New("device authorization URL is empty")
	}

	if p.config.ClientID == "" {
		return nil, errors.New("client_id is empty")
	}

	// Build the device authorization request
	form := url.Values{}
	form.Set("client_id", p.config.ClientID)
	if len(p.config.Scopes) > 0 {
		form.Set("scope", strings.Join(p.config.Scopes, " "))
	}

	header := http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
		"Accept":       []string{"application/json"},
	}
	if p.config.UserAgent != "" {
		header.Set("User-Agent", p.config.UserAgent)
	}

	req := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     p.config.DeviceAuthURL,
		Headers: header,
		Body:    []byte(form.Encode()),
	}

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("device authorization request failed: %w", err)
	}

	var deviceResp DeviceFlowResponse
	if err := json.Unmarshal(resp.Body, &deviceResp); err != nil {
		return nil, fmt.Errorf("decode device authorization response: %w", err)
	}

	if deviceResp.DeviceCode == "" {
		// Check for error response
		var tokenErr TokenError
		if err := json.Unmarshal(resp.Body, &tokenErr); err == nil && tokenErr.Error != "" {
			return nil, fmt.Errorf("device authorization failed: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
		}
		return nil, errors.New("device authorization response missing device_code")
	}

	return &deviceResp, nil
}

// Poll polls the token endpoint for an access token.
// It should be called repeatedly until the user completes the authorization
// or the device code expires.
// Returns OAuthCredentials when successful, or an error if:
// - authorization_pending: user hasn't authorized yet (caller should retry)
// - slow_down: polling too fast (caller should increase interval)
// - expired_token: device code expired
// - access_denied: user denied authorization
func (p *DeviceFlowProvider) Poll(ctx context.Context, deviceCode string) (*OAuthCredentials, error) {
	if p.httpClient == nil {
		return nil, errors.New("http client is nil")
	}

	if p.config.TokenURL == "" {
		return nil, errors.New("token URL is empty")
	}

	if deviceCode == "" {
		return nil, errors.New("device_code is empty")
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("client_id", p.config.ClientID)
	form.Set("device_code", deviceCode)

	header := http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
		"Accept":       []string{"application/json"},
	}
	if p.config.UserAgent != "" {
		header.Set("User-Agent", p.config.UserAgent)
	}

	req := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     p.config.TokenURL,
		Headers: header,
		Body:    []byte(form.Encode()),
	}

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}

	// Check for error response first
	var tokenErr TokenError
	if err := json.Unmarshal(resp.Body, &tokenErr); err == nil && tokenErr.Error != "" {
		switch tokenErr.Error {
		case "authorization_pending":
			return nil, errors.New("authorization_pending")
		case "slow_down":
			return nil, errors.New("slow_down")
		case "expired_token":
			return nil, errors.New("expired_token")
		case "access_denied":
			return nil, errors.New("access_denied")
		default:
			return nil, fmt.Errorf("token request failed: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
		}
	}

	creds, err := ParseTokenResponse(resp.Body, p.config.ClientID)
	if err != nil {
		return nil, err
	}

	// Store the credentials
	p.mu.Lock()
	p.creds = creds
	p.mu.Unlock()

	return creds, nil
}

// GetToken returns a valid token for API requests.
// If TokenExchanger is set, it exchanges the access_token for a provider-specific token.
// Otherwise, it returns the access_token directly.
// Handles token refresh if the token is expired and refresh_token is available.
func (p *DeviceFlowProvider) GetToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if creds == nil {
		return "", errors.New("credentials is nil")
	}

	if creds.AccessToken == "" {
		return "", errors.New("access token is empty")
	}

	// If token exchanger is set, use it to get the exchanged token
	if p.tokenExchanger != nil {
		return p.getExchangedToken(ctx, creds.AccessToken)
	}

	// Otherwise, return the access_token (with refresh if needed)
	return p.getAccessTokenWithRefresh(ctx)
}

// getExchangedToken exchanges the access token using the TokenExchanger.
func (p *DeviceFlowProvider) getExchangedToken(ctx context.Context, accessToken string) (string, error) {
	v, err, _ := p.sf.Do("exchange", func() (any, error) {
		// Try with custom HTTP client first
		if p.httpClient != nil {
			token, _, err := p.tokenExchanger.ExchangeWithClient(ctx, p.httpClient, accessToken)
			if err == nil {
				return token, nil
			}
			// Fall back to default exchange
		}

		token, _, err := p.tokenExchanger.Exchange(ctx, accessToken)
		if err != nil {
			return nil, fmt.Errorf("token exchange failed: %w", err)
		}
		return token, nil
	})
	if err != nil {
		return "", err
	}

	token, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return token, nil
}

// getAccessTokenWithRefresh returns the access token, refreshing if needed.
func (p *DeviceFlowProvider) getAccessTokenWithRefresh(ctx context.Context) (string, error) {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if creds == nil {
		return "", errors.New("credentials is nil")
	}

	// If not expired, return current token
	now := time.Now()
	if !creds.IsExpired(now) {
		return creds.AccessToken, nil
	}

	// Need to refresh
	v, err, _ := p.sf.Do("refresh", func() (any, error) {
		p.mu.RLock()
		current := p.creds
		onRefreshed := p.onRefreshed
		p.mu.RUnlock()

		if current == nil {
			return nil, errors.New("credentials is nil")
		}

		if current.RefreshToken == "" {
			return nil, errors.New("refresh_token is empty")
		}

		// Double-check expiration inside singleflight
		if !current.IsExpired(time.Now()) {
			return current.AccessToken, nil
		}

		fresh, err := p.refresh(ctx, current)
		if err != nil {
			return nil, err
		}

		p.mu.Lock()
		p.creds = fresh
		p.mu.Unlock()

		if onRefreshed != nil {
			if err := onRefreshed(ctx, fresh); err != nil {
				slog.WarnContext(ctx, "failed to persist refreshed credentials", slog.Any("error", err))
			}
		}

		return fresh.AccessToken, nil
	})
	if err != nil {
		return "", err
	}

	token, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return token, nil
}

// GetCredentials returns a copy of the current OAuth credentials.
// Returns nil if no credentials are stored.
func (p *DeviceFlowProvider) GetCredentials() *OAuthCredentials {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.creds != nil {
		c := *p.creds
		return &c
	}
	return nil
}

// UpdateCredentials updates the stored OAuth credentials.
// This is called when new credentials are obtained (e.g., after device flow completes).
// Stores a shallow copy to prevent concurrent mutation.
func (p *DeviceFlowProvider) UpdateCredentials(creds *OAuthCredentials) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if creds != nil {
		c := *creds
		p.creds = &c
	} else {
		p.creds = nil
	}
}

// StartAutoRefresh starts automatic background token refresh.
// The token will be refreshed before it expires based on the RefreshBefore option.
func (p *DeviceFlowProvider) StartAutoRefresh(ctx context.Context, opts AutoRefreshOptions) {
	slog.DebugContext(ctx, "start auto refresh device flow provider")

	fallbackInterval := opts.Interval
	if fallbackInterval <= 0 {
		fallbackInterval = 1 * time.Minute
	}

	refreshBefore := opts.RefreshBefore
	if refreshBefore <= 0 {
		refreshBefore = 5 * time.Minute
	}

	p.autoMu.Lock()

	if p.autoCancel != nil {
		p.autoMu.Unlock()
		return
	}

	autoCtx, cancel := context.WithCancel(ctx)
	p.autoCancel = cancel
	p.autoExecutor = executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1))
	exec := p.autoExecutor
	p.autoMu.Unlock()

	p.scheduleNextAutoRefresh(autoCtx, exec, refreshBefore, fallbackInterval, true)
}

// StopAutoRefresh stops automatic token refresh.
func (p *DeviceFlowProvider) StopAutoRefresh() {
	slog.DebugContext(context.Background(), "stop auto refresh device flow provider")

	p.autoMu.Lock()
	cancel := p.autoCancel
	exec := p.autoExecutor
	taskCancel := p.autoTaskCancel
	p.autoCancel = nil
	p.autoExecutor = nil
	p.autoTaskCancel = nil
	p.autoMu.Unlock()

	if cancel != nil {
		cancel()
	}

	if taskCancel != nil {
		taskCancel()
	}

	if exec != nil {
		if err := exec.Shutdown(context.Background()); err != nil {
			slog.WarnContext(context.Background(), "failed to shutdown device flow auto refresh executor", slog.Any("error", err))
		}
	}
}

// scheduleNextAutoRefresh schedules the next auto-refresh check.
func (p *DeviceFlowProvider) scheduleNextAutoRefresh(
	autoCtx context.Context,
	exec executors.ScheduledExecutor,
	refreshBefore time.Duration,
	fallbackInterval time.Duration,
	runImmediately bool,
) {
	if autoCtx.Err() != nil {
		return
	}

	delay := time.Duration(0)
	if !runImmediately {
		delay = p.nextAutoRefreshDelay(refreshBefore, fallbackInterval)
	}

	p.autoMu.Lock()

	if p.autoCancel == nil || p.autoExecutor == nil || exec != p.autoExecutor {
		p.autoMu.Unlock()
		return
	}

	prevCancel := p.autoTaskCancel
	p.autoTaskCancel = nil
	p.autoMu.Unlock()

	if prevCancel != nil {
		prevCancel()
	}

	cancelFunc, err := exec.ScheduleFunc(func(_ context.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(autoCtx, "auto refresh device flow provider panicked", slog.Any("cause", r))
			}
		}()

		if autoCtx.Err() != nil {
			return
		}

		// Ensure credentials are fresh
		if _, err := p.GetToken(autoCtx); err != nil {
			slog.WarnContext(autoCtx, "failed to auto refresh device flow token", slog.Any("error", err))
		}

		if autoCtx.Err() != nil {
			return
		}

		p.scheduleNextAutoRefresh(autoCtx, exec, refreshBefore, fallbackInterval, false)
	}, delay)
	if err != nil {
		p.StopAutoRefresh()
		return
	}

	p.autoMu.Lock()

	if p.autoCancel == nil || p.autoExecutor == nil || exec != p.autoExecutor {
		p.autoMu.Unlock()
		cancelFunc()
		return
	}

	p.autoTaskCancel = cancelFunc
	p.autoMu.Unlock()
}

// nextAutoRefreshDelay calculates the delay until the next refresh.
func (p *DeviceFlowProvider) nextAutoRefreshDelay(refreshBefore time.Duration, fallbackInterval time.Duration) time.Duration {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if fallbackInterval <= 0 {
		fallbackInterval = 1 * time.Minute
	}

	if creds == nil || creds.RefreshToken == "" || creds.ExpiresAt.IsZero() {
		return fallbackInterval
	}

	target := creds.ExpiresAt.Add(-refreshBefore)

	delay := time.Until(target)
	if delay < 0 {
		return 0
	}

	return delay
}

// refresh performs the OAuth2 token refresh flow.
func (p *DeviceFlowProvider) refresh(ctx context.Context, creds *OAuthCredentials) (*OAuthCredentials, error) {
	if creds == nil {
		return nil, errors.New("nil credentials")
	}

	if creds.RefreshToken == "" {
		return nil, errors.New("refresh_token is empty")
	}

	if p.config.TokenURL == "" {
		return nil, errors.New("token URL is empty")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", p.config.ClientID)
	form.Set("refresh_token", creds.RefreshToken)

	header := http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
		"Accept":       []string{"application/json"},
	}
	if p.config.UserAgent != "" {
		header.Set("User-Agent", p.config.UserAgent)
	}

	req := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     p.config.TokenURL,
		Headers: header,
		Body:    []byte(form.Encode()),
	}

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}

	refreshed, err := ParseTokenResponse(resp.Body, p.config.ClientID)
	if err != nil {
		return nil, err
	}

	// Preserve refresh token if not returned in response
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = creds.RefreshToken
	}

	slog.DebugContext(ctx, "device flow token refreshed", slog.String("expires_at", refreshed.ExpiresAt.Format(time.RFC3339)))

	return refreshed, nil
}
