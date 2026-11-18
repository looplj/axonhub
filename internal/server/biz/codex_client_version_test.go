// 职责: 验证 Codex 版本缓存、渠道覆盖、账号目录隔离和同步回退。
// 关联文件:
//   internal/server/biz/codex_client_version.go
//   internal/server/biz/model_fetcher.go
//   internal/server/biz/channel_model_sync.go

package biz

import (
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
	require.Equal(t, "0.158.0", result.CodexCatalogVersion.Version)
	require.Equal(t, persisted, db.Channel.GetX(ctx, b.ID).SupportedModels)
}
