package copilot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

const (
	defaultCopilotTokenEndpoint = "https://" + "api.github.com" + "/copilot_internal/v2/token"
	tokenExpiryBuffer           = 5 * time.Minute
	tokenExchangeTimeout        = 30 * time.Second
)

type copilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type copilotTokenCacheEntry struct {
	copilotToken string
	expiresAt    time.Time
	cachedAt     time.Time
}

func (e *copilotTokenCacheEntry) isExpired(now time.Time) bool {
	if e == nil || e.expiresAt.IsZero() {
		return true
	}

	return now.After(e.expiresAt.Add(-tokenExpiryBuffer))
}

type TokenExchangerParams struct {
	HTTPClient *httpclient.HttpClient
	Endpoint   string
}

type TokenExchanger struct {
	httpClient *httpclient.HttpClient
	endpoint   string

	mu    sync.RWMutex
	cache map[string]*copilotTokenCacheEntry
	sf    singleflight.Group
}

var _ oauth.TokenExchanger = (*TokenExchanger)(nil)

func NewTokenExchanger(params TokenExchangerParams) *TokenExchanger {
	hc := params.HTTPClient
	if hc == nil {
		hc = httpclient.NewHttpClient()
	}

	endpoint := params.Endpoint
	if endpoint == "" {
		endpoint = defaultCopilotTokenEndpoint
	}

	return &TokenExchanger{
		httpClient: hc,
		endpoint:   endpoint,
		cache:      make(map[string]*copilotTokenCacheEntry),
	}
}

func (e *TokenExchanger) Exchange(ctx context.Context, accessToken string) (string, int64, error) {
	return e.ExchangeWithClient(ctx, nil, accessToken)
}

func (e *TokenExchanger) ExchangeWithClient(ctx context.Context, httpClient *httpclient.HttpClient, accessToken string) (string, int64, error) {
	if accessToken == "" {
		return "", 0, errors.New("access token is empty")
	}

	client := httpClient
	if client == nil {
		client = e.httpClient
	}

	cacheKey := tokenCacheKey(accessToken)

	e.mu.RLock()
	entry, ok := e.cache[cacheKey]
	e.mu.RUnlock()

	if ok && !entry.isExpired(time.Now()) {
		if slog.Default().Enabled(ctx, slog.LevelDebug) {
			slog.DebugContext(ctx, "copilot token cache hit",
				slog.Time("expires_at", entry.expiresAt),
				slog.Time("cached_at", entry.cachedAt),
			)
		}

		return entry.copilotToken, entry.expiresAt.Unix(), nil
	}

	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		slog.DebugContext(ctx, "copilot token cache miss or expired, performing exchange",
			slog.String("endpoint", e.endpoint),
		)
	}

	sfKey := exchangeSingleflightKey(accessToken, client, e.endpoint)

	v, err, _ := e.sf.Do(sfKey, func() (any, error) {
		// Avoid one caller's cancellation canceling the shared in-flight exchange.
		return e.exchange(context.WithoutCancel(ctx), client, accessToken)
	})
	if err != nil {
		return "", 0, err
	}

	resp, ok := v.(*copilotTokenResponse)
	if !ok {
		return "", 0, fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	now := time.Now()
	expiresAt := time.Unix(resp.ExpiresAt, 0)

	e.mu.Lock()
	e.cache[cacheKey] = &copilotTokenCacheEntry{
		copilotToken: resp.Token,
		expiresAt:    expiresAt,
		cachedAt:     now,
	}
	e.mu.Unlock()

	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		slog.DebugContext(ctx, "copilot token exchanged and cached",
			slog.Time("expires_at", expiresAt),
		)
	}

	return resp.Token, resp.ExpiresAt, nil
}

func tokenCacheKey(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return hex.EncodeToString(sum[:])
}

func exchangeSingleflightKey(accessToken string, client *httpclient.HttpClient, endpoint string) string {
	return tokenCacheKey(accessToken) + ":" + fmt.Sprintf("%p", client) + ":" + endpoint
}

func (e *TokenExchanger) exchange(ctx context.Context, httpClient *httpclient.HttpClient, accessToken string) (*copilotTokenResponse, error) {
	req := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(e.endpoint).
		WithHeader("Authorization", "token "+accessToken).
		WithHeader("Accept", "application/json").
		Build()

	exchangeCtx, cancel := context.WithTimeout(ctx, tokenExchangeTimeout)
	defer cancel()

	resp, err := httpClient.Do(exchangeCtx, req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange returned non-2xx status: %d", resp.StatusCode)
	}

	var tokenResp copilotTokenResponse
	if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.Token == "" {
		return nil, errors.New("copilot token is empty in response")
	}

	if tokenResp.ExpiresAt == 0 {
		return nil, errors.New("expires_at is missing in response")
	}

	return &tokenResp, nil
}
