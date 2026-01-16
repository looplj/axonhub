package codex

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TokenProvider manages Codex OAuth2 credentials for a channel.
// It refreshes tokens when needed and persists refreshed credentials back to DB.
type TokenProvider struct {
	cache      xcache.Cache[string]
	httpClient *httpclient.HttpClient
	sf         singleflight.Group
}

func NewTokenProvider(cacheConfig xcache.Config, httpClient *httpclient.HttpClient) *TokenProvider {
	return &TokenProvider{
		cache:      xcache.NewFromConfig[string](cacheConfig),
		httpClient: httpClient,
	}
}

func tokenCacheKey(projectID, channelID int) string {
	return fmt.Sprintf("codex:token:%d:%d", projectID, channelID)
}

// Get returns a valid access token and optional account id.
// It reads the channel credentials JSON and refreshes it if expired.
func (p *TokenProvider) Get(ctx context.Context, channel *ent.Channel) (string, string, error) {
	if channel == nil {
		return "", "", fmt.Errorf("channel is nil")
	}

	projectID, _ := contexts.GetProjectID(ctx)
	key := tokenCacheKey(projectID, channel.ID)

	if cached, err := p.cache.Get(ctx, key); err == nil {
		creds, err := ParseCredentialsJSON(cached)
		if err == nil && !creds.IsExpired(time.Now()) {
			return creds.AccessToken, creds.AccountID, nil
		}
	}

	creds, err := ParseCredentialsJSON(channel.Credentials.APIKey)
	if err != nil {
		return "", "", err
	}

	if !creds.IsExpired(time.Now()) {
		if err := p.cache.Set(ctx, key, channel.Credentials.APIKey, xcache.WithExpiration(55*time.Minute)); err != nil {
			log.Warn(ctx, "failed to cache codex credentials", log.String("key", key), log.Cause(err))
		}

		return creds.AccessToken, creds.AccountID, nil
	}

	// Refresh with singleflight to avoid stampede.
	sfKey := fmt.Sprintf("codex:refresh:%d", channel.ID)

	v, err, _ := p.sf.Do(sfKey, func() (any, error) {
		if cached, err := p.cache.Get(ctx, key); err == nil {
			cachedCreds, err := ParseCredentialsJSON(cached)
			if err == nil && !cachedCreds.IsExpired(time.Now()) {
				return cachedCreds, nil
			}
		}

		fresh, err := creds.Refresh(ctx, p.httpClient, p.cache, key)
		if err != nil {
			return nil, err
		}

		raw, err := fresh.ToJSON()
		if err != nil {
			return nil, err
		}

		if err := p.cache.Set(ctx, key, raw, xcache.WithExpiration(55*time.Minute)); err != nil {
			log.Warn(ctx, "failed to cache refreshed codex credentials", log.String("key", key), log.Cause(err))
		}

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
