package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/llm/httpclient"
)

func wrapHttpError(err error) error {
	if err == nil {
		return nil
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && len(httpErr.Body) > 0 {
		return fmt.Errorf("%w (response body: %s)", err, string(httpErr.Body))
	}

	return err
}

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

	autoMu     sync.Mutex
	autoCancel context.CancelFunc
	autoSeq    uint64
}

// CredentialConflictError reports that persistence found newer channel credentials.
// Credentials is the winner; nil means the channel is no longer OAuth and the
// refreshing provider must retire its automatic refresh loop.
type CredentialConflictError struct {
	Credentials *OAuthCredentials
}

func (e *CredentialConflictError) Error() string {
	return "OAuth credentials changed while refreshing"
}

type refreshResult struct {
	credentials  *OAuthCredentials
	retryExpired bool
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

const (
	// autoRefreshFailureMaxMultiplier bounds exponential failure backoff
	// relative to the configured fallback interval.
	autoRefreshFailureMaxMultiplier = 4
)

type autoRefreshConfig struct {
	refreshBefore    time.Duration
	fallbackInterval time.Duration
	failureBase      time.Duration
	failureMax       time.Duration
}

type autoRefreshOutcome int

const (
	autoRefreshSucceeded autoRefreshOutcome = iota
	autoRefreshTransientFailure
	autoRefreshPermanentFailure
	autoRefreshStopped
)

// autoRefreshBackoff tracks consecutive transient failures and produces a
// bounded retry delay. Jitter is applied only below the cap; at the cap the
// delay stays fixed.
type autoRefreshBackoff struct {
	base                time.Duration
	max                 time.Duration
	consecutiveFailures int
}

func newAutoRefreshBackoff(base, max time.Duration) *autoRefreshBackoff {
	if base <= 0 {
		base = 1 * time.Minute
	}

	if max <= 0 {
		max = base * autoRefreshFailureMaxMultiplier
	}

	if max < base {
		max = base
	}

	return &autoRefreshBackoff{base: base, max: max}
}

func (b *autoRefreshBackoff) reset() {
	b.consecutiveFailures = 0
}

func (b *autoRefreshBackoff) next() time.Duration {
	b.consecutiveFailures++

	delay := b.base
	for i := 1; i < b.consecutiveFailures && delay < b.max; i++ {
		delay *= 2
	}

	if delay > b.max {
		delay = b.max
	}

	return jitterAutoRefreshDelay(delay, b.max)
}

// jitterAutoRefreshDelay adds up to 25% positive jitter so multiple providers
// do not retry in lockstep. Jitter never makes a retry earlier than delay and
// never exceeds max; once delay has reached max the retry is the fixed cap.
func jitterAutoRefreshDelay(delay, max time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}

	span := delay / 4
	if span <= 0 {
		return delay
	}

	jittered := delay + time.Duration(rand.Int64N(int64(span)+1))
	if max > 0 && jittered > max {
		return max
	}

	return jittered
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

// SetHTTPClient updates the transport used by future token requests.
// An in-flight request keeps the client it already started with.
func (p *TokenProvider) SetHTTPClient(client *httpclient.HttpClient) {
	p.mu.Lock()
	p.httpClient = client
	p.mu.Unlock()
}

func (p *TokenProvider) getHTTPClient() *httpclient.HttpClient {
	p.mu.RLock()
	client := p.httpClient
	p.mu.RUnlock()
	return client
}

// Exchange performs OAuth2 authorization_code exchange and returns credentials.
func (p *TokenProvider) Exchange(ctx context.Context, params ExchangeParams) (*OAuthCredentials, error) {
	httpClient := p.getHTTPClient()
	if httpClient == nil {
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

	resp, err := httpClient.Do(ctx, req)
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
	for {
		v, err, _ := p.sf.Do("refresh", func() (any, error) {
			p.mu.RLock()
			current := p.creds
			onRefreshed := p.onRefreshed
			p.mu.RUnlock()

			if current == nil {
				return nil, fmt.Errorf("credentials is nil")
			}

			if !current.IsExpired(time.Now()) {
				return refreshResult{credentials: current}, nil
			}

			fresh, err := p.refresh(ctx, current)
			if err != nil {
				return nil, err
			}

			retryExpired := false
			if onRefreshed != nil {
				if err := onRefreshed(ctx, fresh); err != nil {
					var conflict *CredentialConflictError
					if errors.As(err, &conflict) {
						fresh = conflict.Credentials
						retryExpired = fresh != nil
					} else {
						slog.WarnContext(ctx, "failed to persist refreshed credentials", slog.Any("error", err))
					}
				}
			}

			p.mu.Lock()
			p.creds = fresh
			p.mu.Unlock()
			if fresh == nil {
				p.StopAutoRefresh()
				return nil, errors.New("credentials is nil")
			}

			return refreshResult{credentials: fresh, retryExpired: retryExpired}, nil
		})
		if err != nil {
			return nil, err
		}

		result, ok := v.(refreshResult)
		if !ok {
			return nil, fmt.Errorf("singleflight returned unexpected type %T", v)
		}
		fresh := result.credentials
		if result.retryExpired && fresh.RefreshToken != "" && fresh.IsExpired(time.Now()) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			continue
		}

		return fresh, nil
	}
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

	for {
		v, err, _ := p.sf.Do("refresh", func() (any, error) {
			p.mu.RLock()
			current := p.creds
			onRefreshed := p.onRefreshed
			p.mu.RUnlock()

			if current == nil {
				return nil, fmt.Errorf("credentials is nil")
			}

			if current.RefreshToken == "" {
				return refreshResult{credentials: current}, nil
			}

			n := time.Now()

			need := current.ExpiresAt.IsZero() || n.Add(refreshBefore).After(current.ExpiresAt)
			if !need {
				return refreshResult{credentials: current}, nil
			}

			fresh, err := p.refresh(ctx, current)
			if err != nil {
				return nil, err
			}

			retryExpired := false
			if onRefreshed != nil {
				if err := onRefreshed(ctx, fresh); err != nil {
					var conflict *CredentialConflictError
					if errors.As(err, &conflict) {
						fresh = conflict.Credentials
						retryExpired = fresh != nil
					} else {
						slog.WarnContext(ctx, "failed to persist refreshed credentials", slog.Any("error", err))
					}
				}
			}

			p.mu.Lock()
			p.creds = fresh
			p.mu.Unlock()
			if fresh == nil {
				p.StopAutoRefresh()
				return nil, errors.New("credentials is nil")
			}

			return refreshResult{credentials: fresh, retryExpired: retryExpired}, nil
		})
		if err != nil {
			return nil, err
		}

		result, ok := v.(refreshResult)
		if !ok {
			return nil, fmt.Errorf("singleflight returned unexpected type %T", v)
		}
		fresh := result.credentials
		if result.retryExpired && fresh.RefreshToken != "" && fresh.IsExpired(time.Now()) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			continue
		}

		return fresh, nil
	}
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

	cfg := autoRefreshConfig{
		refreshBefore:    refreshBefore,
		fallbackInterval: fallbackInterval,
		failureBase:      fallbackInterval,
		failureMax:       fallbackInterval * autoRefreshFailureMaxMultiplier,
	}

	p.autoMu.Lock()

	if p.autoCancel != nil {
		p.autoMu.Unlock()
		return
	}

	autoCtx, cancel := context.WithCancel(ctx)
	p.autoSeq++
	id := p.autoSeq
	p.autoCancel = cancel
	p.autoMu.Unlock()

	go p.runAutoRefresh(autoCtx, cancel, id, cfg)
}

func (p *TokenProvider) StopAutoRefresh() {
	slog.DebugContext(context.Background(), "stop auto refresh token provider")

	p.autoMu.Lock()
	cancel := p.autoCancel
	p.autoCancel = nil
	p.autoMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (p *TokenProvider) runAutoRefresh(
	autoCtx context.Context,
	cancel context.CancelFunc,
	id uint64,
	cfg autoRefreshConfig,
) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(autoCtx, "auto refresh token provider goroutine panicked", slog.Any("cause", r))
		}
	}()

	backoff := newAutoRefreshBackoff(cfg.failureBase, cfg.failureMax)
	delay := time.Duration(0)

	for {
		if !sleepForAutoRefresh(autoCtx, delay) {
			return
		}

		switch p.runAutoRefreshOnce(autoCtx, cfg.refreshBefore) {
		case autoRefreshStopped:
			return
		case autoRefreshPermanentFailure:
			p.retireAutoRefresh(id, cancel)
			return
		case autoRefreshTransientFailure:
			delay = backoff.next()
		case autoRefreshSucceeded:
			backoff.reset()
			delay = p.nextAutoRefreshDelay(cfg.refreshBefore, cfg.fallbackInterval)
		default:
			return
		}
	}
}

// retireAutoRefresh clears the loop state only when the finishing loop is still
// the active one, so a concurrently started loop keeps its own cancellation.
func (p *TokenProvider) retireAutoRefresh(id uint64, cancel context.CancelFunc) {
	p.autoMu.Lock()
	if p.autoSeq == id && p.autoCancel != nil {
		p.autoCancel = nil
	}
	p.autoMu.Unlock()

	cancel()
}

func (p *TokenProvider) runAutoRefreshOnce(autoCtx context.Context, refreshBefore time.Duration) (outcome autoRefreshOutcome) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(autoCtx, "auto refresh token provider panicked", slog.Any("cause", r))
			outcome = autoRefreshStopped
		}
	}()

	if autoCtx.Err() != nil {
		return autoRefreshStopped
	}

	if _, err := p.EnsureFresh(autoCtx, refreshBefore); err != nil {
		if autoCtx.Err() != nil {
			return autoRefreshStopped
		}

		attrs := []slog.Attr{slog.String("error", sanitizedOAuthRefreshError(err))}

		if code := oauthRefreshErrorCode(err); code != "" {
			attrs = append(attrs, slog.String("oauth_error", safeOAuthErrorCode(code)))
		}

		if isPermanentOAuthRefreshError(err) {
			slog.LogAttrs(autoCtx, slog.LevelWarn, "oauth token refresh rejected by provider; stopping automatic refresh", attrs...)

			return autoRefreshPermanentFailure
		}

		slog.LogAttrs(autoCtx, slog.LevelWarn, "failed to auto refresh token", attrs...)

		return autoRefreshTransientFailure
	}

	if autoCtx.Err() != nil {
		return autoRefreshStopped
	}

	return autoRefreshSucceeded
}

func sleepForAutoRefresh(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
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

// permanentOAuthRefreshErrors are OAuth 2.0 error codes that mean the stored
// credentials are unusable and cannot be repaired by retrying. A bare HTTP 401
// is deliberately not enough: proxies and transient provider faults can return
// it without invalidating the refresh token.
var permanentOAuthRefreshErrors = map[string]struct{}{
	"invalid_grant":       {},
	"invalid_client":      {},
	"unauthorized_client": {},
}

// unknownOAuthErrorCode replaces any provider-supplied code that is not in the
// safe enum, so an error body cannot smuggle secrets into logs.
const unknownOAuthErrorCode = "unknown_oauth_error"

func safeOAuthErrorCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if _, ok := permanentOAuthRefreshErrors[code]; ok {
		return code
	}

	return unknownOAuthErrorCode
}

func isPermanentOAuthRefreshError(err error) bool {
	code := oauthRefreshErrorCode(err)
	if code == "" {
		return false
	}

	_, ok := permanentOAuthRefreshErrors[code]

	return ok
}

func oauthRefreshErrorCode(err error) string {
	var tokenErr *tokenResponseError
	if errors.As(err, &tokenErr) {
		return strings.ToLower(strings.TrimSpace(tokenErr.Code))
	}

	var httpErr *httpclient.Error
	if !errors.As(err, &httpErr) {
		return ""
	}

	switch httpErr.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
	default:
		return ""
	}

	return oauthErrorCodeFromBody(httpErr.Body)
}

func oauthErrorCodeFromBody(body []byte) string {
	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || len(payload.Error) == 0 {
		return ""
	}

	var code string
	if err := json.Unmarshal(payload.Error, &code); err == nil {
		return strings.ToLower(strings.TrimSpace(code))
	}

	var obj struct {
		Code string `json:"code"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload.Error, &obj); err == nil {
		code = obj.Code
		if code == "" {
			code = obj.Type
		}

		return strings.ToLower(strings.TrimSpace(code))
	}

	return ""
}

// sanitizedOAuthRefreshError keeps the useful status information while leaving
// out response bodies, which may echo credentials back from the provider.
func sanitizedOAuthRefreshError(err error) string {
	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) {
		return httpErr.Error()
	}

	var tokenErr *tokenResponseError
	if errors.As(err, &tokenErr) {
		return fmt.Sprintf("token request failed: %s", safeOAuthErrorCode(tokenErr.Code))
	}

	return err.Error()
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

	httpClient := p.getHTTPClient()
	if httpClient == nil {
		return nil, errors.New("http client is nil")
	}

	req, err := p.strategy.BuildRefreshRequest(creds, p.oauthUrls.TokenUrl)
	if err != nil {
		return nil, fmt.Errorf("build refresh request: %w", err)
	}

	resp, err := httpClient.Do(ctx, req)
	if err != nil {
		return nil, wrapHttpError(err)
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
