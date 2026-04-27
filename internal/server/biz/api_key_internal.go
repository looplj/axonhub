package biz

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
)

func (s *APIKeyService) onLoadOneKey(ctx context.Context, cacheKey string) (*ent.APIKey, error) {
	bypassCtx, err := authz.WithSystemBypass(ctx, "apikey-cache-load-one")
	if err != nil {
		return nil, err
	}
	return s.loadAPIKeyByKey(bypassCtx, cacheKey)
}

func (s *APIKeyService) onLoadAPIKeysSince(ctx context.Context, since time.Time) ([]*ent.APIKey, time.Time, error) {
	bypassCtx, err := authz.WithSystemBypass(ctx, "apikey-cache-load-since")
	if err != nil {
		return nil, time.Time{}, err
	}
	return s.loadAPIKeysSince(bypassCtx, since)
}
