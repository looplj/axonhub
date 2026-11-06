package plugin

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
	"github.com/looplj/axonhub/internal/ent/pluginstorage"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
)

func TestKVSetGetDeleteList(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))
	manifestID := installed.PluginID
	installationID := installed.ID

	missing, err := manager.KVGet(ctx, manifestID, installationID, "", "cursor")
	require.NoError(t, err)
	require.Nil(t, missing)
	deleted, err := manager.KVDelete(ctx, manifestID, installationID, "default", "cursor")
	require.NoError(t, err)
	require.False(t, deleted)

	empty, err := manager.KVSet(ctx, manifestID, installationID, "", "cursor", "", 0)
	require.NoError(t, err)
	require.Equal(t, "default", empty.Namespace)
	require.Equal(t, "cursor", empty.Key)
	require.Empty(t, empty.Value)
	require.Nil(t, empty.ExpiresAt)

	got, err := manager.KVGet(ctx, manifestID, installationID, "default", "cursor")
	require.NoError(t, err)
	require.Equal(t, "", got.Value)
	got, err = manager.KVGet(ctx, manifestID, installationID, "", "cursor")
	require.NoError(t, err)
	require.Equal(t, "", got.Value)

	replaced, err := manager.KVSet(ctx, manifestID, installationID, "default", "cursor", "second", time.Hour)
	require.NoError(t, err)
	require.Equal(t, "second", replaced.Value)
	require.NotNil(t, replaced.ExpiresAt)
	cleared, err := manager.KVSet(ctx, manifestID, installationID, "default", "cursor", "third", 0)
	require.NoError(t, err)
	require.Equal(t, "third", cleared.Value)
	require.Nil(t, cleared.ExpiresAt)

	_, err = manager.KVSet(ctx, manifestID, installationID, "custom", "b", "bee", 0)
	require.NoError(t, err)
	_, err = manager.KVSet(ctx, manifestID, installationID, "custom", "a.b:c_d-e", "dotted", 0)
	require.NoError(t, err)
	_, err = manager.KVSet(ctx, manifestID, installationID, "custom", "c", "cee", 0)
	require.NoError(t, err)

	listed, err := listKVEntries(manager, ctx, manifestID, installationID, "", "")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, "cursor", listed[0].Key)
	require.Equal(t, "third", listed[0].Value)

	custom, err := listKVEntries(manager, ctx, manifestID, installationID, "custom", "")
	require.NoError(t, err)
	require.Equal(t, []string{"a.b:c_d-e", "b", "c"}, keysOf(custom))
	prefixed, err := listKVEntries(manager, ctx, manifestID, installationID, "custom", "a.b")
	require.NoError(t, err)
	require.Equal(t, []string{"a.b:c_d-e"}, keysOf(prefixed))
	none, err := listKVEntries(manager, ctx, manifestID, installationID, "custom", "missing")
	require.NoError(t, err)
	require.Empty(t, none)

	removed, err := manager.KVDelete(ctx, manifestID, installationID, "custom", "b")
	require.NoError(t, err)
	require.True(t, removed)
	gone, err := manager.KVGet(ctx, manifestID, installationID, "custom", "b")
	require.NoError(t, err)
	require.Nil(t, gone)
	again, err := manager.KVDelete(ctx, manifestID, installationID, "", "b")
	require.NoError(t, err)
	require.False(t, again)
}

func TestKVValidation(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))
	manifestID := installed.PluginID
	installationID := installed.ID

	_, err := manager.KVSet(ctx, manifestID, 0, "", "cursor", "v", 0)
	require.ErrorContains(t, err, "positive")
	_, err = manager.KVGet(ctx, manifestID, installationID, "bad name", "cursor")
	require.ErrorContains(t, err, "namespace")
	_, err = manager.KVSet(ctx, manifestID, installationID, strings.Repeat("a", 129), "cursor", "v", 0)
	require.ErrorContains(t, err, "namespace")
	_, err = manager.KVDelete(ctx, manifestID, installationID, "default", "")
	require.ErrorContains(t, err, "key")
	_, err = manager.KVSet(ctx, manifestID, installationID, "default", "bad key", "v", 0)
	require.ErrorContains(t, err, "key")
	_, err = listKVEntries(manager, ctx, manifestID, installationID, "bad name", "")
	require.ErrorContains(t, err, "namespace")

	_, err = manager.KVSet(ctx, manifestID, installationID, "default", "cursor", string([]byte{0xff}), 0)
	require.ErrorContains(t, err, "UTF-8")
	_, err = manager.KVSet(ctx, manifestID, installationID, "default", "cursor", strings.Repeat("a", kvMaxValueBytes+1), 0)
	require.ErrorContains(t, err, "256 KiB")
	_, err = manager.KVSet(ctx, manifestID, installationID, "default", "cursor", "v", -time.Nanosecond)
	require.ErrorContains(t, err, "negative")
	_, err = manager.KVSet(ctx, manifestID, installationID, "default", "cursor", "v", kvMaxTTL+time.Nanosecond)
	require.ErrorContains(t, err, "90")

	start := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	manager.kvNow = func() time.Time { return start }
	exact, err := manager.KVSet(ctx, manifestID, installationID, "default", "limit", strings.Repeat("a", kvMaxValueBytes), kvMaxTTL)
	require.NoError(t, err)
	require.Len(t, exact.Value, kvMaxValueBytes)
	require.NotNil(t, exact.ExpiresAt)
	require.True(t, exact.ExpiresAt.Equal(start.Add(kvMaxTTL)))
	require.Equal(t, 1, kvCount(t, client, installationID))
}

func TestKVListPagination(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))
	for i := range 101 {
		_, err := manager.KVSet(ctx, installed.PluginID, installed.ID, "", kvKey(i), "v", 0)
		require.NoError(t, err)
	}
	page, err := manager.KVList(ctx, installed.PluginID, installed.ID, "", "k", 100, "")
	require.NoError(t, err)
	require.Len(t, page.Entries, 100)
	require.Equal(t, kvKey(0), page.Entries[0].Key)
	require.Equal(t, kvKey(99), *page.NextCursor)
	// Removing the cursor row does not prevent fetching the next page.
	_, err = manager.KVDelete(ctx, installed.PluginID, installed.ID, "", *page.NextCursor)
	require.NoError(t, err)
	last, err := manager.KVList(ctx, installed.PluginID, installed.ID, "", "k", 100, *page.NextCursor)
	require.NoError(t, err)
	require.Equal(t, []string{kvKey(100)}, keysOf(last.Entries))
	require.Nil(t, last.NextCursor)
	empty, err := manager.KVList(ctx, installed.PluginID, installed.ID, "", "missing", 1, "")
	require.NoError(t, err)
	require.Empty(t, empty.Entries)
	require.Nil(t, empty.NextCursor)
	for _, size := range []int{-1, 0, 101} {
		_, err := manager.KVList(ctx, installed.PluginID, installed.ID, "", "", size, "")
		require.Error(t, err)
	}
	_, err = manager.KVList(ctx, installed.PluginID, installed.ID, "", "", 1, "bad cursor")
	require.Error(t, err)
}

// listKVEntries keeps non-pagination tests focused on their entry assertions.
func listKVEntries(manager *Manager, ctx context.Context, manifestID string, installationID int, namespace, prefix string) ([]KVEntry, error) {
	page, err := manager.KVList(ctx, manifestID, installationID, namespace, prefix, 100, "")
	if err != nil {
		return nil, err
	}
	return page.Entries, nil
}

func TestKVExpiredReadPreservesConcurrentSave(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))
	now := time.Now()
	manager.kvNow = func() time.Time { return now }
	_, err := manager.KVSet(ctx, installed.PluginID, installed.ID, "", "key", "expired", time.Second)
	require.NoError(t, err)
	now = now.Add(2 * time.Second)
	refreshed := false
	client.PluginStorage.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			if mutation.Op().Is(ent.OpDelete|ent.OpDeleteOne) && !refreshed {
				refreshed = true
				// Another writer renews the row after KVGet read its expired snapshot.
				_, err := manager.KVSet(ctx, installed.PluginID, installed.ID, "", "key", "fresh", 0)
				if err != nil {
					return nil, err
				}
			}
			return next.Mutate(ctx, mutation)
		})
	})
	_, err = manager.KVGet(ctx, installed.PluginID, installed.ID, "", "key")
	require.NoError(t, err)
	require.True(t, refreshed)
	entry, err := manager.KVGet(ctx, installed.PluginID, installed.ID, "", "key")
	require.NoError(t, err)
	require.NotNil(t, entry)
	require.Equal(t, "fresh", entry.Value)
}

func TestKVDeleteExpiry(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	manager.kvNow = func() time.Time { return now }
	for _, key := range []string{"expired", "live"} {
		_, err := manager.KVSet(ctx, installed.PluginID, installed.ID, "", key, "value", time.Minute)
		require.NoError(t, err)
	}
	deleted, err := manager.KVDelete(ctx, installed.PluginID, installed.ID, "", "live")
	require.NoError(t, err)
	require.True(t, deleted)
	now = now.Add(time.Minute)
	deleted, err = manager.KVDelete(ctx, installed.PluginID, installed.ID, "", "expired")
	require.NoError(t, err)
	require.False(t, deleted)
	deleted, err = manager.KVDelete(ctx, installed.PluginID, installed.ID, "", "missing")
	require.NoError(t, err)
	require.False(t, deleted)
	_, err = manager.KVSet(ctx, installed.PluginID, installed.ID, "", "expired", "renewed", 0)
	require.NoError(t, err)
	deleted, err = manager.KVDelete(ctx, installed.PluginID, installed.ID, "", "expired")
	require.NoError(t, err)
	require.True(t, deleted)
}

func TestKVExpiry(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))
	manifestID := installed.PluginID
	installationID := installed.ID

	start := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	current := start
	manager.kvNow = func() time.Time { return current }

	_, err := manager.KVSet(ctx, manifestID, installationID, "default", "early", "gone", time.Minute)
	require.NoError(t, err)
	_, err = manager.KVSet(ctx, manifestID, installationID, "default", "temp", "gone", time.Minute)
	require.NoError(t, err)
	_, err = manager.KVSet(ctx, manifestID, installationID, "default", "keep", "stay", 0)
	require.NoError(t, err)
	_, err = manager.KVSet(ctx, manifestID, installationID, "other", "temp", "other-gone", time.Minute)
	require.NoError(t, err)
	require.Equal(t, 4, kvCount(t, client, installationID))

	current = start.Add(time.Minute)
	got, err := manager.KVGet(ctx, manifestID, installationID, "default", "early")
	require.NoError(t, err)
	require.Nil(t, got)
	require.Equal(t, 3, kvCount(t, client, installationID))

	listed, err := listKVEntries(manager, ctx, manifestID, installationID, "", "")
	require.NoError(t, err)
	require.Equal(t, []string{"keep"}, keysOf(listed))
	require.Equal(t, "stay", listed[0].Value)
	// Listing the default namespace does not delete another namespace's expired row.
	require.Equal(t, 2, kvCount(t, client, installationID))

	other, err := manager.KVGet(ctx, manifestID, installationID, "other", "temp")
	require.NoError(t, err)
	require.Nil(t, other)
	require.Equal(t, 1, kvCount(t, client, installationID))
	kept, err := manager.KVGet(ctx, manifestID, installationID, "default", "keep")
	require.NoError(t, err)
	require.Equal(t, "stay", kept.Value)
}

func TestKVSurvivesDisableAndRestart(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))
	_, err := manager.KVSet(ctx, installed.PluginID, installed.ID, "", "cursor", "42", 0)
	require.NoError(t, err)
	require.NoError(t, manager.Enable(ctx, installed.ID, nil, nil))
	require.NoError(t, manager.Disable(ctx, installed.ID))

	got, err := manager.KVGet(ctx, installed.PluginID, installed.ID, "default", "cursor")
	require.NoError(t, err)
	require.Equal(t, "42", got.Value)

	restarted := newTestManager(t, client)
	got, err = restarted.KVGet(ctx, installed.PluginID, installed.ID, "", "cursor")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "default", got.Namespace)
	require.Equal(t, "42", got.Value)
	require.Nil(t, got.ExpiresAt)
}

func TestKVInstallationIsolation(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	first := installUI(t, manager, uiZip(t, nil, nil))
	second := installUI(t, manager, otherUIZip(t))

	_, err := manager.KVSet(ctx, first.PluginID, first.ID, "", "cursor", "alpha", 0)
	require.NoError(t, err)
	_, err = manager.KVSet(ctx, second.PluginID, second.ID, "", "cursor", "beta", 0)
	require.NoError(t, err)

	gotFirst, err := manager.KVGet(ctx, first.PluginID, first.ID, "default", "cursor")
	require.NoError(t, err)
	require.Equal(t, "alpha", gotFirst.Value)
	gotSecond, err := manager.KVGet(ctx, second.PluginID, second.ID, "", "cursor")
	require.NoError(t, err)
	require.Equal(t, "beta", gotSecond.Value)

	// The manifest id is only an audit label. The installation id selects the row.
	crossed, err := manager.KVGet(ctx, first.PluginID, second.ID, "", "cursor")
	require.NoError(t, err)
	require.Equal(t, "beta", crossed.Value)

	listed, err := listKVEntries(manager, ctx, first.PluginID, first.ID, "default", "")
	require.NoError(t, err)
	require.Equal(t, []string{"cursor"}, keysOf(listed))
	require.Equal(t, "alpha", listed[0].Value)
}

func TestKVDeletePlugin(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))
	other := installUI(t, manager, otherUIZip(t))
	_, err := manager.KVSet(ctx, installed.PluginID, installed.ID, "", "cursor", "gone", 0)
	require.NoError(t, err)
	_, err = manager.KVSet(ctx, installed.PluginID, installed.ID, "custom", "cursor", "also-gone", 0)
	require.NoError(t, err)
	_, err = manager.KVSet(ctx, other.PluginID, other.ID, "", "cursor", "kept", 0)
	require.NoError(t, err)

	before, err := client.Plugin.Get(ctx, installed.ID)
	require.NoError(t, err)
	require.NotEmpty(t, before.Package)
	require.NotEmpty(t, before.PackageSha256)

	require.NoError(t, manager.Delete(ctx, installed.ID))

	deleted, err := client.Plugin.Query().Where(entplugin.IDEQ(installed.ID)).Only(schematype.SkipSoftDelete(ctx))
	require.NoError(t, err)
	require.NotZero(t, deleted.DeletedAt)
	require.Empty(t, deleted.Package)
	require.Equal(t, before.PackageSha256, deleted.PackageSha256)
	require.Zero(t, kvCount(t, client, installed.ID))

	kept, err := manager.KVGet(ctx, other.PluginID, other.ID, "", "cursor")
	require.NoError(t, err)
	require.Equal(t, "kept", kept.Value)
}

func TestKVLateWriteIsolation(t *testing.T) {
	client := newPluginDB(t)
	writer := newTestManager(t, client)
	paused := newTestManager(t, client)
	ctx := pluginCtx()

	original := installUI(t, writer, uiZip(t, nil, nil))
	_, err := writer.KVSet(ctx, original.PluginID, original.ID, "", "cursor", "old", 0)
	require.NoError(t, err)

	disabled := installUI(t, writer, otherUIZip(t))
	require.NoError(t, writer.Enable(ctx, disabled.ID, nil, nil))
	_, err = writer.KVSet(ctx, disabled.PluginID, disabled.ID, "", "cursor", "kept", 0)
	require.NoError(t, err)
	require.NoError(t, writer.Disable(ctx, disabled.ID))

	require.NoError(t, writer.Delete(ctx, original.ID))
	reinstalled := installUI(t, writer, uiZip(t, nil, map[string]string{"ui/page.html": "<html>reinstall</html>"}))
	require.NotEqual(t, original.ID, reinstalled.ID)
	_, err = writer.KVSet(ctx, reinstalled.PluginID, reinstalled.ID, "", "cursor", "new", 0)
	require.NoError(t, err)

	_, err = paused.KVSet(ctx, original.PluginID, original.ID, "", "cursor", "late", 0)
	require.NoError(t, err)
	late, err := paused.KVGet(ctx, original.PluginID, original.ID, "", "cursor")
	require.NoError(t, err)
	require.Equal(t, "late", late.Value)

	current, err := writer.KVGet(ctx, reinstalled.PluginID, reinstalled.ID, "default", "cursor")
	require.NoError(t, err)
	require.Equal(t, "new", current.Value)

	require.Equal(t, 1, kvCount(t, client, original.ID))
	require.Equal(t, 1, kvCount(t, client, reinstalled.ID))
	kept, err := writer.KVGet(ctx, disabled.PluginID, disabled.ID, "default", "cursor")
	require.NoError(t, err)
	require.Equal(t, "kept", kept.Value)
}

func TestKVJoinsCallerTransaction(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := ent.NewTxContext(ctx, tx)
	txCtx = ent.NewContext(txCtx, tx.Client())
	_, err = manager.KVSet(txCtx, installed.PluginID, installed.ID, "", "cursor", "hidden", 0)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())

	got, err := manager.KVGet(ctx, installed.PluginID, installed.ID, "", "cursor")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestKVPrivacyDeniesWithoutBypass(t *testing.T) {
	client := newPluginDB(t)
	ctx := context.Background()
	_, err := client.PluginStorage.Query().Count(ctx)
	require.Error(t, err)
	err = client.PluginStorage.Create().
		SetInstallationID(1).
		SetKey("cursor").
		SetValue("v").
		Exec(ctx)
	require.Error(t, err)
}

func kvCount(t *testing.T, client *ent.Client, installationID int) int {
	t.Helper()
	count, err := client.PluginStorage.Query().
		Where(pluginstorage.InstallationID(installationID)).
		Count(pluginCtx())
	require.NoError(t, err)
	return count
}

func keysOf(entries []KVEntry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}

func kvKey(index int) string {
	return fmt.Sprintf("k%03d", index)
}

func otherUIZip(t *testing.T) []byte {
	t.Helper()
	return uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.other-tool"
		manifest["name"] = "Other"
	}, map[string]string{"ui/page.html": "<html>other</html>"})
}
