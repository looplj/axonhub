// 职责: 验证 Codex 401 与普通临时失败在持久化状态、进程缓存和通知上的不同收敛。
// 关联文件:
//
//	internal/server/biz/provider_quota.go
//	internal/server/biz/provider_quota/codex_checker.go
//
// 注意事项:
//
//	缓存是进程内状态，种子行必须在创建 service 前写入；断言需同时比对数据库与缓存。
//	“临时失败保留旧 exhausted”是既有观测语义，不能当作已恢复。
package biz

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm/httpclient"
)

type codexQuotaTransportFunc func(*http.Request) (*http.Response, error)

func (f codexQuotaTransportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func codexQuotaStatusResponse(statusCode int) codexQuotaTransportFunc {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":"upstream rejected the request"}`)),
		}, nil
	}
}

func buildCodexQuotaServiceTestJWT(t *testing.T) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_quota_state_freshness",
		},
	})

	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	return signed
}

// newCodexCredentialTestChannel creates an OAuth-backed Codex channel so the
// usage checker reaches the HTTP probe instead of failing on missing credentials.
func newCodexCredentialTestChannel(t *testing.T, ctx context.Context, client *ent.Client) *ent.Channel {
	t.Helper()

	ch := createProviderQuotaCollectionChannel(t, ctx, client, "Codex", channel.TypeCodex)
	require.NoError(t, client.Channel.UpdateOne(ch).
		SetCredentials(objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: buildCodexQuotaServiceTestJWT(t)},
		}).
		Exec(ctx))

	return ch
}

func seedExhaustedCodexQuotaRow(t *testing.T, ctx context.Context, client *ent.Client, channelID int, nextResetAt time.Time) {
	t.Helper()

	seedQuotaRow(t, ctx, client, channelID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusExhausted, map[string]any{
		"plan_type": "plus",
		"_limits": []provider_quota.QuotaLimitStatus{{
			Type:        provider_quota.QuotaLimitTypeToken,
			Status:      string(providerquotastatus.StatusExhausted),
			UsageRatio:  1,
			Ready:       false,
			NextResetAt: &nextResetAt,
			Window:      provider_quota.QuotaWindow5h,
		}},
	})
	affected, err := client.ProviderQuotaStatus.Update().
		Where(providerquotastatus.ChannelIDEQ(channelID)).
		SetNextResetAt(nextResetAt).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, affected)
}

// TestCheckChannelQuota_Codex401DemotesStaleExhaustedWithoutRecovery covers the
// credential-expiry path end to end: the checker sees HTTP 401, the service
// demotes the previously exhausted row to unknown everywhere, drops the old
// limits and reset, and does not report a quota recovery.
func TestCheckChannelQuota_Codex401DemotesStaleExhaustedWithoutRecovery(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	ch := newCodexCredentialTestChannel(t, ctx, client)

	oldResetAt := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	seedExhaustedCodexQuotaRow(t, ctx, client, ch.ID, oldResetAt)

	svc, notifications, requery := newTestQuotaWebhookHarness(t, client)
	svc.checkers["codex"] = provider_quota.NewCodexQuotaChecker(
		httpclient.NewHttpClientWithClient(&http.Client{Transport: codexQuotaStatusResponse(http.StatusUnauthorized)}),
	)

	loaded := requery(ch)
	svc.checkChannelQuota(ctx, quotaCheckGroup{channels: []*ent.Channel{loaded}}, time.Now())

	persisted, err := client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(ch.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, providerquotastatus.StatusUnknown, persisted.Status)
	require.False(t, persisted.Ready)
	require.Equal(t, "exhausted", persisted.QuotaData["_previous_status"])
	require.Equal(t, "check_failed", persisted.QuotaData["error_code"])
	require.NotContains(t, persisted.QuotaData, "_limits",
		"stale limits must not survive a dead credential")
	require.Nil(t, persisted.NextResetAt, "a stale reset must not keep pretending to be a live quota window")

	cached := svc.GetQuotaStatus(ctx, ch.ID)
	require.NotNil(t, cached)
	require.Equal(t, providerquotastatus.StatusUnknown, cached.Status)
	require.False(t, cached.Ready)
	require.Empty(t, cached.Limits, "the cache must drop limits together with the database row")
	require.Equal(t, "exhausted", cached.LastKnownStatus)
	require.Equal(t, persisted.Status.String(), cached.Status.String(), "database and cache must agree")

	expectNoQuotaWebhook(t, notifications)
}

// TestCheckChannelQuota_CodexTransientFailureKeepsExhausted pins the existing
// observation semantics: a 5xx probe failure is not a credential problem and
// must not silently turn the channel back into an available or unknown quota.
func TestCheckChannelQuota_CodexTransientFailureKeepsExhausted(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	ch := newCodexCredentialTestChannel(t, ctx, client)

	oldResetAt := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	seedExhaustedCodexQuotaRow(t, ctx, client, ch.ID, oldResetAt)

	svc, notifications, requery := newTestQuotaWebhookHarness(t, client)
	svc.checkers["codex"] = provider_quota.NewCodexQuotaChecker(
		httpclient.NewHttpClientWithClient(&http.Client{Transport: codexQuotaStatusResponse(http.StatusInternalServerError)}),
	)

	loaded := requery(ch)
	svc.checkChannelQuota(ctx, quotaCheckGroup{channels: []*ent.Channel{loaded}}, time.Now())

	persisted, err := client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(ch.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, providerquotastatus.StatusExhausted, persisted.Status)
	require.False(t, persisted.Ready)
	require.Equal(t, "check_failed", persisted.QuotaData["error_code"])
	require.EqualValues(t, 1, persisted.QuotaData["error_count"])
	require.Contains(t, persisted.QuotaData, "_limits", "a transient failure keeps the observed limits")
	require.NotNil(t, persisted.NextResetAt)
	require.True(t, persisted.NextResetAt.Equal(oldResetAt))

	cached := svc.GetQuotaStatus(ctx, ch.ID)
	require.NotNil(t, cached)
	require.Equal(t, providerquotastatus.StatusExhausted, cached.Status)
	require.False(t, cached.Ready)
	require.Len(t, cached.Limits, 1)
	require.Equal(t, persisted.Status.String(), cached.Status.String(), "database and cache must agree")

	expectNoQuotaWebhook(t, notifications)
}
