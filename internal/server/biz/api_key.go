package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

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

	CacheConfig    xcache.Config
	ProjectService *ProjectService
}

type APIKeyService struct {
	ProjectService *ProjectService
	APIKeyCache    xcache.Cache[ent.APIKey]
}

func NewAPIKeyService(params APIKeyServiceParams) *APIKeyService {
	return &APIKeyService{
		ProjectService: params.ProjectService,
		APIKeyCache:    xcache.NewFromConfig[ent.APIKey](params.CacheConfig),
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

	// Validate that profile names are unique (case-insensitive)
	if err := validateProfileNames(profiles.Profiles); err != nil {
		return nil, err
	}

	// Validate that active profile exists in the profiles list
	if err := validateActiveProfile(profiles.ActiveProfile, profiles.Profiles); err != nil {
		return nil, err
	}

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

// validateProfileNames checks that all profile names are unique (case-insensitive).
func validateProfileNames(profiles []objects.APIKeyProfile) error {
	seen := make(map[string]bool)

	for _, profile := range profiles {
		nameLower := strings.ToLower(strings.TrimSpace(profile.Name))
		if nameLower == "" {
			return fmt.Errorf("profile name cannot be empty")
		}

		if seen[nameLower] {
			return fmt.Errorf("duplicate profile name: %s", profile.Name)
		}

		seen[nameLower] = true
	}

	return nil
}

// validateActiveProfile checks that the active profile exists in the profiles list.
func validateActiveProfile(activeProfile string, profiles []objects.APIKeyProfile) error {
	for _, profile := range profiles {
		if profile.Name == activeProfile {
			return nil
		}
	}

	return fmt.Errorf("active profile '%s' does not exist in the profiles list", activeProfile)
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

	// DO NOT CACHE PROJECT
	project, err := s.ProjectService.GetProjectByID(ctx, apiKey.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get api key project: %w", err)
	}

	apiKey.Edges.Project = project

	return &apiKey, nil
}

// invalidateAPIKeyCache removes an API key from cache.
func (s *APIKeyService) invalidateAPIKeyCache(ctx context.Context, key string) {
	cacheKey := buildAPIKeyCacheKey(key)
	_ = s.APIKeyCache.Delete(ctx, cacheKey)
}

// BulkDisableAPIKeys disables multiple API keys by their IDs.
func (s *APIKeyService) BulkDisableAPIKeys(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	client := ent.FromContext(ctx)
	// Verify all API keys exist
	count, err := client.APIKey.Query().
		Where(apikey.IDIn(ids...)).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to query API keys: %w", err)
	}

	if count != len(ids) {
		return fmt.Errorf("expected to find %d API keys, but found %d", len(ids), count)
	}

	// Invalidate cache for all affected API keys
	apiKeys, err := client.APIKey.Query().
		Where(apikey.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query API keys for cache invalidation: %w", err)
	}

	// Update all API keys status to disabled
	_, err = client.APIKey.Update().
		Where(apikey.IDIn(ids...)).
		SetStatus(apikey.StatusDisabled).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to disable API keys: %w", err)
	}

	for _, apiKey := range apiKeys {
		s.invalidateAPIKeyCache(ctx, apiKey.Key)
	}

	return nil
}

// BulkArchiveAPIKeys archives multiple API keys by their IDs.
func (s *APIKeyService) BulkArchiveAPIKeys(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	client := ent.FromContext(ctx)
	// Verify all API keys exist
	count, err := client.APIKey.Query().
		Where(apikey.IDIn(ids...)).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to query API keys: %w", err)
	}

	if count != len(ids) {
		return fmt.Errorf("expected to find %d API keys, but found %d", len(ids), count)
	}

	// Invalidate cache for all affected API keys
	apiKeys, err := client.APIKey.Query().
		Where(apikey.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query API keys for cache invalidation: %w", err)
	}

	// Update all API keys status to archived
	_, err = client.APIKey.Update().
		Where(apikey.IDIn(ids...)).
		SetStatus(apikey.StatusArchived).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to archive API keys: %w", err)
	}

	for _, apiKey := range apiKeys {
		s.invalidateAPIKeyCache(ctx, apiKey.Key)
	}

	return nil
}
