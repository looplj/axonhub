package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	CopilotTokenEndpoint = "https://api.github.com/copilot_internal/v2/token" //nolint:gosec
	TokenExpiryBuffer    = 5 * time.Minute
	TokenExchangeTimeout = 30 * time.Second
)

type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type CopilotTokenCacheEntry struct {
	Token        string
	CopilotToken string
	ExpiresAt    time.Time
	CachedAt     time.Time
}

func (e *CopilotTokenCacheEntry) IsExpired(now time.Time) bool {
	if e == nil || e.ExpiresAt.IsZero() {
		return true
	}
	return now.After(e.ExpiresAt.Add(-TokenExpiryBuffer))
}

type CopilotTokenExchanger struct {
	httpClient *httpclient.HttpClient
	cache      map[string]*CopilotTokenCacheEntry
	mu         sync.RWMutex
	sf         singleflight.Group
}

func NewCopilotTokenExchanger(httpClient *httpclient.HttpClient) *CopilotTokenExchanger {
	return &CopilotTokenExchanger{
		httpClient: httpClient,
		cache:      make(map[string]*CopilotTokenCacheEntry),
	}
}

func (e *CopilotTokenExchanger) GetToken(ctx context.Context, accessToken string) (string, int64, error) {
	return e.GetTokenWithClient(ctx, nil, accessToken)
}

func (e *CopilotTokenExchanger) GetTokenWithClient(ctx context.Context, httpClient *httpclient.HttpClient, accessToken string) (string, int64, error) {
	if accessToken == "" {
		return "", 0, errors.New("access token is empty")
	}
	// Use the provided httpClient if given, otherwise fall back to the default
	client := httpClient
	if client == nil {
		client = e.httpClient
	}
	e.mu.RLock()
	entry, exists := e.cache[accessToken]
	e.mu.RUnlock()
	if exists && !entry.IsExpired(time.Now()) {
		log.Debug(ctx, "copilot token cache hit",
			log.Time("expires_at", entry.ExpiresAt),
			log.Time("cached_at", entry.CachedAt),
		)
		return entry.CopilotToken, entry.ExpiresAt.Unix(), nil
	}
	log.Debug(ctx, "copilot token cache miss or expired, performing exchange")
	return e.refreshTokenWithClient(ctx, client, accessToken)
}

func (e *CopilotTokenExchanger) RefreshToken(ctx context.Context, accessToken string) (string, int64, error) {
	return e.refreshTokenWithClient(ctx, e.httpClient, accessToken)
}

func (e *CopilotTokenExchanger) refreshTokenWithClient(ctx context.Context, httpClient *httpclient.HttpClient, accessToken string) (string, int64, error) {
	if accessToken == "" {
		return "", 0, errors.New("access token is empty")
	}
	// Use composite key including httpClient to ensure different proxy configs get separate deduplication
	sfKey := accessToken + ":" + fmt.Sprintf("%p", httpClient)

	v, err, _ := e.sf.Do(sfKey, func() (any, error) {
		// Use background context to avoid cancellation propagation to all waiters
		return e.exchangeWithClient(context.Background(), httpClient, accessToken)
	})
	if err != nil {
		return "", 0, err
	}
	result, ok := v.(*CopilotTokenResponse)
	if !ok {
		return "", 0, fmt.Errorf("singleflight returned unexpected type %T", v)
	}
	return result.Token, result.ExpiresAt, nil
}

func (e *CopilotTokenExchanger) exchangeWithClient(ctx context.Context, httpClient *httpclient.HttpClient, accessToken string) (*CopilotTokenResponse, error) {
	req := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(CopilotTokenEndpoint).
		WithHeader("Authorization", "token "+accessToken).
		WithHeader("Accept", "application/json").
		Build()
	log.Debug(ctx, "exchanging OAuth token for Copilot token",
		log.String("endpoint", CopilotTokenEndpoint),
	)
	// Create a bounded context with timeout for the token exchange operation
	exchangeCtx, cancel := context.WithTimeout(ctx, TokenExchangeTimeout)
	defer cancel()

	resp, err := httpClient.Do(exchangeCtx, req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	// Check status code before parsing response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange returned non-2xx status: %d", resp.StatusCode)
	}
	var tokenResp CopilotTokenResponse
	if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}
	if tokenResp.Token == "" {
		return nil, errors.New("copilot token is empty in response")
	}
	if tokenResp.ExpiresAt == 0 {
		return nil, errors.New("expires_at is missing in response")
	}

	expiresAt := time.Unix(tokenResp.ExpiresAt, 0)
	e.mu.Lock()
	e.cache[accessToken] = &CopilotTokenCacheEntry{
		Token:        accessToken,
		CopilotToken: tokenResp.Token,
		ExpiresAt:    expiresAt,
		CachedAt:     time.Now(),
	}
	e.mu.Unlock()
	log.Debug(ctx, "copilot token exchanged and cached",
		log.Time("expires_at", expiresAt),
		log.Duration("buffer", TokenExpiryBuffer),
	)
	return &tokenResp, nil
}

func (e *CopilotTokenExchanger) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	clear(e.cache)
}

func (e *CopilotTokenExchanger) RemoveFromCache(accessToken string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.cache, accessToken)
}
