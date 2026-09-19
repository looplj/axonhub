package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/objects"
)

func TestMetricResourceNamesRefresh(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:metric_names?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(t.Context())
	p, err := client.Project.Create().SetName("研发项目").Save(ctx)
	require.NoError(t, err)
	u, err := client.User.Create().SetEmail("private@example.com").SetPassword("password-secret").SetFirstName("Alice").SetLastName("Lee").Save(ctx)
	require.NoError(t, err)
	k, err := client.APIKey.Create().SetName("生产调用").SetKey("key-secret").SetProjectID(p.ID).SetUserID(u.ID).Save(ctx)
	require.NoError(t, err)
	c, err := client.Channel.Create().SetName("主渠道").SetType(channel.TypeOpenai).SetBaseURL("https://example.com").SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetCredentials(objects.ChannelCredentials{APIKey: "provider-secret"}).Save(ctx)
	require.NoError(t, err)
	svc := &metricResourceInfo{client: client}
	names, err := svc.snapshot(t.Context())
	require.NoError(t, err)
	require.Contains(t, names.Projects, metrics.ResourceName{ID: p.ID, Name: "研发项目"})
	require.Contains(t, names.Users, metrics.ResourceName{ID: u.ID, Name: "Alice Lee"})
	require.Contains(t, names.APIKeys, metrics.ResourceName{ID: k.ID, Name: "生产调用"})
	require.Contains(t, names.Channels, metrics.ResourceName{ID: c.ID, Name: "主渠道"})

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	reg, err := metrics.RegisterResourceInfo(provider.Meter("test"), svc.snapshot)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reg.Unregister() })
	collect := func() metricdata.ResourceMetrics {
		var result metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(t.Context(), &result))
		return result
	}
	before := collect()
	require.NotContains(t, fmtResourceNames(before), "key-secret")
	require.NotContains(t, fmtResourceNames(before), "private@example.com")
	require.Contains(t, fmtResourceNames(before), "主渠道")
	require.NoError(t, client.Channel.UpdateOneID(c.ID).SetName("新渠道").Exec(ctx))
	require.Contains(t, fmtResourceNames(collect()), "主渠道")
	svc.loadedAt = time.Now().Add(-2 * time.Minute)
	after := fmtResourceNames(collect())
	require.Contains(t, after, "新渠道")
	require.NotContains(t, after, "主渠道")
}

func fmtResourceNames(r metricdata.ResourceMetrics) []string {
	var labels []string
	for _, scope := range r.ScopeMetrics {
		for _, mt := range scope.Metrics {
			for _, point := range mt.Data.(metricdata.Gauge[int64]).DataPoints {
				for _, attr := range point.Attributes.ToSlice() {
					labels = append(labels, attr.Value.AsString())
				}
			}
		}
	}
	return labels
}
