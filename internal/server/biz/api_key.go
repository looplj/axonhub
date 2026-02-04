package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/watcher"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xcache/live"
	"github.com/looplj/axonhub/internal/scopes"
)

type APIKeyServiceParams struct {
	fx.In

	CacheConfig    xcache.Config
	Ent            *ent.Client
	ProjectService *ProjectService
}

type APIKeyService struct {
	*AbstractService

	ProjectService *ProjectService
	APIKeyCache    *live.IndexedCache[string, *ent.APIKey]
	apiKeyNotifier watcher.Notifier[live.CacheEvent[string]]
}

func NewAPIKeyService(params APIKeyServiceParams) *APIKeyService {
	svc := &APIKeyService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		ProjectService: params.ProjectService,
	}

	cacheMode := params.CacheConfig.Mode
	if cacheMode == "" {
		cacheMode = xcache.ModeMemory
	}

	watcherMode := cacheMode
	if watcherMode == xcache.ModeTwoLevel {
		watcherMode = watcher.ModeRedis
	}

	notifier, err := watcher.NewWatcherFromConfig[live.CacheEvent[string]](watcher.Config{
		Mode:  watcherMode,
		Redis: params.CacheConfig.Redis,
	}, watcher.WatcherFromConfigOptions{
		RedisChannel: "axonhub:cache:api_keys",
		Buffer:       32,
	})
	if err != nil {
		panic(fmt.Errorf("api key watcher init failed: %w", err))
	}

	ttl := params.CacheConfig.Memory.Expiration
	if ttl == 0 {
		ttl = 5 * time.Minute
	}

	svc.apiKeyNotifier = notifier
	svc.APIKeyCache = live.NewIndexedCache(live.IndexedOptions[string, *ent.APIKey]{
		Name:            "axonhub:api_keys",
		TTL:             ttl,
		RefreshInterval: 30 * time.Second,
		DebounceDelay:   500 * time.Millisecond,
		KeyFunc:         func(v *ent.APIKey) string { return buildAPIKeyCacheKey(v.Key) },
		DeletedFunc:     func(v *ent.APIKey) bool { return v.DeletedAt != 0 },
		Watcher:         notifier,
		LoadOneFunc:     svc.loadAPIKeyByKey,
		LoadSinceFunc:   svc.loadAPIKeysSince,
	})

	if err := svc.APIKeyCache.Load(context.Background()); err != nil {
		panic(fmt.Errorf("api key cache initial load failed: %w", err))
	}

	return svc
}

func (s *APIKeyService) Stop() {
	s.APIKeyCache.Stop()
}

func (s *APIKeyService) loadAPIKeyByKey(ctx context.Context, cacheKey string) (*ent.APIKey, error) {
	originalKey, ok := ctx.Value(apiKeyCtxKey{}).(string)
	if !ok || originalKey == "" {
		return nil, live.ErrKeyNotFound
	}

	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := s.entFromContext(ctx)

	item, err := client.APIKey.Query().Where(apikey.KeyEQ(originalKey), apikey.DeletedAtEQ(0)).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, live.ErrKeyNotFound
		}

		return nil, err
	}

	if buildAPIKeyCacheKey(item.Key) != cacheKey {
		return nil, live.ErrKeyNotFound
	}

	return item, nil
}

func (s *APIKeyService) loadAPIKeysSince(ctx context.Context, since time.Time) ([]*ent.APIKey, time.Time, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	ctx = schematype.SkipSoftDelete(ctx)
	client := s.entFromContext(ctx)

	q := client.APIKey.Query()
	if !since.IsZero() {
		q = q.Where(apikey.UpdatedAtGT(since))
	}

	items, err := q.All(ctx)
	if err != nil {
		return nil, since, err
	}

	maxUpdated := since
	if len(items) > 0 {
		maxUpdated = lo.MaxBy(items, func(a, b *ent.APIKey) bool {
			return a.UpdatedAt.After(b.UpdatedAt)
		}).UpdatedAt
	}

	return items, maxUpdated, nil
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

// CreateLLMAPIKey creates a new API key for LLM calls using a service account API key.
func (s *APIKeyService) CreateLLMAPIKey(ctx context.Context, owner *ent.APIKey, name string) (*ent.APIKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrAPIKeyNameRequired
	}

	client := s.entFromContext(ctx)

	generatedKey, err := GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate api key: %w", err)
	}

	create := client.APIKey.Create().
		SetName(name).
		SetKey(generatedKey).
		SetUserID(owner.UserID).
		SetProjectID(owner.ProjectID).
		SetType(apikey.TypeUser).
		SetScopes([]string{
			string(scopes.ScopeReadChannels),
			string(scopes.ScopeWriteRequests),
		})

	apiKey, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create api key: %w", err)
	}

	return apiKey, nil
}

// CreateAPIKey creates a new API key for a user.
func (s *APIKeyService) CreateAPIKey(ctx context.Context, input ent.CreateAPIKeyInput) (*ent.APIKey, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}

	client := s.entFromContext(ctx)

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

	apiKeyType := apikey.TypeUser // default

	// Set type (default is 'user' from schema)
	if input.Type != nil {
		create.SetType(*input.Type)
		apiKeyType = *input.Type
	}

	// For user type, use default scopes from schema (read_channels, write_requests)
	// No need to set explicitly as schema default will be used
	if apiKeyType == apikey.TypeServiceAccount {
		// For service account, use provided scopes or empty array
		if input.Scopes != nil {
			create.SetScopes(input.Scopes)
		} else {
			create.SetScopes([]string{})
		}
	}

	apiKey, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
}

// UpdateAPIKey updates an existing API key.
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, id int, input ent.UpdateAPIKeyInput) (*ent.APIKey, error) {
	client := s.entFromContext(ctx)

	apiKey, err := client.APIKey.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	if apiKey.Type == apikey.TypeUser {
		if len(input.Scopes) > 0 || len(input.AppendScopes) > 0 || input.ClearScopes {
			return nil, fmt.Errorf("user type API key cannot update scopes")
		}
	}

	update := client.APIKey.UpdateOneID(id).SetNillableName(input.Name)

	if apiKey.Type == apikey.TypeServiceAccount {
		if len(input.Scopes) > 0 {
			update.SetScopes(input.Scopes)
		}

		if len(input.AppendScopes) > 0 {
			update.AppendScopes(input.AppendScopes)
		}

		if input.ClearScopes {
			update.ClearScopes()
		}
	}

	apiKey, err = update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	s.invalidateAPIKeyCaches(ctx, apiKey.Key)

	return apiKey, nil
}

// UpdateAPIKeyStatus updates the status of an API key.
func (s *APIKeyService) UpdateAPIKeyStatus(ctx context.Context, id int, status apikey.Status) (*ent.APIKey, error) {
	client := s.entFromContext(ctx)

	apiKey, err := client.APIKey.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key status: %w", err)
	}

	// Invalidate cache
	s.invalidateAPIKeyCaches(ctx, apiKey.Key)

	return apiKey, nil
}

// UpdateAPIKeyProfiles updates the profiles of an API key.
func (s *APIKeyService) UpdateAPIKeyProfiles(ctx context.Context, id int, profiles objects.APIKeyProfiles) (*ent.APIKey, error) {
	client := s.entFromContext(ctx)

	// Validate that profile names are unique (case-insensitive)
	if err := validateProfileNames(profiles.Profiles); err != nil {
		return nil, err
	}

	// Validate that active profile exists in the profiles list
	if err := validateActiveProfile(profiles.ActiveProfile, profiles.Profiles); err != nil {
		return nil, err
	}

	// Validate quota configuration (if present)
	if err := validateProfileQuota(profiles.Profiles); err != nil {
		return nil, err
	}

	apiKey, err := client.APIKey.UpdateOneID(id).
		SetProfiles(&profiles).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key profiles: %w", err)
	}

	// Invalidate cache
	s.invalidateAPIKeyCaches(ctx, apiKey.Key)

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

func validateProfileQuota(profiles []objects.APIKeyProfile) error {
	for _, profile := range profiles {
		if profile.Quota == nil {
			continue
		}

		q := profile.Quota
		if q.Requests == nil && q.TotalTokens == nil && q.Cost == nil {
			return fmt.Errorf("profile '%s' quota must set at least one limit", profile.Name)
		}

		if q.Requests != nil && *q.Requests <= 0 {
			return fmt.Errorf("profile '%s' quota.requests must be positive", profile.Name)
		}

		if q.TotalTokens != nil && *q.TotalTokens <= 0 {
			return fmt.Errorf("profile '%s' quota.totalTokens must be positive", profile.Name)
		}

		if q.Cost != nil && q.Cost.IsNegative() {
			return fmt.Errorf("profile '%s' quota.cost must be non-negative", profile.Name)
		}

		switch q.Period.Type {
		case objects.APIKeyQuotaPeriodTypeAllTime:
		case objects.APIKeyQuotaPeriodTypePastDuration:
			if q.Period.PastDuration == nil {
				return fmt.Errorf("profile '%s' quota.period.pastDuration is required", profile.Name)
			}

			if q.Period.PastDuration.Value <= 0 {
				return fmt.Errorf("profile '%s' quota.period.pastDuration.value must be positive", profile.Name)
			}

			switch q.Period.PastDuration.Unit {
			case objects.APIKeyQuotaPastDurationUnitHour, objects.APIKeyQuotaPastDurationUnitDay:
			default:
				return fmt.Errorf("profile '%s' quota.period.pastDuration.unit is invalid", profile.Name)
			}
		case objects.APIKeyQuotaPeriodTypeCalendarDuration:
			if q.Period.CalendarDuration == nil {
				return fmt.Errorf("profile '%s' quota.period.calendarDuration is required", profile.Name)
			}

			switch q.Period.CalendarDuration.Unit {
			case objects.APIKeyQuotaCalendarDurationUnitDay, objects.APIKeyQuotaCalendarDurationUnitMonth:
			default:
				return fmt.Errorf("profile '%s' quota.period.calendarDuration.unit is invalid", profile.Name)
			}
		default:
			return fmt.Errorf("profile '%s' quota.period.type is invalid", profile.Name)
		}
	}

	return nil
}

type apiKeyCtxKey struct{}

func buildAPIKeyCacheKey(key string) string {
	hash := xxhash.Sum64String(key)
	return fmt.Sprintf("api_key:%d", hash)
}

func buildAPIKeyCacheKeys(keys []string) []string {
	cacheKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		cacheKeys = append(cacheKeys, buildAPIKeyCacheKey(key))
	}

	return cacheKeys
}

// GetAPIKey authenticates an API key and returns the API key entity.
func (s *APIKeyService) GetAPIKey(ctx context.Context, key string) (*ent.APIKey, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	ctx = context.WithValue(ctx, apiKeyCtxKey{}, key)
	cacheKey := buildAPIKeyCacheKey(key)

	cached, err := s.APIKeyCache.Get(ctx, cacheKey)

	if err != nil {
		if errors.Is(err, live.ErrKeyNotFound) {
			return nil, fmt.Errorf("%w:failed to get api key: %w", ErrInvalidAPIKey, err)
		}

		return nil, fmt.Errorf("failed to get api key: %w", err)
	}

	apiKey := *cached

	// DO NOT CACHE PROJECT
	project, err := s.ProjectService.GetProjectByID(ctx, apiKey.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get api key project: %w", err)
	}

	apiKey.Edges.Project = project

	return &apiKey, nil
}

func (s *APIKeyService) invalidateAPIKeyCaches(ctx context.Context, keys ...string) {
	if len(keys) == 0 {
		return
	}

	cacheKeys := buildAPIKeyCacheKeys(keys)
	if err := s.apiKeyNotifier.Notify(ctx, live.NewInvalidateKeysEvent(cacheKeys...)); err != nil {
		log.Warn(ctx, "api key cache watcher notify failed", log.Cause(err))
	}
}

func (s *APIKeyService) bulkUpdateAPIKeyStatus(ctx context.Context, ids []int, status apikey.Status, action string) error {
	if len(ids) == 0 {
		return nil
	}

	client := s.entFromContext(ctx)

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

	apiKeys, err := client.APIKey.Query().
		Where(apikey.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query API keys for cache invalidation: %w", err)
	}

	// Update all API keys status
	_, err = client.APIKey.Update().
		Where(apikey.IDIn(ids...)).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to %s API keys: %w", action, err)
	}

	s.invalidateAPIKeyCaches(ctx, lo.Map(apiKeys, func(apiKey *ent.APIKey, _ int) string { return apiKey.Key })...)
	return nil
}

// BulkDisableAPIKeys disables multiple API keys by their IDs.
func (s *APIKeyService) BulkDisableAPIKeys(ctx context.Context, ids []int) error {
	return s.bulkUpdateAPIKeyStatus(ctx, ids, apikey.StatusDisabled, "disable")
}

// BulkEnableAPIKeys enables multiple API keys by their IDs.
func (s *APIKeyService) BulkEnableAPIKeys(ctx context.Context, ids []int) error {
	return s.bulkUpdateAPIKeyStatus(ctx, ids, apikey.StatusEnabled, "enable")
}

// BulkArchiveAPIKeys archives multiple API keys by their IDs.
func (s *APIKeyService) BulkArchiveAPIKeys(ctx context.Context, ids []int) error {
	return s.bulkUpdateAPIKeyStatus(ctx, ids, apikey.StatusArchived, "archive")
}
