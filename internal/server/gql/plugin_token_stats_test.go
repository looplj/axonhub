package gql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
)

func TestPluginTokenStatsActualQuery(t *testing.T) {
	reset := func() {
		allTimeCacheMu.Lock()
		allTimeCache = nil
		allTimeCacheTime = time.Time{}
		allTimeCacheMu.Unlock()
		allTimeRefreshGroup.Forget("allTimeTokenStats")
	}
	reset()
	t.Cleanup(reset)
	fx := newPluginEndpointFixture(t, "plugin_token_stats")
	fx.db.User.UpdateOneID(fx.readerID).SetScopes([]string{"read_dashboard"}).ExecX(fx.setupCtx)
	project := fx.db.Project.Query().OnlyX(fx.setupCtx)
	req := fx.db.Request.Create().SetProjectID(project.ID).SetModelID("test-model").SetRequestBody([]byte(`{}`)).SetStream(false).SetStatus(request.StatusCompleted).SaveX(fx.setupCtx)
	usage := fx.db.UsageLog.Create().SetRequestID(req.ID).SetProjectID(project.ID).SetModelID("test-model").SetPromptTokens(123).SetCompletionTokens(0).SetTotalTokens(123).SaveX(fx.setupCtx)
	install := func(id string, grants []string) int {
		result, err := fx.manager.Install(fx.setupCtx, pluginEndpointZip(t, id, []string{"read_dashboard"}, true), nil)
		require.NoError(t, err)
		require.NoError(t, fx.manager.Enable(fx.setupCtx, result.Plugin.ID, grants))
		return result.Plugin.ID
	}
	query := `{ tokenStats { totalInputTokensToday totalInputTokensThisWeek totalInputTokensThisMonth totalInputTokensAllTime } }`
	_, payload, raw := fx.postAdmin(fx.ownerToken, query)
	require.NotContains(t, payload, "errors", raw)
	require.Contains(t, raw, `"totalInputTokensAllTime":123`)
	install("acme.empty", nil)
	_, payload, raw = fx.postPlugin(fx.ownerToken, "acme.empty", query)
	require.Contains(t, payload, "errors", raw)
	require.NotContains(t, raw, `"totalInputTokensAllTime":123`)
	install("acme.tokens-a", []string{"read_dashboard"})
	other := install("acme.tokens-b", []string{"read_dashboard"})
	fx.db.UsageLog.UpdateOneID(usage.ID).SetPromptTokens(234).ExecX(fx.setupCtx)
	_, payload, raw = fx.postPlugin(fx.ownerToken, "acme.tokens-a", query)
	require.NotContains(t, payload, "errors", raw)
	// All-time totals are caller-independent and come from the shared cache.
	require.Contains(t, raw, `"totalInputTokensAllTime":123`)
	for _, field := range []string{"totalInputTokensToday", "totalInputTokensThisWeek", "totalInputTokensThisMonth"} {
		require.Equal(t, float64(234), payload["data"].(map[string]any)["tokenStats"].(map[string]any)[field], raw)
	}
	_, payload, raw = fx.postPlugin(fx.readerToken, "acme.tokens-a", query)
	require.NotContains(t, payload, "errors", raw)
	for _, field := range []string{"totalInputTokensToday", "totalInputTokensThisWeek", "totalInputTokensThisMonth"} {
		require.Equal(t, float64(234), payload["data"].(map[string]any)["tokenStats"].(map[string]any)[field], raw)
	}
	// Neither the user's Owner status nor the fixed statistic may authorize raw entities.
	for _, token := range []string{fx.ownerToken, fx.readerToken} {
		for _, rawQuery := range []string{`{ requests(first: 1) { edges { node { id } } } }`, `{ usageLogs(first: 1) { edges { node { id } } } }`} {
			_, payload, raw = fx.postPlugin(token, "acme.tokens-a", rawQuery)
			require.Contains(t, payload, "errors", raw)
		}
	}
	_, payload, raw = fx.postPlugin(fx.writerToken, "acme.tokens-a", query)
	require.Contains(t, payload, "errors", raw, "user without read_dashboard cannot borrow plugin authority")

	fx.db.UsageLog.UpdateOneID(usage.ID).SetPromptTokens(345).ExecX(fx.setupCtx)
	_, payload, raw = fx.postPlugin(fx.ownerToken, "acme.tokens-b", query)
	require.NotContains(t, payload, "errors", raw)
	require.Equal(t, float64(345), payload["data"].(map[string]any)["tokenStats"].(map[string]any)["totalInputTokensToday"], raw)
	// Exercise every fixed read_dashboard entry point with only that permission.
	_, payload, raw = fx.postPlugin(fx.readerToken, "acme.tokens-a", `{
 dashboardOverview { totalRequests }
 requestStats { requestsToday }
 requestStatsByChannel { count }
 requestStatsByModel { modelId count }
 requestStatsByAPIKey { count }
 tokenStatsByAPIKey { totalTokens }
 dailyRequestStats { count }
 topRequestsProjects { requestCount }
 tokenStats { totalInputTokensAllTime }
 channelSuccessRates { totalCount }
 fastestChannels(input: {timeWindow: "day"}) { requestCount }
 fastestModels(input: {timeWindow: "day"}) { requestCount }
 modelPerformanceStats { requestCount }
 channelPerformanceStats { requestCount }
 tokenStatsByChannel { totalTokens }
 tokenStatsByModel { inputTokens }
 costStatsByChannel { cost }
 costStatsByModel { cost }
 costStatsByAPIKey { cost }
 analyticsOverview { totalInputTokens totalRequests }
 analyticsDailyStats { inputTokens requestCount }
 analyticsDimensionStats(dimension: "model") { inputTokens requestCount }
 }`)
	require.NotContains(t, payload, "errors", raw)
	data := payload["data"].(map[string]any)
	require.Len(t, data, 22)
	require.Equal(t, float64(1), data["dashboardOverview"].(map[string]any)["totalRequests"])
	require.Equal(t, float64(1), data["requestStats"].(map[string]any)["requestsToday"])
	models := data["requestStatsByModel"].([]any)
	require.Len(t, models, 1)
	require.Equal(t, float64(1), models[0].(map[string]any)["count"])
	require.Equal(t, float64(345), data["analyticsOverview"].(map[string]any)["totalInputTokens"])
	dimensions := data["analyticsDimensionStats"].([]any)
	require.Len(t, dimensions, 1)
	require.Equal(t, float64(345), dimensions[0].(map[string]any)["inputTokens"])

	require.NoError(t, fx.manager.Enable(fx.setupCtx, other, nil))
	_, payload, raw = fx.postPlugin(fx.ownerToken, "acme.tokens-b", query)
	require.Contains(t, payload, "errors", raw)
	require.NotContains(t, raw, `"totalInputTokensAllTime"`, "revoked plugin must not read the shared cache")
}

func TestPluginDashboardDoesNotExportAuthority(t *testing.T) {
	resolver, _ := newDashboardUsageTestResolver(t)
	user := &ent.User{ID: 7, Scopes: []string{"read_dashboard"}}
	parent := contexts.WithUser(authz.NewUserContext(ent.NewContext(context.Background(), resolver.client), user.ID), user)
	ctx, err := authz.DelegatePlugin(parent, 77, []string{"read_dashboard"})
	require.NoError(t, err)
	original, _ := authz.GetPrincipal(ctx)
	_, err = resolver.RequestStats(ctx)
	require.NoError(t, err)
	// The scope decision stays inside the resolver: the caller context keeps the
	// Plugin principal and gains no decision, so raw entities remain denied.
	current, _ := authz.GetPrincipal(ctx)
	require.Equal(t, original, current)
	_, hasDecision := privacy.DecisionFromContext(ctx)
	require.False(t, hasDecision)
	_, err = resolver.client.UsageLog.Query().All(ctx)
	require.Error(t, err)
	_, err = resolver.client.UsageLog.Query().All(authz.WithSystemBypass(ctx, "must-still-reject"))
	require.Error(t, err)
	empty, err := authz.DelegatePlugin(parent, 78, nil)
	require.NoError(t, err)
	_, err = resolver.TokenStats(empty)
	require.Error(t, err)
}
