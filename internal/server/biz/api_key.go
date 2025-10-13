package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/cespare/xxhash/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

type APIKeyServiceParams struct {
	fx.In

	CacheConfig xcache.Config
	UserService *UserService
}

type APIKeyService struct {
	UserService *UserService
	APIKeyCache xcache.Cache[ent.APIKey]
}

func NewAPIKeyService(params APIKeyServiceParams) *APIKeyService {
	return &APIKeyService{
		UserService: params.UserService,
		APIKeyCache: xcache.NewFromConfig[ent.APIKey](params.CacheConfig),
	}
}

// GenerateAPIKey generates a new API key with ah- prefix (similar to OpenAI format).
func GenerateAPIKey() (string, error) {
	// Generate 32 bytes of random data
	bytes := make([]byte, 32)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex and add ah- prefix
	return "ah-" + hex.EncodeToString(bytes), nil
}

// CreateAPIKey creates a new API key for a user.
func (s *APIKeyService) CreateAPIKey(ctx context.Context, input ent.CreateAPIKeyInput) (*ent.APIKey, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}

	client := ent.FromContext(ctx)

	// Generate API key with ah- prefix (similar to OpenAI format)
	generatedKey, err := GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	create := client.APIKey.Create().
		SetName(input.Name).
		SetKey(generatedKey).
		SetUserID(user.ID).
		SetProjectID(input.ProjectID)

	apiKey, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
}

// UpdateAPIKey updates an existing API key.
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, id int, input ent.UpdateAPIKeyInput) (*ent.APIKey, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	apiKey, err := client.APIKey.UpdateOneID(id).
		SetNillableName(input.Name).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	// Invalidate cache
	s.invalidateAPIKeyCache(ctx, apiKey.Key)

	return apiKey, nil
}

// UpdateAPIKeyStatus updates the status of an API key.
func (s *APIKeyService) UpdateAPIKeyStatus(ctx context.Context, id int, status apikey.Status) (*ent.APIKey, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	apiKey, err := client.APIKey.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key status: %w", err)
	}

	// Invalidate cache
	s.invalidateAPIKeyCache(ctx, apiKey.Key)

	return apiKey, nil
}

// UpdateAPIKeyProfiles updates the profiles of an API key.
func (s *APIKeyService) UpdateAPIKeyProfiles(ctx context.Context, id int, profiles objects.APIKeyProfiles) (*ent.APIKey, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	apiKey, err := client.APIKey.UpdateOneID(id).
		SetProfiles(&profiles).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key profiles: %w", err)
	}

	// Invalidate cache
	s.invalidateAPIKeyCache(ctx, apiKey.Key)

	return apiKey, nil
}

func buildAPIKeyCacheKey(key string) string {
	hash := xxhash.Sum64String(key)
	return "api_key:" + fmt.Sprintf("%d", hash)
}

// GetAPIKey authenticates an API key and returns the API key entity.
func (s *APIKeyService) GetAPIKey(ctx context.Context, key string) (*ent.APIKey, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Try cache first
	cacheKey := buildAPIKeyCacheKey(key)

	apiKey, err := s.APIKeyCache.Get(ctx, cacheKey)
	if err != nil || apiKey.Key != key {
		client := ent.FromContext(ctx)

		dbApiKey, err := client.APIKey.Query().
			Where(apikey.KeyEQ(key)).
			First(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get api key: %w", err)
		}

		// Cache API key using configured expiration
		err = s.APIKeyCache.Set(ctx, cacheKey, *dbApiKey)
		if err != nil {
			log.Error(ctx, "failed to cache api key", zap.Error(err))
		}

		apiKey = *dbApiKey
	}

	// DO NOT CACHE USER
	user, err := s.UserService.GetUserByID(ctx, apiKey.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get api key owner: %w", err)
	}

	apiKey.Edges.User = user

	return &apiKey, nil
}

// invalidateAPIKeyCache removes an API key from cache.
func (s *APIKeyService) invalidateAPIKeyCache(ctx context.Context, key string) {
	cacheKey := buildAPIKeyCacheKey(key)
	_ = s.APIKeyCache.Delete(ctx, cacheKey)
}
