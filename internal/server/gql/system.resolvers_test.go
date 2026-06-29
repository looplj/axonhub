package gql

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func setupTestSystemMutationResolver(t *testing.T) (*mutationResolver, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	systemService := &biz.SystemService{
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	resolver := &mutationResolver{&Resolver{systemService: systemService}}
	return resolver, ctx, client
}

func TestMutationResolver_UpdateSystemChannelSettings_MergesAutoSyncWithoutOverwritingProbe(t *testing.T) {
	resolver, ctx, client := setupTestSystemMutationResolver(t)
	defer client.Close()

	err := resolver.systemService.SetChannelSetting(ctx, biz.SystemChannelSettings{
		Probe: biz.ChannelProbeSetting{
			Enabled:   true,
			Frequency: biz.ProbeFrequency5Min,
		},
		AutoSync: biz.ChannelModelAutoSyncSetting{
			Frequency: biz.AutoSyncFrequencyOneHour,
		},
	})
	require.NoError(t, err)

	ok, err := resolver.UpdateSystemChannelSettings(ctx, biz.SystemChannelSettings{
		AutoSync: biz.ChannelModelAutoSyncSetting{
			Frequency: biz.AutoSyncFrequencySixHours,
		},
	})
	require.NoError(t, err)
	require.True(t, ok)

	setting, err := resolver.systemService.ChannelSetting(ctx)
	require.NoError(t, err)
	require.True(t, setting.Probe.Enabled)
	require.Equal(t, biz.ProbeFrequency5Min, setting.Probe.Frequency)
	require.Equal(t, biz.AutoSyncFrequencySixHours, setting.AutoSync.Frequency)
}

func TestMutationResolver_UpdateSystemChannelSettings_MergesProbeWithoutOverwritingAutoSync(t *testing.T) {
	resolver, ctx, client := setupTestSystemMutationResolver(t)
	defer client.Close()

	err := resolver.systemService.SetChannelSetting(ctx, biz.SystemChannelSettings{
		Probe: biz.ChannelProbeSetting{
			Enabled:   true,
			Frequency: biz.ProbeFrequency5Min,
		},
		AutoSync: biz.ChannelModelAutoSyncSetting{
			Frequency: biz.AutoSyncFrequencySixHours,
		},
	})
	require.NoError(t, err)

	ok, err := resolver.UpdateSystemChannelSettings(ctx, biz.SystemChannelSettings{
		Probe: biz.ChannelProbeSetting{
			Enabled:   false,
			Frequency: biz.ProbeFrequency1Hour,
		},
	})
	require.NoError(t, err)
	require.True(t, ok)

	setting, err := resolver.systemService.ChannelSetting(ctx)
	require.NoError(t, err)
	require.False(t, setting.Probe.Enabled)
	require.Equal(t, biz.ProbeFrequency1Hour, setting.Probe.Frequency)
	require.Equal(t, biz.AutoSyncFrequencySixHours, setting.AutoSync.Frequency)
}

func TestMutationResolver_UpdateRetryPolicy_PreservesOmittedFields(t *testing.T) {
	resolver, ctx, client := setupTestSystemMutationResolver(t)
	defer client.Close()

	err := resolver.systemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled:                         true,
		MaxChannelRetries:               4,
		MaxSingleChannelRetries:         2,
		RetryDelayMs:                    300,
		StreamFirstEventTimeoutSeconds:  10,
		NonStreamResponseTimeoutSeconds: 20,
		LoadBalancerStrategy:            biz.LoadBalancerStrategyAdaptive,
		AutoDisableChannel: biz.AutoDisableChannel{
			Enabled: true,
			Statuses: []biz.AutoDisableChannelStatus{
				{Status: http.StatusTooManyRequests, Times: 3},
			},
		},
		EmptyResponseDetection: true,
		UpstreamErrorPolicy: biz.UpstreamErrorPolicy{
			Mode:          biz.UpstreamErrorModeCustom,
			CustomMessage: "custom upstream failure",
		},
		StreamProbeDurationMs: 5000,
	})
	require.NoError(t, err)

	srv := handler.New(NewExecutableSchema(Config{Resolvers: resolver.Resolver}))
	srv.AddTransport(transport.POST{})
	body, err := json.Marshal(map[string]any{
		"query": `mutation($input: UpdateRetryPolicyInput!) {
			updateRetryPolicy(input: $input)
		}`,
		"variables": map[string]any{
			"input": map[string]any{
				"maxChannelRetries": 5,
				"autoDisableChannel": map[string]any{
					"enabled": false,
				},
				"upstreamErrorPolicy": map[string]any{
					"mode": biz.UpstreamErrorModeHidden,
				},
			},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), `"errors"`)

	policy, err := resolver.systemService.RetryPolicy(ctx)
	require.NoError(t, err)
	require.Equal(t, 5, policy.MaxChannelRetries)
	require.Equal(t, 2, policy.MaxSingleChannelRetries)
	require.Equal(t, 300, policy.RetryDelayMs)
	require.Equal(t, 10, policy.StreamFirstEventTimeoutSeconds)
	require.Equal(t, 20, policy.NonStreamResponseTimeoutSeconds)
	require.Equal(t, biz.LoadBalancerStrategyAdaptive, policy.LoadBalancerStrategy)
	require.False(t, policy.AutoDisableChannel.Enabled)
	require.Equal(t, []biz.AutoDisableChannelStatus{
		{Status: http.StatusTooManyRequests, Times: 3},
	}, policy.AutoDisableChannel.Statuses)
	require.True(t, policy.EmptyResponseDetection)
	require.Equal(t, biz.UpstreamErrorModeHidden, policy.UpstreamErrorPolicy.Mode)
	require.Equal(t, "custom upstream failure", policy.UpstreamErrorPolicy.CustomMessage)
	require.Equal(t, 5000, policy.StreamProbeDurationMs)
}
