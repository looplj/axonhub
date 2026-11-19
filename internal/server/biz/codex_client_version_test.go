// 职责: 验证 Codex 版本缓存、渠道覆盖、账号目录隔离和同步回退。
// 关联文件:
//   internal/server/biz/codex_client_version.go
//   internal/server/biz/model_fetcher.go
//   internal/server/biz/channel_model_sync.go

package biz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

func TestCodexVersionFromUserAgent(t *testing.T) {
	for _, tc := range []struct {
		ua, version string
	}{
		{"codex/0.170.0", "0.170.0"},
		{"codex_cli_rs/0.171.0-alpha.12 (test)", "0.171.0"},
		{"codex_tui/0.171.0", "0.171.0"},
		{"codex-tui/0.171.0", "0.171.0"},
		{"codex_exec/0.171.0", "0.171.0"},
		{"codex_vscode/0.171.0", "0.171.0"},
		{"Codex Desktop/0.170.0 (Macintosh)", "0.170.0"},
		{"codex_atlas/0.171.0", "0.171.0"},
		{"codex_chatgpt_desktop/0.171.0", "0.171.0"},
		{"codex_sdk_ts/0.171.0", "0.171.0"},
		{"codex_mcp_server/0.171.0", "0.171.0"},
		{"codex-app-server/0.171.0", "0.171.0"},
		{"codex_relay/2.0.0", ""},
		{"Codex_Future/0.172.0", ""},
		{"CodexProxy/2.0.0", ""},
		{"MyRelay/2.0.0 codex_cli_rs/0.171.0", ""},
		{"codex_exec/invalid", ""},
		{"Codex Desktop/ 0.170.0", ""},
	} {
		require.Equal(t, tc.version, codexVersionFromUserAgent(tc.ua), tc.ua)
	}
}

func TestCodexClientRelease_ContinuesAfterConcurrentFirstInsert(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { db.Close() })
	svc := &ChannelService{AbstractService: &AbstractService{db: db}}
	candidate := codexRelease{Version: "0.200.0", LastChecked: time.Now().UTC()}
	other := codexRelease{Version: "0.199.0", LastChecked: candidate.LastChecked.Add(time.Second)}
	otherValue, err := json.Marshal(other)
	require.NoError(t, err)

	// Insert the competing row after saveCodexRelease reads the empty table.
	inserted := false
	db.System.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			if !inserted {
				inserted = true
				if err := db.System.Create().SetKey(codexReleaseKey).SetValue(string(otherValue)).Exec(ctx); err != nil {
					return nil, err
				}
			}
			return next.Mutate(ctx, m)
		})
	})

	saved, err := svc.saveCodexRelease(t.Context(), candidate)
	require.NoError(t, err)
	require.True(t, inserted)
	require.Equal(t, candidate.Version, saved.Version)
	require.Equal(t, other.LastChecked, saved.LastChecked)

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	row, err := db.System.Query().Where(system.KeyEQ(codexReleaseKey)).Only(ctx)
	require.NoError(t, err)
	var stored codexRelease
	require.NoError(t, json.Unmarshal([]byte(row.Value), &stored))
	require.Equal(t, saved, stored)
}

func TestCodexClientRelease_StopsAfterFiveAttemptsWithSoftDeletedKey(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { db.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	row := db.System.Create().SetKey(codexReleaseKey).SetValue(`{"version":"0.199.0"}`).SaveX(ctx)
	require.NoError(t, db.System.DeleteOne(row).Exec(ctx))
	require.False(t, db.System.Query().Where(system.KeyEQ(codexReleaseKey)).ExistX(ctx))

	attempts := 0
	db.System.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			attempts++
			return next.Mutate(ctx, m)
		})
	})

	svc := &ChannelService{AbstractService: &AbstractService{db: db}}
	saveCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err := svc.saveCodexRelease(saveCtx, codexRelease{Version: "0.200.0", LastChecked: time.Now().UTC()})
	require.ErrorContains(t, err, "after 5 attempts")
	require.Equal(t, 5, attempts)
}

func TestCodexClientVersion_FetchModelsRefreshesBeforeDiscovery(t *testing.T) {
	var checks atomic.Int32
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch checks.Add(1) {
		case 1:
			_, _ = io.WriteString(w, `{"version":"0.200.0"}`)
		case 2:
			_, _ = io.WriteString(w, `{"version":"0.201.0"}`)
		default:
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer registry.Close()
	svc := &ChannelService{AbstractService: &AbstractService{}}
	svc.codexVersions.registryURL = registry.URL
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "Bearer synthetic-token", r.Header.Get("Authorization"))
		version := r.URL.Query().Get("client_version")
		require.Equal(t, version, r.Header.Get("Version"))
		return &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"models":[{"slug":%q,"visibility":"list"}]}`, version))),
		}, nil
	})})
	fetcher := NewModelFetcher(client, svc)
	input := FetchModelsInput{ChannelType: "codex", BaseURL: "https://chatgpt.com/backend-api/codex"}
	result, err := fetcher.FetchModels(t.Context(), input)
	require.NoError(t, err)
	require.True(t, result.Fallback)
	require.Equal(t, codexCatalogNoCredentials, *result.CodexCatalogSkippedReason)
	require.Nil(t, result.CodexCatalogVersion)
	require.Zero(t, checks.Load())

	key := `{"access_token":"synthetic-token","token_type":"bearer","expires_at":"2099-01-01T00:00:00Z"}`
	input.APIKey = &key
	for _, expected := range []CodexCatalogVersion{{"0.200.0", "npm"}, {"0.201.0", "npm"}, {"0.201.0", "cache"}} {
		result, err := fetcher.FetchModels(t.Context(), input)
		require.NoError(t, err)
		require.False(t, result.Fallback)
		require.Nil(t, result.CodexCatalogSkippedReason)
		require.Equal(t, &expected, result.CodexCatalogVersion)
		require.Contains(t, result.Models, ModelIdentify{ID: expected.Version})
	}
	require.EqualValues(t, 3, checks.Load())
}

func TestCodexClientVersion_StaticOAuthCatalogSkipsSync(t *testing.T) {
	svc, db := setupTestChannelService(t)
	t.Cleanup(func() { db.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	ch := db.Channel.Create().SetName("custom-codex-oauth").SetType(channel.TypeCodex).
		SetBaseURL("https://relay.example.com/codex").SetDefaultTestModel("existing").
		SetSupportedModels([]string{"existing"}).
		SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "synthetic-token"}}).SaveX(ctx)
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("static catalog should not trigger an npm check")
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer registry.Close()
	svc.codexVersions.registryURL = registry.URL

	result, err := svc.SyncChannelModels(ctx, ch.ID, nil)
	require.NoError(t, err)
	require.True(t, result.Fallback)
	require.Equal(t, codexCatalogUnsupportedURL, *result.CodexCatalogSkippedReason)
	require.Nil(t, result.CodexCatalogVersion)
	require.Equal(t, ch.SupportedModels, db.Channel.GetX(ctx, ch.ID).SupportedModels)
}

func TestCodexClientRelease_PersistsAfterModelSyncTransaction(t *testing.T) {
	for _, commit := range []bool{true, false} {
		t.Run(fmt.Sprintf("commit=%t", commit), func(t *testing.T) {
			svc, db := setupTestChannelService(t)
			t.Cleanup(func() { db.Close() })
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
			registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, `{"version":"0.201.0"}`)
			}))
			defer registry.Close()
			svc.codexVersions.registryURL = registry.URL
			svc.httpClient = httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, "0.201.0", r.URL.Query().Get("client_version"))
				return &http.Response{
					StatusCode: http.StatusOK, Header: make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{"models":[{"slug":"new-model","visibility":"list"}]}`)),
				}, nil
			})})
			ch := db.Channel.Create().SetName("transactional-codex").SetType(channel.TypeCodex).
				SetBaseURL("https://chatgpt.com/backend-api/codex").SetDefaultTestModel("existing").
				SetSupportedModels([]string{"existing"}).SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{
				AccessToken: "synthetic-token", ExpiresAt: time.Now().Add(time.Hour),
			}}).SaveX(ctx)

			tx, err := db.Tx(ctx)
			require.NoError(t, err)
			defer tx.Rollback()
			txCtx := ent.NewContext(ent.NewTxContext(ctx, tx), tx.Client())
			// Hold a SQLite write lock before model sync, as an earlier mutation can.
			require.NoError(t, tx.System.Create().SetKey("other-mutation").SetValue("pending").Exec(txCtx))
			result, err := svc.SyncChannelModels(txCtx, ch.ID, nil)
			require.NoError(t, err)
			require.Equal(t, &CodexCatalogVersion{"0.201.0", "npm"}, result.CodexCatalogVersion)
			require.Contains(t, result.SupportedModels, "new-model")
			require.False(t, tx.System.Query().Where(system.KeyEQ(codexReleaseKey)).ExistX(txCtx))
			if commit {
				require.NoError(t, tx.Commit())
				require.Contains(t, db.Channel.GetX(ctx, ch.ID).SupportedModels, "new-model")
			} else {
				require.NoError(t, tx.Rollback())
				require.Equal(t, []string{"existing"}, db.Channel.GetX(ctx, ch.ID).SupportedModels)
				require.False(t, db.System.Query().Where(system.KeyEQ("other-mutation")).ExistX(ctx))
			}
			row, err := db.System.Query().Where(system.KeyEQ(codexReleaseKey)).Only(ctx)
			require.NoError(t, err)
			var saved codexRelease
			require.NoError(t, json.Unmarshal([]byte(row.Value), &saved))
			require.Equal(t, "0.201.0", saved.Version)
		})
	}
}

func TestCodexClientRelease_PersistsLatestStableVersion(t *testing.T) {
	ctx := t.Context()
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { db.Close() })

	var latest atomic.Value
	latest.Store("0.199.0")
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("Cookie"))
		_ = json.NewEncoder(w).Encode(map[string]string{"version": latest.Load().(string)})
	}))
	defer registry.Close()

	svc := &ChannelService{AbstractService: &AbstractService{db: db}}
	svc.codexVersions.registryURL = registry.URL
	require.Equal(t, CodexCatalogVersion{codex.DefaultClientVersion, "builtin"}, svc.resolveCodexClientVersion(nil))
	svc.refreshCodexRelease(ctx)
	require.Equal(t, CodexCatalogVersion{"0.199.0", "npm"}, svc.resolveCodexClientVersion(nil))
	latest.Store("0.200.0")
	svc.refreshCodexRelease(ctx)
	require.Equal(t, CodexCatalogVersion{"0.200.0", "npm"}, svc.resolveCodexClientVersion(nil))

	// The other instance cannot persist an older npm version over the stored release.
	restarted := &ChannelService{AbstractService: &AbstractService{db: db}}
	restarted.codexVersions.registryURL = registry.URL
	latest.Store("0.99.0")
	restarted.refreshCodexRelease(ctx)
	require.Equal(t, CodexCatalogVersion{"0.200.0", "cache"}, restarted.resolveCodexClientVersion(nil))

	dbCtx := authz.WithTestBypass(ent.NewContext(ctx, db))
	before, err := db.System.Query().Where(system.KeyEQ(codexReleaseKey)).Only(dbCtx)
	require.NoError(t, err)
	var stored codexRelease
	require.NoError(t, json.Unmarshal([]byte(before.Value), &stored))
	require.Equal(t, "0.200.0", stored.Version)
	require.False(t, stored.LastChecked.IsZero())

	latest.Store("0.201.0-alpha.1")
	restarted.refreshCodexRelease(ctx)
	require.Equal(t, before.Value, db.System.GetX(dbCtx, before.ID).Value)
	registry.Close()
	offline := &ChannelService{AbstractService: &AbstractService{db: db}}
	offline.codexVersions.registryURL = registry.URL
	offline.loadCodexRelease(ctx)
	offline.refreshCodexRelease(ctx)
	require.Equal(t, CodexCatalogVersion{"0.200.0", "cache"}, offline.resolveCodexClientVersion(nil))
	require.Equal(t, before.Value, db.System.GetX(dbCtx, before.ID).Value)
}

func TestCodexClientVersion_ManualSyncAndAccountCatalogs(t *testing.T) {
	svc, db := setupTestChannelService(t)
	t.Cleanup(func() { db.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))

	var registryChecks atomic.Int32
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch registryChecks.Add(1) {
		case 1:
			_, _ = io.WriteString(w, `{"version":"0.199.0"}`)
		case 2:
			_, _ = io.WriteString(w, `{"version":"0.200.0"}`)
		default:
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer registry.Close()
	svc.codexVersions.registryURL = registry.URL

	const upstreamURL = "https://chatgpt.com/backend-api/codex"
	const channelUA = "codex_cli_rs/0.160.0 (test)"
	tokens := make(map[string]string)
	var gotHeaders http.Header
	var catalogVersion string
	var catalogFailed bool
	svc.httpClient = httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/backend-api/codex/models", r.URL.Path)
		account := r.Header.Get("Chatgpt-Account-Id")
		require.Contains(t, tokens, account)
		require.Equal(t, "Bearer "+tokens[account], r.Header.Get("Authorization"))
		gotHeaders = r.Header.Clone()
		versions := r.URL.Query()["client_version"]
		require.Len(t, versions, 1)
		catalogVersion = versions[0]
		status := http.StatusOK
		body := fmt.Sprintf(`{"models":[{"slug":%q,"visibility":"list"}]}`, "model-"+account)
		if catalogFailed {
			status, body = http.StatusBadGateway, `{}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})})
	for _, account := range []string{"a", "b"} {
		payload := fmt.Sprintf(`{"https://api.openai.com/auth":{"chatgpt_account_id":%q}}`, account)
		tokens[account] = "eyJhbGciOiJIUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
	}
	create := func(account string) *ent.Channel {
		return db.Channel.Create().SetName("account-" + account).SetType(channel.TypeCodex).SetBaseURL(upstreamURL).
			SetDefaultTestModel("existing").SetSupportedModels([]string{"existing"}).
			SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{
				AccessToken: tokens[account], ExpiresAt: time.Now().Add(time.Hour),
			}}).SaveX(ctx)
	}
	a, b := create("a"), create("b")
	for _, version := range []string{"0.199.0", "0.200.0"} {
		result, err := svc.SyncChannelModels(ctx, a.ID, nil)
		require.NoError(t, err)
		require.Equal(t, &CodexCatalogVersion{version, "npm"}, result.CodexCatalogVersion)
		require.Equal(t, version, catalogVersion)
		require.Equal(t, version, gotHeaders.Get("Version"))
	}

	result, err := svc.SyncChannelModels(ctx, b.ID, nil)
	require.NoError(t, err)
	require.False(t, result.Fallback)
	require.Equal(t, &CodexCatalogVersion{"0.200.0", "cache"}, result.CodexCatalogVersion)
	require.Contains(t, result.SupportedModels, "model-b")
	require.NotContains(t, result.SupportedModels, "model-a")
	require.Contains(t, db.Channel.GetX(ctx, a.ID).SupportedModels, "model-a")

	ops := []objects.OverrideOperation{
		{Op: objects.OverrideOpSet, Path: "Version", Value: "0.158.0"},
		{Op: objects.OverrideOpSet, Path: "User-Agent", Value: channelUA},
		{Op: objects.OverrideOpSet, Path: "Version", Value: "0.300.0", Condition: "model == 'other'"},
		{Op: objects.OverrideOpSet, Path: "Version", Value: "{{ .Model }}"},
	}
	db.Channel.UpdateOneID(b.ID).SetSettings(&objects.ChannelSettings{HeaderOverrideOperations: ops}).SaveX(ctx)
	result, err = svc.SyncChannelModels(ctx, b.ID, nil)
	require.NoError(t, err)
	require.Equal(t, &CodexCatalogVersion{"0.158.0", "channel_version"}, result.CodexCatalogVersion)
	require.Equal(t, "0.158.0", catalogVersion)
	require.Equal(t, "0.158.0", gotHeaders.Get("Version"))
	require.Equal(t, channelUA, gotHeaders.Get("User-Agent"))
	require.EqualValues(t, 4, registryChecks.Load())
	require.Equal(t, "0.200.0", svc.resolveCodexClientVersion(nil).Version)

	catalogFailed = true
	persisted := db.Channel.GetX(ctx, b.ID).SupportedModels
	result, err = svc.SyncChannelModels(ctx, b.ID, nil)
	require.ErrorContains(t, err, "fallback")
	require.True(t, result.Fallback)
	require.Nil(t, result.CodexCatalogVersion)
	require.Equal(t, persisted, db.Channel.GetX(ctx, b.ID).SupportedModels)
}
