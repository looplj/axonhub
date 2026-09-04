package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm/httpclient"
)

type zenmuxCountingQuotaChecker struct {
	calls atomic.Int32
	err   error
}

func (c *zenmuxCountingQuotaChecker) CheckQuota(context.Context, *ent.Channel) (provider_quota.QuotaData, error) {
	c.calls.Add(1)
	if c.err != nil {
		return provider_quota.QuotaData{}, c.err
	}
	return provider_quota.QuotaData{
		Status:       "available",
		ProviderType: "zenmux",
		Ready:        true,
	}, nil
}

func (c *zenmuxCountingQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeZenmux ||
		ch.Type == channel.TypeZenmuxResponses ||
		ch.Type == channel.TypeZenmuxAnthropic ||
		ch.Type == channel.TypeZenmuxGemini
}

func TestQuotaAccountKey_HashesOnlyZenMuxManagementKeys(t *testing.T) {
	sum := sha256.Sum256([]byte("zenmux:management-key"))
	want := hex.EncodeToString(sum[:])[:16]

	got := quotaAccountKey("zenmux", &ent.Channel{
		Credentials: objects.ChannelCredentials{ManagementAPIKey: " management-key "},
	})

	require.Equal(t, want, got)
	require.Len(t, got, 16)
	require.NotContains(t, got, "management-key")
	require.Empty(t, quotaAccountKey("zenmux", &ent.Channel{}))
	require.Empty(t, quotaAccountKey("codex", &ent.Channel{
		Credentials: objects.ChannelCredentials{ManagementAPIKey: "management-key"},
	}))
}

func TestProviderQuotaService_GroupChannelsByQuotaAccount_DeduplicatesSharedZenMuxKey(t *testing.T) {
	service := &ProviderQuotaService{}
	channels := []*ent.Channel{
		{ID: 1, Type: channel.TypeZenmux, Credentials: objects.ChannelCredentials{ManagementAPIKey: "shared"}},
		{ID: 2, Type: channel.TypeZenmuxAnthropic, Credentials: objects.ChannelCredentials{ManagementAPIKey: "shared"}},
		{ID: 3, Type: channel.TypeZenmuxGemini, Credentials: objects.ChannelCredentials{ManagementAPIKey: "other"}},
		{ID: 4, Type: channel.TypeMinimax, Credentials: objects.ChannelCredentials{APIKey: "key"}},
	}

	groups := service.groupChannelsByQuotaAccount(channels)

	require.Len(t, groups, 3)
	require.Equal(t, []int{1, 2}, []int{groups[0].channels[0].ID, groups[0].channels[1].ID})
	require.NotEmpty(t, groups[0].accountKey)
	require.Equal(t, 3, groups[1].channels[0].ID)
	require.NotEmpty(t, groups[1].accountKey)
	require.Equal(t, 4, groups[2].channels[0].ID)
	require.Empty(t, groups[2].accountKey)
}

func TestProviderQuotaService_GroupChannelsByQuotaAccount_SeparatesProxyConfigurations(t *testing.T) {
	service := &ProviderQuotaService{}
	channels := []*ent.Channel{
		{
			ID:          1,
			Type:        channel.TypeZenmux,
			Credentials: objects.ChannelCredentials{ManagementAPIKey: "shared"},
			Settings:    &objects.ChannelSettings{Proxy: &httpclient.ProxyConfig{Type: httpclient.ProxyTypeURL, URL: "http://proxy-a.example"}},
		},
		{
			ID:          2,
			Type:        channel.TypeZenmuxAnthropic,
			Credentials: objects.ChannelCredentials{ManagementAPIKey: "shared"},
			Settings:    &objects.ChannelSettings{Proxy: &httpclient.ProxyConfig{Type: httpclient.ProxyTypeURL, URL: "http://proxy-b.example"}},
		},
	}

	groups := service.groupChannelsByQuotaAccount(channels)

	require.Len(t, groups, 2)
	require.Equal(t, groups[0].accountKey, groups[1].accountKey)
}

func TestQuotaCheckGroupIsDue_WhenOneSharedChannelIsDue(t *testing.T) {
	now := time.Now()
	group := quotaCheckGroup{
		channels: []*ent.Channel{
			{Edges: ent.ChannelEdges{ProviderQuotaStatus: &ent.ProviderQuotaStatus{NextCheckAt: now.Add(time.Minute)}}},
			{Edges: ent.ChannelEdges{ProviderQuotaStatus: &ent.ProviderQuotaStatus{NextCheckAt: now.Add(-time.Minute)}}},
		},
	}

	require.True(t, quotaCheckGroupIsDue(group, now))
}

func TestProviderQuotaService_RunQuotaCheck_DeduplicatesZenMuxAccountAndFansOutStatus(t *testing.T) {
	service, _, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()
	service.checkInterval = time.Minute
	checker := &zenmuxCountingQuotaChecker{}
	service.checkers["zenmux"] = checker

	fixtures := []struct {
		name string
		typ  channel.Type
		key  string
	}{
		{name: "shared openai", typ: channel.TypeZenmux, key: "shared-key"},
		{name: "shared anthropic", typ: channel.TypeZenmuxAnthropic, key: "shared-key"},
		{name: "other gemini", typ: channel.TypeZenmuxGemini, key: "other-key"},
	}
	for _, fixture := range fixtures {
		client.Channel.Create().
			SetName(fixture.name).
			SetType(fixture.typ).
			SetStatus(channel.StatusEnabled).
			SetCredentials(objects.ChannelCredentials{APIKey: "inference-key", ManagementAPIKey: fixture.key}).
			SetSupportedModels([]string{"test-model"}).
			SetDefaultTestModel("test-model").
			SaveX(ctx)
	}

	service.runQuotaCheck(ctx, true)

	require.EqualValues(t, 2, checker.calls.Load())
	statuses := client.ProviderQuotaStatus.Query().
		Order(ent.Asc(providerquotastatus.FieldChannelID)).
		AllX(ctx)
	require.Len(t, statuses, 3)
	require.NotEmpty(t, statuses[0].AccountKey)
	require.Equal(t, statuses[0].AccountKey, statuses[1].AccountKey)
	require.True(t, statuses[0].NextCheckAt.Equal(statuses[1].NextCheckAt))
	require.NotEqual(t, statuses[0].AccountKey, statuses[2].AccountKey)
}

func TestProviderQuotaService_RunQuotaCheck_FansOutZenMuxErrorOnOneSchedule(t *testing.T) {
	service, _, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()
	service.checkInterval = time.Minute
	checker := &zenmuxCountingQuotaChecker{err: errors.New("quota unavailable")}
	service.checkers["zenmux"] = checker

	channels := make([]*ent.Channel, 0, 2)
	for i, channelType := range []channel.Type{channel.TypeZenmux, channel.TypeZenmuxResponses} {
		channels = append(channels, client.Channel.Create().
			SetName([]string{"openai", "responses"}[i]).
			SetType(channelType).
			SetStatus(channel.StatusEnabled).
			SetCredentials(objects.ChannelCredentials{APIKey: "inference-key", ManagementAPIKey: "shared-key"}).
			SetSupportedModels([]string{"test-model"}).
			SetDefaultTestModel("test-model").
			SaveX(ctx))
	}

	for i, ch := range channels {
		client.ProviderQuotaStatus.Create().
			SetChannelID(ch.ID).
			SetProviderType(providerquotastatus.ProviderTypeZenmux).
			SetStatus(providerquotastatus.StatusUnknown).
			SetReady(false).
			SetQuotaData(map[string]any{"error_count": i + 1}).
			SetNextCheckAt(time.Now().Add(-time.Minute)).
			SaveX(ctx)
	}

	service.runQuotaCheck(ctx, true)

	require.EqualValues(t, 1, checker.calls.Load())
	statuses := client.ProviderQuotaStatus.Query().
		Order(ent.Asc(providerquotastatus.FieldChannelID)).
		AllX(ctx)
	require.Len(t, statuses, 2)
	require.Equal(t, statuses[0].AccountKey, statuses[1].AccountKey)
	require.NotEmpty(t, statuses[0].AccountKey)
	require.True(t, statuses[0].NextCheckAt.Equal(statuses[1].NextCheckAt))
	require.EqualValues(t, 3, statuses[0].QuotaData["error_count"])
	require.EqualValues(t, 3, statuses[1].QuotaData["error_count"])
}

func TestProviderQuotaService_ZenMuxTypesRequireManagementKey(t *testing.T) {
	service := &ProviderQuotaService{}
	for _, channelType := range []channel.Type{
		channel.TypeZenmux,
		channel.TypeZenmuxResponses,
		channel.TypeZenmuxAnthropic,
		channel.TypeZenmuxGemini,
	} {
		ch := &ent.Channel{
			Type:        channelType,
			Credentials: objects.ChannelCredentials{APIKey: "inference-key", ManagementAPIKey: " management-key "},
		}
		require.Equal(t, "zenmux", service.getProviderType(ch))
		require.True(t, hasCredentialsForProvider(ch))

		ch.Credentials.ManagementAPIKey = ""
		require.False(t, hasCredentialsForProvider(ch))
	}
}
