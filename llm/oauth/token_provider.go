package oauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/zhenzou/executors"
	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/llm/httpclient"
)

type OAuthUrls struct {
	AuthorizeUrl string
	TokenUrl     string
}

type TokenGetter interface {
	Get(ctx context.Context) (*OAuthCredentials, error)
}

// TokenProvider manages OAuth2 credentials for a transformer instance.
// Each transformer has its own provider, so we can keep the credentials in memory.
type TokenProvider struct {
	httpClient  *httpclient.HttpClient
	oauthUrls   OAuthUrls
	strategy    ExchangeStrategy
	sf          singleflight.Group
	mu          sync.RWMutex
	creds       *OAuthCredentials
	userAgent   string
	onRefreshed func(ctx context.Context, refreshed *OAuthCredentials) error

	autoMu         sync.Mutex
	autoCancel     context.CancelFunc
	autoExecutor   executors.ScheduledExecutor
	autoTaskCancel executors.CancelFunc
}

type TokenProviderParams struct {
	Credentials *OAuthCredentials
	// HTTPClient should be pre-configured with proxy settings if needed
	HTTPClient  *httpclient.HttpClient
	OAuthUrls   OAuthUrls
	UserAgent   string
	OnRefreshed func(ctx context.Context, refreshed *OAuthCredentials) error
	// ExchangeStrategy defines how to format token requests (form-encoded or JSON)
	// If not provided, defaults to FormEncodedStrategy
	ExchangeStrategy ExchangeStrategy
}
type ExchangeParams struct {
	Code         string
	CodeVerifier string
	ClientID     string
	RedirectURI  string
	State        string // Optional: for providers that require state in token exchange
}

type AutoRefreshOptions struct {
	Interval      time.Duration
	RefreshBefore time.Duration
}

func NewTokenProvider(params TokenProviderParams) *TokenProvider {
	strategy := params.ExchangeStrategy
	if strategy == nil {
		strategy = &FormEncodedStrategy{UserAgent: params.UserAgent}
	}

	return &TokenProvider{
		httpClient:  params.HTTPClient,
		oauthUrls:   params.OAuthUrls,
		strategy:    strategy,
		userAgent:   params.UserAgent,
		creds:       params.Credentials,
		onRefreshed: params.OnRefreshed,
	}
}

// Exchange performs OAuth2 authorization_code exchange and returns credentials.
func (p *TokenProvider) Exchange(ctx context.Context, params ExchangeParams) (*OAuthCredentials, error) {
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

	req, err := p.strategy.BuildExchangeRequest(params, p.oauthUrls.TokenUrl)
	if err != nil {
		return nil, fmt.Errorf("build exchange request: %w", err)
	}

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	creds, err := ParseTokenResponse(resp.Body, params.ClientID)
	if err != nil {
		// Wrap the error to indicate this was an exchange operation
		if strings.Contains(err.Error(), "token request failed:") {
			return nil, fmt.Errorf("token exchange failed: %s", strings.TrimPrefix(err.Error(), "token request failed: "))
		}
		return nil, err
	}

	p.mu.Lock()
	p.creds = creds
	p.mu.Unlock()

	return creds, nil
}

// Get returns valid OAuth2 credentials.
// It refreshes them if expired.
func (p *TokenProvider) Get(ctx context.Context) (*OAuthCredentials, error) {
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
				slog.WarnContext(ctx, "failed to persist refreshed credentials", slog.Any("error", err))
			}
		}

		return fresh, nil
	})
	if err != nil {
		return nil, err
	}

	fresh, ok := v.(*OAuthCredentials)
	if !ok {
		return nil, fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return fresh, nil
}

func (p *TokenProvider) EnsureFresh(ctx context.Context, refreshBefore time.Duration) (*OAuthCredentials, error) {
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
				slog.WarnContext(ctx, "failed to persist refreshed credentials", slog.Any("error", err))
			}
		}

		return fresh, nil
	})
	if err != nil {
		return nil, err
	}

	fresh, ok := v.(*OAuthCredentials)
	if !ok {
		return nil, fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return fresh, nil
}

func (p *TokenProvider) StartAutoRefresh(ctx context.Context, opts AutoRefreshOptions) {
	slog.DebugContext(ctx, "start auto refresh token provider")

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

func (p *TokenProvider) StopAutoRefresh() {
	slog.DebugContext(context.Background(), "stop auto refresh token provider")

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
			slog.WarnContext(context.Background(), "failed to shutdown token provider auto refresh executor", slog.Any("error", err))
		}
	}
}

func (p *TokenProvider) scheduleNextAutoRefresh(
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
				slog.ErrorContext(autoCtx, "auto refresh token provider panicked", slog.Any("cause", r))
			}
		}()

		if autoCtx.Err() != nil {
			return
		}

		if _, err := p.EnsureFresh(autoCtx, refreshBefore); err != nil {
			slog.WarnContext(autoCtx, "failed to auto refresh token", slog.Any("error", err))
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

func (p *TokenProvider) nextAutoRefreshDelay(refreshBefore time.Duration, fallbackInterval time.Duration) time.Duration {
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

// StaticTokenProvider provides a fixed set of credentials.
type StaticTokenProvider struct {
	creds *OAuthCredentials
}

func NewStaticTokenProvider(creds *OAuthCredentials) *StaticTokenProvider {
	return &StaticTokenProvider{creds: creds}
}

func (p *StaticTokenProvider) Get(ctx context.Context) (*OAuthCredentials, error) {
	return p.creds, nil
}

// APIKeyProviderFunc is a function type that implements auth.APIKeyProvider interface.
type APIKeyProviderFunc func(ctx context.Context) string

func (f APIKeyProviderFunc) Get(ctx context.Context) string {
	return f(ctx)
}

// APIKeyTokenProvider adapts an APIKeyProvider to a TokenGetter.
// This allows transformers that expect OAuth tokens to work with regular API keys.
type APIKeyTokenProvider struct {
	provider APIKeyProviderFunc
}

// NewAPIKeyTokenProvider creates a new APIKeyTokenProvider from an APIKeyProvider function.
func NewAPIKeyTokenProvider(provider APIKeyProviderFunc) *APIKeyTokenProvider {
	return &APIKeyTokenProvider{provider: provider}
}

// Get implements TokenGetter by returning the API key as an OAuthCredentials.
func (p *APIKeyTokenProvider) Get(ctx context.Context) (*OAuthCredentials, error) {
	apiKey := p.provider(ctx)
	if apiKey == "" {
		return nil, errors.New("api key is empty")
	}

	return &OAuthCredentials{
		AccessToken: apiKey,
	}, nil
}

// refresh performs the OAuth2 token refresh flow.
func (p *TokenProvider) refresh(ctx context.Context, creds *OAuthCredentials) (*OAuthCredentials, error) {
	if creds == nil {
		return nil, errors.New("nil credentials")
	}

	if creds.RefreshToken == "" {
		return nil, errors.New("refresh_token is empty")
	}

	if p.oauthUrls.TokenUrl == "" {
		return nil, errors.New("token URL is empty")
	}

	req, err := p.strategy.BuildRefreshRequest(creds, p.oauthUrls.TokenUrl)
	if err != nil {
		return nil, fmt.Errorf("build refresh request: %w", err)
	}

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	refreshed, err := ParseTokenResponse(resp.Body, creds.ClientID)
	if err != nil {
		return nil, err
	}

	// Preserve refresh token if not returned in response
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = creds.RefreshToken
	}

	slog.DebugContext(ctx, "oauth token refreshed", slog.String("expires_at", refreshed.ExpiresAt.Format(time.RFC3339)))

	return refreshed, nil
}
