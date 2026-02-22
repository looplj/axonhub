package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	CopilotTokenEndpoint = "https://api.github.com/copilot_internal/v2/token"
	TokenExpiryBuffer    = 5 * time.Minute
)

type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type CopilotTokenCacheEntry struct {
	AccessToken  string
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
	if httpClient == nil {
		httpClient = httpclient.NewHttpClient()
	}
	return &CopilotTokenExchanger{
		httpClient: httpClient,
		cache:      make(map[string]*CopilotTokenCacheEntry),
	}
}

func (e *CopilotTokenExchanger) GetToken(ctx context.Context, accessToken string) (string, int64, error) {
	if accessToken == "" {
		return "", 0, errors.New("access token is empty")
	}
	e.mu.RLock()
	entry, exists := e.cache[accessToken]
	e.mu.RUnlock()
	if exists && !entry.IsExpired(time.Now()) {
		slog.DebugContext(ctx, "copilot token cache hit",
			slog.Time("expires_at", entry.ExpiresAt),
			slog.Time("cached_at", entry.CachedAt),
		)
		return entry.CopilotToken, entry.ExpiresAt.Unix(), nil
	}
	slog.DebugContext(ctx, "copilot token cache miss or expired, performing exchange")
	return e.RefreshToken(ctx, accessToken)
}

func (e *CopilotTokenExchanger) RefreshToken(ctx context.Context, accessToken string) (string, int64, error) {
	if accessToken == "" {
		return "", 0, errors.New("access token is empty")
	}
	v, err, _ := e.sf.Do(accessToken, func() (any, error) {
		return e.exchange(ctx, accessToken)
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

func (e *CopilotTokenExchanger) exchange(ctx context.Context, accessToken string) (*CopilotTokenResponse, error) {
	req := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(CopilotTokenEndpoint).
		WithHeader("Authorization", "token "+accessToken).
		WithHeader("Accept", "application/json").
		Build()
	slog.DebugContext(ctx, "exchanging OAuth token for Copilot token",
		slog.String("endpoint", CopilotTokenEndpoint),
	)
	resp, err := e.httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
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
		AccessToken:  accessToken,
		CopilotToken: tokenResp.Token,
		ExpiresAt:    expiresAt,
		CachedAt:     time.Now(),
	}
	e.mu.Unlock()
	slog.DebugContext(ctx, "copilot token exchanged and cached",
		slog.Time("expires_at", expiresAt),
		slog.Duration("buffer", TokenExpiryBuffer),
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
