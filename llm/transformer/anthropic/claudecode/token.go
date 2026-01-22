package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zhenzou/executors"
	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

// DefaultTokenURLs are the production Claude OAuth endpoints.
var DefaultTokenURLs = oauth.OAuthUrls{
	AuthorizeUrl: AuthorizeURL,
	TokenUrl:     TokenURL,
}

type TokenProviderParams struct {
	Credentials *oauth.OAuthCredentials
	HTTPClient  *httpclient.HttpClient
	OnRefreshed func(ctx context.Context, refreshed *oauth.OAuthCredentials) error
}

// ClaudeTokenProvider implements OAuth token management for Claude Code.
// Claude uses JSON format instead of form-encoded data.
type ClaudeTokenProvider struct {
	httpClient  *httpclient.HttpClient
	oauthUrls   oauth.OAuthUrls
	sf          singleflight.Group
	mu          sync.RWMutex
	creds       *oauth.OAuthCredentials
	userAgent   string
	onRefreshed func(ctx context.Context, refreshed *oauth.OAuthCredentials) error

	autoMu         sync.Mutex
	autoCancel     context.CancelFunc
	autoExecutor   executors.ScheduledExecutor
	autoTaskCancel executors.CancelFunc
}

// exchangeRequest is the JSON body for token exchange.
// exchangeRequest is the JSON body for token exchange.
type exchangeRequest struct {
	Code         string `json:"code"`
	State        string `json:"state,omitempty"` // Claude requires this
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
}

// refreshRequest is the JSON body for token refresh.
type refreshRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

// tokenResponse matches Claude's OAuth response format.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Organization struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	} `json:"organization"`
	Account struct {
		UUID         string `json:"uuid"`
		EmailAddress string `json:"email_address"`
	} `json:"account"`
}

// tokenError matches OAuth error response.
type tokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func NewTokenProvider(params TokenProviderParams) *ClaudeTokenProvider {
	return &ClaudeTokenProvider{
		httpClient:  params.HTTPClient,
		oauthUrls:   DefaultTokenURLs,
		userAgent:   UserAgent,
		creds:       params.Credentials,
		onRefreshed: params.OnRefreshed,
	}
}

// Exchange performs OAuth2 authorization_code exchange and returns credentials.
// Claude uses JSON format instead of form-encoded data.
func (p *ClaudeTokenProvider) Exchange(ctx context.Context, params oauth.ExchangeParams) (*oauth.OAuthCredentials, error) {
	if p.httpClient == nil {
		return nil, errors.New("http client is nil")
	}

	if p.oauthUrls.TokenUrl == "" {
		return nil, errors.New("token URL is empty")
	}

	if params.Code == "" {
		return nil, errors.New("code is empty")
	}

	if params.CodeVerifier == "" {
		return nil, errors.New("code_verifier is empty")
	}

	if params.ClientID == "" {
		return nil, errors.New("client_id is empty")
	}

	if params.RedirectURI == "" {
		return nil, errors.New("redirect_uri is empty")
	}

	reqBody := exchangeRequest{
		Code:         params.Code,
		State:        params.State,
		GrantType:    "authorization_code",
		ClientID:     params.ClientID,
		RedirectURI:  params.RedirectURI,
		CodeVerifier: params.CodeVerifier,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal exchange request: %w", err)
	}

	header := http.Header{
		"Content-Type": []string{"application/json"},
		"Accept":       []string{"application/json"},
	}
	if p.userAgent != "" {
		header.Set("User-Agent", p.userAgent)
	}

	req := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     p.oauthUrls.TokenUrl,
		Headers: header,
		Body:    bodyBytes,
	}

	// Log the request for debugging
	log.Info(ctx, "claude oauth exchange request",
		log.String("url", p.oauthUrls.TokenUrl),
		log.String("body", string(bodyBytes)))

	resp, err := p.httpClient.Do(ctx, req)

	// Handle HTTP errors (4xx, 5xx) - the httpclient returns error for these
	if err != nil {
		var httpErr *httpclient.Error
		if errors.As(err, &httpErr) {
			// Log the error response
			log.Info(ctx, "claude oauth exchange error response",
				log.Int("status", httpErr.StatusCode),
				log.String("body", string(httpErr.Body)))

			// Try to decode the error body
			var tokenErr tokenError
			if jsonErr := json.Unmarshal(httpErr.Body, &tokenErr); jsonErr == nil && tokenErr.Error != "" {
				return nil, fmt.Errorf("token exchange failed: %s - %s (status %d)",
					tokenErr.Error, tokenErr.ErrorDescription, httpErr.StatusCode)
			}
			return nil, fmt.Errorf("token exchange failed with status %d: %s", httpErr.StatusCode, string(httpErr.Body))
		}
		return nil, err
	}

	// Log the success response for debugging
	log.Info(ctx, "claude oauth exchange response",
		log.Int("status", resp.StatusCode),
		log.String("body", string(resp.Body)))

	var tokenResp tokenResponse
	if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode exchange response (status %d, body: %s): %w", resp.StatusCode, string(resp.Body), err)
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("token exchange response missing required fields (body: %s)", string(resp.Body))
	}

	now := time.Now()
	creds := &oauth.OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     params.ClientID,
		TokenType:    tokenResp.TokenType,
	}

	if tokenResp.Scope != "" {
		creds.Scopes = strings.Fields(tokenResp.Scope)
	}

	if tokenResp.ExpiresIn > 0 {
		creds.ExpiresAt = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	p.mu.Lock()
	p.creds = creds
	p.mu.Unlock()

	return creds, nil
}

// Get returns valid OAuth2 credentials. It refreshes them if expired.
func (p *ClaudeTokenProvider) Get(ctx context.Context) (*oauth.OAuthCredentials, error) {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if creds == nil {
		return nil, fmt.Errorf("credentials is nil")
	}

	now := time.Now()
	if !creds.IsExpired(now) {
		return creds, nil
	}

	// Refresh with singleflight to avoid stampede inside the same transformer.
	v, err, _ := p.sf.Do("refresh", func() (any, error) {
		p.mu.RLock()
		current := p.creds
		onRefreshed := p.onRefreshed
		p.mu.RUnlock()

		if current == nil {
			return nil, fmt.Errorf("credentials is nil")
		}

		if !current.IsExpired(time.Now()) {
			return current, nil
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
				log.Warn(ctx, "failed to persist refreshed credentials", log.Cause(err))
			}
		}

		return fresh, nil
	})
	if err != nil {
		return nil, err
	}

	fresh, ok := v.(*oauth.OAuthCredentials)
	if !ok {
		return nil, fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return fresh, nil
}

// refresh performs the OAuth2 token refresh flow using JSON format.
func (p *ClaudeTokenProvider) refresh(ctx context.Context, creds *oauth.OAuthCredentials) (*oauth.OAuthCredentials, error) {
	if creds == nil {
		return nil, errors.New("nil credentials")
	}

	if creds.RefreshToken == "" {
		return nil, errors.New("refresh_token is empty")
	}

	if p.oauthUrls.TokenUrl == "" {
		return nil, errors.New("token URL is empty")
	}

	reqBody := refreshRequest{
		GrantType:    "refresh_token",
		ClientID:     creds.ClientID,
		RefreshToken: creds.RefreshToken,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal refresh request: %w", err)
	}

	header := http.Header{
		"Content-Type": []string{"application/json"},
		"Accept":       []string{"application/json"},
	}
	if p.userAgent != "" {
		header.Set("User-Agent", p.userAgent)
	}

	req := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     p.oauthUrls.TokenUrl,
		Headers: header,
		Body:    bodyBytes,
	}

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		var tokenErr tokenError
		if err := json.Unmarshal(resp.Body, &tokenErr); err == nil && tokenErr.Error != "" {
			return nil, fmt.Errorf("token refresh failed: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
		}

		return nil, errors.New("token refresh response missing access_token")
	}

	now := time.Now()

	updated := *creds
	updated.AccessToken = tokenResp.AccessToken
	updated.TokenType = tokenResp.TokenType

	if tokenResp.RefreshToken != "" {
		updated.RefreshToken = tokenResp.RefreshToken
	}

	if tokenResp.Scope != "" {
		updated.Scopes = strings.Fields(tokenResp.Scope)
	}

	if tokenResp.ExpiresIn > 0 {
		updated.ExpiresAt = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	log.Debug(ctx, "oauth token refreshed", log.String("expires_at", updated.ExpiresAt.Format(time.RFC3339)))

	return &updated, nil
}

// StartAutoRefresh starts a background task that periodically refreshes the token.
func (p *ClaudeTokenProvider) StartAutoRefresh(ctx context.Context, opts oauth.AutoRefreshOptions) {
	if ctx == nil {
		ctx = context.Background()
	}

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

// StopAutoRefresh stops the background token refresh task.
func (p *ClaudeTokenProvider) StopAutoRefresh() {
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
			log.Warn(context.Background(), "failed to shutdown token provider auto refresh executor", log.Cause(err))
		}
	}
}

func (p *ClaudeTokenProvider) scheduleNextAutoRefresh(
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
				log.Error(autoCtx, "auto refresh token provider panicked", log.Any("cause", r))
			}
		}()

		if autoCtx.Err() != nil {
			return
		}

		if _, err := p.ensureFresh(autoCtx, refreshBefore); err != nil {
			log.Warn(autoCtx, "failed to auto refresh token", log.Cause(err))
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

func (p *ClaudeTokenProvider) nextAutoRefreshDelay(refreshBefore time.Duration, fallbackInterval time.Duration) time.Duration {
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

func (p *ClaudeTokenProvider) ensureFresh(ctx context.Context, refreshBefore time.Duration) (*oauth.OAuthCredentials, error) {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if creds == nil {
		return nil, fmt.Errorf("credentials is nil")
	}

	if creds.RefreshToken == "" {
		return creds, nil
	}

	if refreshBefore <= 0 {
		refreshBefore = 5 * time.Minute
	}

	now := time.Now()

	shouldRefresh := creds.ExpiresAt.IsZero() || now.Add(refreshBefore).After(creds.ExpiresAt)
	if !shouldRefresh {
		return creds, nil
	}

	v, err, _ := p.sf.Do("refresh", func() (any, error) {
		p.mu.RLock()
		current := p.creds
		onRefreshed := p.onRefreshed
		p.mu.RUnlock()

		if current == nil {
			return nil, fmt.Errorf("credentials is nil")
		}

		if current.RefreshToken == "" {
			return current, nil
		}

		n := time.Now()

		need := current.ExpiresAt.IsZero() || n.Add(refreshBefore).After(current.ExpiresAt)
		if !need {
			return current, nil
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
				log.Warn(ctx, "failed to persist refreshed credentials", log.Cause(err))
			}
		}

		return fresh, nil
	})
	if err != nil {
		return nil, err
	}

	fresh, ok := v.(*oauth.OAuthCredentials)
	if !ok {
		return nil, fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return fresh, nil
}
