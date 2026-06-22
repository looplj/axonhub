package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/cespare/xxhash/v2"
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/watcher"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xcache/live"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/scopes"
)

const (
	//nolint:gosec // Checked.
	NoAuthAPIKeyValue = "AXONHUB_API_KEY_NO_AUTH"

	//nolint:gosec // Checked.
	NoAuthAPIKeyName = "No Auth System Key"
)

type APIKeyServiceParams struct {
	fx.In

	CacheConfig    xcache.Config
	Ent            *ent.Client
	ProjectService *ProjectService
	KeyPrefix      string `name:"api_key_prefix"`
}

type APIKeyService struct {
	*AbstractService

	ProjectService *ProjectService
	APIKeyCache    *live.IndexedCache[string, *ent.APIKey]
	apiKeyNotifier watcher.Notifier[live.CacheEvent[string]]
	keyPrefix      string
}

func NewAPIKeyService(params APIKeyServiceParams) *APIKeyService {
	svc := &APIKeyService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		ProjectService: params.ProjectService,
		keyPrefix:      params.KeyPrefix,
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
		LoadOneFunc:     svc.onLoadOneKey,
		LoadSinceFunc:   svc.onLoadAPIKeysSince,
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

// GenerateAPIKey generates a new API key with the given prefix.
func GenerateAPIKey(prefix string) (string, error) {
	if strings.TrimSpace(prefix) == "" {
		return "", fmt.Errorf("api key prefix must not be empty")
	}

	// Generate 32 bytes of random data
	bytes := make([]byte, 32)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex and add prefix
	return prefix + "-" + hex.EncodeToString(bytes), nil
}

// lockProjectForAPIKeyName serializes API key name create/rename within a single
// project so the live-name check and the write are atomic across concurrent
// writers (there is no DB unique constraint backing the name). It MUST be called
// inside a transaction.
//
// It takes a row-level lock on the parent project row (SELECT ... FOR UPDATE):
// concurrent name operations in the same project then block until the lock
// holder's transaction commits/rolls back, so the loser's check observes the
// committed row and is rejected. Because the lock is on a per-project row, name
// operations in different projects do not contend. This is portable across the
// multi-writer server dialects (PostgreSQL, MySQL, TiDB). SQLite serializes
// writers itself and rejects SELECT ... FOR UPDATE, so the lock is a no-op there.
//
// The project row is read with a system bypass because some write callers (e.g.
// the OpenAPI service-account principal) may lack project read scope, and it runs
// on the transaction's connection so the lock is held for the rest of the tx.
func (s *APIKeyService) lockProjectForAPIKeyName(ctx context.Context, projectID int) error {
	client := s.entFromContext(ctx)

	// SQLite is single-writer and does not support SELECT ... FOR UPDATE; the lock
	// is both unnecessary and unsupported there.
	if client.Driver().Dialect() == dialect.SQLite {
		return nil
	}

	bypassCtx := authz.WithSystemBypass(ctx, "api key name uniqueness lock")

	var ids []int

	err := client.Project.Query().
		Where(project.IDEQ(projectID)).
		Modify(func(s *sql.Selector) {
			s.Select(s.C(project.FieldID)).ForUpdate()
		}).
		Scan(bypassCtx, &ids)
	if err != nil {
		return fmt.Errorf("failed to lock project for api key name uniqueness: %w", err)
	}

	return nil
}

// CreateLLMAPIKey creates a new API key for LLM calls using a service account API key.
func (s *APIKeyService) CreateLLMAPIKey(ctx context.Context, owner *ent.APIKey, name string) (*ent.APIKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrAPIKeyNameRequired
	}

	generatedKey, err := GenerateAPIKey(s.keyPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to generate api key: %w", err)
	}

	var apiKey *ent.APIKey

	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		// Serialize same-project name operations so the check-then-write is atomic
		// across concurrent writers (PostgreSQL, MySQL, TiDB); no-op on SQLite.
		if err := s.lockProjectForAPIKeyName(ctx, owner.ProjectID); err != nil {
			return err
		}

		// Names identify keys on the OpenAPI surface (GetForRead resolves a name
		// within the owner's project), so per-project name uniqueness must hold. The
		// privacy mutation policy vets the caller during Save, so an unauthorized
		// caller is denied before the post-insert check below and cannot use
		// duplicate-name errors to probe which names exist.
		created, err := client.APIKey.Create().
			SetName(name).
			SetKey(generatedKey).
			SetUserID(owner.UserID).
			SetProjectID(owner.ProjectID).
			SetType(apikey.TypeUser).
			SetScopes([]string{
				string(scopes.ScopeReadChannels),
				string(scopes.ScopeWriteRequests),
			}).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create api key: %w", err)
		}

		// API key names are unique per project at the application level — there is
		// no DB unique constraint. After the authorized insert, verify no other live
		// key in this project shares the name; checking AFTER Save preserves the
		// privacy-denial ordering (the mutation policy already vetted the caller, so
		// an unauthorized caller is denied before reaching this check and cannot
		// probe which names exist). The count is privacy-bypassed because the OpenAPI
		// service-account principal may lack read scope, and is live-only (the
		// soft-delete interceptor filters deleted_at) so names stay reusable after a
		// soft delete. With the project row lock above held, a concurrent same-name
		// create cannot interleave: it blocks until this transaction commits and then
		// observes this row, so the check is race-safe on multi-writer backends too.
		bypassCtx := authz.WithSystemBypass(ctx, "api key name uniqueness")

		dupCount, err := client.APIKey.Query().
			Where(
				apikey.NameEQ(name),
				apikey.ProjectIDEQ(owner.ProjectID),
			).
			Count(bypassCtx)
		if err != nil {
			return fmt.Errorf("failed to check api key name uniqueness: %w", err)
		}

		if dupCount > 1 {
			return xerrors.DuplicateNameError("API Key", name)
		}

		apiKey = created

		return nil
	})
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

// CreateAPIKey creates a new API key for a user.
func (s *APIKeyService) CreateAPIKey(ctx context.Context, input ent.CreateAPIKeyInput) (*ent.APIKey, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}

	apiKeyType := apikey.TypeUser // default (schema applies it when unset)
	if input.Type != nil {
		if *input.Type == apikey.TypeNoauth {
			return nil, fmt.Errorf("noauth type API key is reserved")
		}

		apiKeyType = *input.Type
	}

	// Generate API key with configured prefix
	generatedKey, err := GenerateAPIKey(s.keyPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	var apiKey *ent.APIKey

	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		// API key names are unique per project at the application level (there is no
		// DB unique constraint). The project row lock serializes same-project name
		// operations so the live-only check (the soft-delete interceptor filters
		// deleted_at, so a name is reusable after a soft delete) and the insert are
		// atomic across concurrent writers (PostgreSQL, MySQL, TiDB); no-op on the
		// single-writer SQLite default.
		if err := s.lockProjectForAPIKeyName(ctx, input.ProjectID); err != nil {
			return err
		}

		exists, err := client.APIKey.Query().
			Where(
				apikey.NameEQ(input.Name),
				apikey.ProjectIDEQ(input.ProjectID),
			).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("failed to check API key name uniqueness: %w", err)
		}

		if exists {
			return xerrors.DuplicateNameError("API Key", input.Name)
		}

		create := client.APIKey.Create().
			SetName(input.Name).
			SetKey(generatedKey).
			SetUserID(user.ID).
			SetProjectID(input.ProjectID)

		if input.Type != nil {
			create.SetType(*input.Type)
		}

		// User type uses the schema default scopes; service account uses provided
		// scopes (or empty array).
		if apiKeyType == apikey.TypeServiceAccount {
			if input.Scopes != nil {
				create.SetScopes(input.Scopes)
			} else {
				create.SetScopes([]string{})
			}
		}

		created, err := create.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create API key: %w", err)
		}

		apiKey = created

		return nil
	})
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

// UpdateAPIKey updates an existing API key.
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, id int, input ent.UpdateAPIKeyInput) (*ent.APIKey, error) {
	var result *ent.APIKey

	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		apiKey, err := client.APIKey.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get API key: %w", err)
		}

		if apiKey.Type == apikey.TypeUser {
			if len(input.Scopes) > 0 || len(input.AppendScopes) > 0 || input.ClearScopes {
				return fmt.Errorf("user type API key cannot update scopes")
			}
		}

		if apiKey.Type == apikey.TypeNoauth {
			return fmt.Errorf("noauth type API key cannot be updated")
		}

		// Renaming: serialize same-project name operations and reject a duplicate
		// live name (no DB unique constraint backs the name). The project row lock
		// makes the check-then-update atomic across concurrent writers (PostgreSQL,
		// MySQL, TiDB); no-op on the single-writer SQLite default.
		if input.Name != nil && *input.Name != apiKey.Name {
			if err := s.lockProjectForAPIKeyName(ctx, apiKey.ProjectID); err != nil {
				return err
			}

			exists, err := client.APIKey.Query().
				Where(
					apikey.NameEQ(*input.Name),
					apikey.ProjectIDEQ(apiKey.ProjectID),
					apikey.IDNEQ(id),
				).
				Exist(ctx)
			if err != nil {
				return fmt.Errorf("failed to check API key name uniqueness: %w", err)
			}

			if exists {
				return xerrors.DuplicateNameError("API Key", *input.Name)
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

		updated, err := update.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update API key: %w", err)
		}

		result = updated

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.invalidateAPIKeyCaches(ctx, result.Key)

	return result, nil
}

// UpdateAPIKeyStatus updates the status of an API key.
func (s *APIKeyService) UpdateAPIKeyStatus(ctx context.Context, id int, status apikey.Status) (*ent.APIKey, error) {
	client := s.entFromContext(ctx)

	existing, err := client.APIKey.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	if existing.Type == apikey.TypeNoauth {
		return nil, fmt.Errorf("noauth type API key status cannot be updated")
	}

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

	existing, err := client.APIKey.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	if existing.Type == apikey.TypeNoauth {
		return nil, fmt.Errorf("noauth type API key profiles cannot be updated")
	}

	// Validate that profile names are unique (case-insensitive)
	if err := validateProfileNames(profiles.Profiles); err != nil {
		return nil, err
	}

	// Validate that active profile exists in the profiles list
	if err := validateActiveProfile(profiles.ActiveProfile, profiles.Profiles); err != nil {
		return nil, err
	}

	if err := validateProfileFilters(profiles.Profiles); err != nil {
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

func validateProfileFilters(profiles []objects.APIKeyProfile) error {
	for _, profile := range profiles {
		if !profile.ChannelTagsMatchMode.IsValid() {
			return fmt.Errorf("profile '%s' channelTagsMatchMode is invalid", profile.Name)
		}
	}

	return nil
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
			case objects.APIKeyQuotaPastDurationUnitMinute, objects.APIKeyQuotaPastDurationUnitHour, objects.APIKeyQuotaPastDurationUnitDay:
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

func (s *APIKeyService) GetAPIKey(ctx context.Context, key string) (*ent.APIKey, error) {
	// Add API key to context for cache.
	ctx = context.WithValue(ctx, apiKeyCtxKey{}, key)
	cacheKey := buildAPIKeyCacheKey(key)

	cached, err := s.APIKeyCache.Get(ctx, cacheKey)

	if err != nil {
		if errors.Is(err, live.ErrKeyNotFound) {
			return nil, fmt.Errorf("%w: failed to get api key: %w", ErrInvalidAPIKey, err)
		}

		return nil, fmt.Errorf("failed to get api key: %w", err)
	}

	apiKey := *cached

	// DO NOT CACHE PROJECT
	project, err := s.ProjectService.GetProjectByID(ctx, apiKey.ProjectID)
	if err != nil {
		// Check if it's a "not found" error
		if errors.Is(err, ErrProjectNotFound) {
			return nil, fmt.Errorf("%w: project not found", ErrInvalidAPIKey)
		}
		// Return original error for other cases (database errors, internal errors, etc.)
		return nil, fmt.Errorf("failed to get api key project: %w", err)
	}

	apiKey.Edges.Project = project

	return &apiKey, nil
}

// GetForRead loads an API key by id, key, or name for read-only access. Exactly
// one of id, key, or name must be non-nil.
//
// It deliberately goes through the context-bound ent client (entFromContext) so
// the APIKey privacy policy runs: an API key principal must hold read_api_keys
// and can only see keys inside its own project. Callers in another project — or
// missing the scope — therefore get a NotFound / privacy error, never a foreign
// key. This is the read-side counterpart to the implicit ent gating used by the
// update mutations.
//
// Name lookups rely on the same project boundary: names are unique within a
// project (enforced on create/update), so once the privacy filter narrows the
// query to the caller's project, a name identifies at most one key.
func (s *APIKeyService) GetForRead(ctx context.Context, id *int, key *string, name *string) (*ent.APIKey, error) {
	if lo.Count([]bool{id != nil, key != nil, name != nil}, true) != 1 {
		return nil, fmt.Errorf("exactly one of api key id, key, or name must be provided")
	}

	client := s.entFromContext(ctx)
	q := client.APIKey.Query()

	switch {
	case id != nil:
		q = q.Where(apikey.IDEQ(*id))
	case key != nil:
		q = q.Where(apikey.KeyEQ(*key))
	case name != nil:
		q = q.Where(apikey.NameEQ(*name))
	}

	apiKey, err := q.Only(ctx)
	if err != nil {
		// Names are unique per project only at the application level (no DB
		// constraint), so a database that predates that enforcement may hold
		// duplicate live names. A name then no longer identifies a single key —
		// surface an actionable error instead of ent's opaque "not singular".
		if name != nil && ent.IsNotSingular(err) {
			return nil, fmt.Errorf("multiple API keys are named %q in this project; use id or key to identify the key", *name)
		}

		return nil, err
	}

	return apiKey, nil
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

	noAuthExists, err := client.APIKey.Query().
		Where(apikey.IDIn(ids...), apikey.TypeEQ(apikey.TypeNoauth)).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to validate API keys for bulk %s: %w", action, err)
	}

	if noAuthExists {
		return fmt.Errorf("noauth type API key cannot be bulk %sd", action)
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

// RotateAPIKey rotates an API key by generating a new key value while preserving all other properties.
// This is useful when a key is compromised or when an employee leaves, without losing usage statistics.
func (s *APIKeyService) RotateAPIKey(ctx context.Context, id int) (*ent.APIKey, error) {
	// Get the existing API key
	existing, err := s.db.APIKey.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Cannot rotate noauth type API key
	if existing.Type == apikey.TypeNoauth {
		return nil, fmt.Errorf("noauth type API key cannot be rotated")
	}

	// Generate a new API key
	newKey, err := GenerateAPIKey(s.keyPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new API key: %w", err)
	}

	oldKey := existing.Key

	// Update the key field directly using Ent
	rotated, err := s.db.APIKey.UpdateOneID(id).
		SetKey(newKey).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate API key: %w", err)
	}

	// Invalidate caches for both old and new keys
	s.invalidateAPIKeyCaches(ctx, oldKey, newKey)

	return rotated, nil
}

func (s *APIKeyService) EnsureNoAuthAPIKey(ctx context.Context) (*ent.APIKey, error) {
	existing, err := s.GetAPIKey(ctx, NoAuthAPIKeyValue)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, ErrInvalidAPIKey) {
		return nil, fmt.Errorf("failed to query noauth api key from cache: %w", err)
	}

	client := s.entFromContext(ctx)
	proj, err := client.Project.Query().
		Order(ent.Asc(project.FieldID)).
		First(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default project: %w", err)
	}

	owner, err := client.User.Query().Where(user.IsOwnerEQ(true)).First(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get owner user for noauth api key: %w", err)
	}

	apiKey, err := client.APIKey.Create().
		SetName(NoAuthAPIKeyName).
		SetKey(NoAuthAPIKeyValue).
		SetUserID(owner.ID).
		SetProjectID(proj.ID).
		SetType(apikey.TypeNoauth).
		SetStatus(apikey.StatusEnabled).
		SetScopes([]string{string(scopes.ScopeWriteRequests), string(scopes.ScopeReadChannels)}).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create noauth api key: %w", err)
	}

	// DO NOT CACHE PROJECT
	project, err := s.ProjectService.GetProjectByID(ctx, apiKey.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get api key project: %w", err)
	}

	apiKey.Edges.Project = project

	s.invalidateAPIKeyCaches(ctx, apiKey.Key)

	return apiKey, nil
}
