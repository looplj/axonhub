package plugin

import (
	"context"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/pluginstorage"
	"github.com/looplj/axonhub/internal/ent/predicate"
)

const (
	kvDefaultNamespace = "default"
	kvMaxValueBytes    = 256 * 1024
	kvListLimit        = 100
	kvMaxTTL           = 90 * 24 * time.Hour
)

// kvNamePattern is the only accepted form for a KV namespace or key.
var kvNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

// KVEntry is one stored value. ExpiresAt is nil when the row does not expire.
type KVEntry struct {
	Namespace string
	Key       string
	Value     string
	ExpiresAt *time.Time
	UpdatedAt time.Time
}

// KVGet returns the row for this installation. A missing or expired row is
// (nil, nil); an expired row is deleted.
func (m *Manager) KVGet(ctx context.Context, manifestID string, installationID int, namespace, key string) (*KVEntry, error) {
	namespace, err := validateKVLookup(installationID, namespace, key)
	if err != nil {
		return nil, err
	}
	ctx = kvBypass(ctx, manifestID, installationID)
	row, err := m.client(ctx).PluginStorage.Query().
		Where(
			pluginstorage.InstallationID(installationID),
			pluginstorage.Namespace(namespace),
			pluginstorage.Key(key),
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read plugin kv: %w", err)
	}
	now := m.now()
	if kvExpired(row.ExpiresAt, now) {
		if _, err := m.client(ctx).PluginStorage.Delete().Where(
			pluginstorage.IDEQ(row.ID), pluginstorage.ExpiresAtLTE(now),
		).Exec(ctx); err != nil {
			return nil, fmt.Errorf("delete expired plugin kv: %w", err)
		}
		return nil, nil
	}
	entry := kvEntryFromRow(row)
	return &entry, nil
}

// KVSet upserts (installation_id, namespace, key), replacing value and expiry.
// ttl 0 stores no expiry. The installation id is the only identity.
func (m *Manager) KVSet(ctx context.Context, manifestID string, installationID int, namespace, key, value string, ttl time.Duration) (*KVEntry, error) {
	namespace, err := validateKVLookup(installationID, namespace, key)
	if err != nil {
		return nil, err
	}
	if err := validateKVValue(value); err != nil {
		return nil, err
	}
	expiresAt, err := m.kvExpiry(ttl)
	if err != nil {
		return nil, err
	}
	ctx = kvBypass(ctx, manifestID, installationID)
	client := m.client(ctx)
	create := client.PluginStorage.Create().
		SetInstallationID(installationID).
		SetNamespace(namespace).
		SetKey(key).
		SetValue(value).
		SetNillableExpiresAt(expiresAt)
	if err := create.OnConflictColumns(
		pluginstorage.FieldInstallationID,
		pluginstorage.FieldNamespace,
		pluginstorage.FieldKey,
	).Update(func(u *ent.PluginStorageUpsert) {
		u.SetValue(value)
		u.UpdateUpdatedAt()
		if expiresAt == nil {
			u.ClearExpiresAt()
		} else {
			u.SetExpiresAt(*expiresAt)
		}
	}).Exec(ctx); err != nil {
		return nil, fmt.Errorf("save plugin kv: %w", err)
	}
	row, err := client.PluginStorage.Query().
		Where(
			pluginstorage.InstallationID(installationID),
			pluginstorage.Namespace(namespace),
			pluginstorage.Key(key),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("read plugin kv after save: %w", err)
	}
	entry := kvEntryFromRow(row)
	return &entry, nil
}

// KVDelete removes one live key. A missing or expired key returns (false, nil).
func (m *Manager) KVDelete(ctx context.Context, manifestID string, installationID int, namespace, key string) (bool, error) {
	namespace, err := validateKVLookup(installationID, namespace, key)
	if err != nil {
		return false, err
	}
	ctx = kvBypass(ctx, manifestID, installationID)
	n, err := m.client(ctx).PluginStorage.Delete().
		Where(
			pluginstorage.InstallationID(installationID),
			pluginstorage.Namespace(namespace),
			pluginstorage.Key(key),
			pluginstorage.Or(pluginstorage.ExpiresAtIsNil(), pluginstorage.ExpiresAtGT(m.now())),
		).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("delete plugin kv: %w", err)
	}
	return n > 0, nil
}

// KVPage contains one page of keys and a cursor for the next page.
// NextCursor is nil when no more keys were found.
type KVPage struct {
	Entries    []KVEntry
	NextCursor *string
}

// KVList returns unexpired keys in ascending key order after the exclusive cursor.
// first must be between 1 and 100. An empty after starts at the first key.
func (m *Manager) KVList(ctx context.Context, manifestID string, installationID int, namespace, prefix string, first int, after string) (*KVPage, error) {
	if first < 1 || first > kvListLimit {
		return nil, fmt.Errorf("plugin kv page size must be between 1 and %d", kvListLimit)
	}
	if after != "" {
		if err := validateKVName("cursor", after); err != nil {
			return nil, err
		}
	}

	namespace, err := validateKVNamespace(installationID, namespace)
	if err != nil {
		return nil, err
	}
	ctx = kvBypass(ctx, manifestID, installationID)
	client := m.client(ctx)
	now := m.now()
	if _, err := client.PluginStorage.Delete().
		Where(
			pluginstorage.InstallationID(installationID),
			pluginstorage.Namespace(namespace),
			pluginstorage.ExpiresAtNotNil(),
			pluginstorage.ExpiresAtLTE(now),
		).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("delete expired plugin kv: %w", err)
	}
	preds := []predicate.PluginStorage{
		pluginstorage.InstallationID(installationID),
		pluginstorage.Namespace(namespace),
		pluginstorage.Or(
			pluginstorage.ExpiresAtIsNil(),
			pluginstorage.ExpiresAtGT(now),
		),
	}
	if prefix != "" {
		preds = append(preds, pluginstorage.KeyHasPrefix(prefix))
	}
	if after != "" {
		preds = append(preds, pluginstorage.KeyGT(after))
	}
	rows, err := client.PluginStorage.Query().
		Where(preds...).
		Order(pluginstorage.ByKey()).
		Limit(first + 1).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list plugin kv: %w", err)
	}
	page := &KVPage{}
	if len(rows) > first {
		rows = rows[:first]
		page.NextCursor = lo.ToPtr(rows[len(rows)-1].Key)
	}
	page.Entries = lo.Map(rows, func(row *ent.PluginStorage, _ int) KVEntry {
		return kvEntryFromRow(row)
	})
	return page, nil
}

// KVDeleteInstallation deletes every KV row for installationID.
// It does not look up the plugin row.
func (m *Manager) KVDeleteInstallation(ctx context.Context, manifestID string, installationID int) error {
	ctx = kvBypass(ctx, manifestID, installationID)
	if _, err := m.client(ctx).PluginStorage.Delete().
		Where(pluginstorage.InstallationID(installationID)).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete plugin kv for installation %d: %w", installationID, err)
	}
	return nil
}

func (m *Manager) now() time.Time {
	if m.kvNow != nil {
		return m.kvNow()
	}
	return time.Now()
}

func (m *Manager) kvExpiry(ttl time.Duration) (*time.Time, error) {
	if ttl < 0 {
		return nil, fmt.Errorf("plugin kv ttl must not be negative")
	}
	if ttl > kvMaxTTL {
		return nil, fmt.Errorf("plugin kv ttl must not exceed 90 days")
	}
	if ttl == 0 {
		return nil, nil
	}
	expiresAt := m.now().Add(ttl)
	return &expiresAt, nil
}

func kvBypass(ctx context.Context, manifestID string, installationID int) context.Context {
	return authz.WithSystemBypass(ctx, fmt.Sprintf("plugin-kv manifest=%s installation=%d", manifestID, installationID))
}

func validateKVLookup(installationID int, namespace, key string) (string, error) {
	namespace, err := validateKVNamespace(installationID, namespace)
	if err != nil {
		return "", err
	}
	if err := validateKVName("key", key); err != nil {
		return "", err
	}
	return namespace, nil
}

func validateKVNamespace(installationID int, namespace string) (string, error) {
	if namespace == "" {
		namespace = kvDefaultNamespace
	}
	if installationID <= 0 {
		return "", fmt.Errorf("plugin kv installation id must be positive")
	}
	if err := validateKVName("namespace", namespace); err != nil {
		return "", err
	}
	return namespace, nil
}

func validateKVName(kind, value string) error {
	if kvNamePattern.MatchString(value) {
		return nil
	}
	return fmt.Errorf("plugin kv %s %q must match %s", kind, value, kvNamePattern.String())
}

func validateKVValue(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("plugin kv value must be valid UTF-8")
	}
	if len(value) > kvMaxValueBytes {
		return fmt.Errorf("plugin kv value exceeds 256 KiB")
	}
	return nil
}

func kvExpired(expiresAt *time.Time, now time.Time) bool {
	return expiresAt != nil && !expiresAt.After(now)
}

func kvEntryFromRow(row *ent.PluginStorage) KVEntry {
	entry := KVEntry{
		Namespace: row.Namespace,
		Key:       row.Key,
		Value:     row.Value,
		UpdatedAt: row.UpdatedAt,
	}
	if row.ExpiresAt != nil {
		expiresAt := *row.ExpiresAt
		entry.ExpiresAt = &expiresAt
	}
	return entry
}
