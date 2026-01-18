package codex

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TokenProvider manages Codex OAuth2 credentials for a transformer instance.
// Each transformer has its own provider, so we can keep the credentials in memory.
type TokenProvider struct {
	httpClient *httpclient.HttpClient
	sf         singleflight.Group
	mu         sync.RWMutex
	creds      *OAuth2Credentials
}

func NewTokenProvider(creds *OAuth2Credentials, httpClient *httpclient.HttpClient) *TokenProvider {
	return &TokenProvider{
		httpClient: httpClient,
		creds:      creds,
	}
}

// Get returns a valid access token and optional account id.
// It refreshes it if expired.
func (p *TokenProvider) Get(ctx context.Context) (string, string, error) {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if creds == nil {
		return "", "", fmt.Errorf("codex credentials is nil")
	}

	now := time.Now()
	if !creds.IsExpired(now) {
		return creds.AccessToken, creds.AccountID, nil
	}

	// Refresh with singleflight to avoid stampede inside the same transformer.
	v, err, _ := p.sf.Do("codex:refresh", func() (any, error) {
		p.mu.RLock()
		current := p.creds
		p.mu.RUnlock()

		if current == nil {
			return nil, fmt.Errorf("codex credentials is nil")
		}

		if !current.IsExpired(time.Now()) {
			return current, nil
		}

		fresh, err := current.Refresh(ctx, p.httpClient)
		if err != nil {
			return nil, err
		}

		p.mu.Lock()
		p.creds = fresh
		p.mu.Unlock()

		log.Debug(ctx, "updated in-memory codex credentials", log.String("expires_at", fresh.ExpiresAt.Format(time.RFC3339)))

		return fresh, nil
	})
	if err != nil {
		return "", "", err
	}

	fresh, ok := v.(*OAuth2Credentials)
	if !ok {
		return "", "", fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return fresh.AccessToken, fresh.AccountID, nil
}
