package biz

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/channelmodelpriceversion"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache/live"
	"github.com/looplj/axonhub/llm/httpclient"
)

type channelSyncNotifierSpy struct {
	notifyCount int
	events      []live.CacheEvent[struct{}]
}

func (*channelSyncNotifierSpy) Watch() (<-chan live.CacheEvent[struct{}], func()) {
	return make(chan live.CacheEvent[struct{}]), func() {}
}

func (s *channelSyncNotifierSpy) Notify(_ context.Context, event live.CacheEvent[struct{}]) error {
	s.notifyCount++
	s.events = append(s.events, event)

	return nil
}

func TestChannelService_SyncChannelModelsAutoConfiguresMissingPrices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"existing-model"},{"id":"priced-model"},{"id":"no-cost-model"},{"id":"archived-model"},{"id":"missing-model"}]}`))
	}))
	defer server.Close()

	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc.httpClient = httpclient.NewHttpClientWithClient(server.Client())
	notifier := &channelSyncNotifierSpy{}
	svc.channelNotifier = notifier
	previousAsyncReloadDisabled := asyncReloadDisabled
	asyncReloadDisabled = false
	t.Cleanup(func() {
		asyncReloadDisabled = previousAsyncReloadDisabled
	})

	createModelLibraryEntry(t, ctx, client, "existing-model", model.StatusEnabled, objects.ModelCardCost{Input: 99})
	createModelLibraryEntry(t, ctx, client, "priced-model", model.StatusDisabled, objects.ModelCardCost{
		Input:      1.25,
		Output:     5,
		CacheRead:  0.25,
		CacheWrite: 1.5,
	})
	createModelLibraryEntry(t, ctx, client, "no-cost-model", model.StatusEnabled, objects.ModelCardCost{})
	createModelLibraryEntry(t, ctx, client, "archived-model", model.StatusArchived, objects.ModelCardCost{Input: 2})

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Sync auto price").
		SetBaseURL(server.URL).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"existing-model"}).
		SetDefaultTestModel("existing-model").
		Save(ctx)
	require.NoError(t, err)

	customPrice := objects.ModelPrice{Items: []objects.ModelPriceItem{{
		ItemCode: objects.PriceItemCodeUsage,
		Pricing: objects.Pricing{
			Mode:         objects.PricingModeUsagePerUnit,
			UsagePerUnit: loToDecimalPtr("7.5"),
		},
	}}}
	existingPrices, err := svc.SaveChannelModelPrices(ctx, ch.ID, []SaveChannelModelPriceInput{{
		ModelID: "existing-model",
		Price:   customPrice,
	}})
	require.NoError(t, err)
	require.Len(t, existingPrices, 1)
	existingReferenceID := existingPrices[0].ReferenceID

	updated, err := svc.SyncChannelModels(ctx, ch.ID, nil)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"existing-model",
		"priced-model",
		"no-cost-model",
		"archived-model",
		"missing-model",
	}, updated.SupportedModels)
	require.Equal(t, 1, notifier.notifyCount)
	require.Equal(t, live.EventForceRefresh, notifier.events[0].Type)
	returnedPrices, err := updated.QueryChannelModelPrices().All(ctx)
	require.NoError(t, err)
	require.Len(t, returnedPrices, 2)

	prices, err := client.ChannelModelPrice.Query().
		Where(channelmodelprice.ChannelID(ch.ID)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, prices, 2)

	pricesByModel := lo.KeyBy(prices, func(price *ent.ChannelModelPrice) string {
		return price.ModelID
	})
	require.Equal(t, customPrice, pricesByModel["existing-model"].Price)
	require.Equal(t, existingReferenceID, pricesByModel["existing-model"].ReferenceID)

	createdPrice := pricesByModel["priced-model"]
	require.NotNil(t, createdPrice)
	require.Equal(t, []objects.PriceItemCode{
		objects.PriceItemCodeUsage,
		objects.PriceItemCodeCompletion,
		objects.PriceItemCodePromptCachedToken,
		objects.PriceItemCodeWriteCachedTokens,
	}, lo.Map(createdPrice.Price.Items, func(item objects.ModelPriceItem, _ int) objects.PriceItemCode {
		return item.ItemCode
	}))

	createdVersion, err := client.ChannelModelPriceVersion.Query().
		Where(channelmodelpriceversion.ChannelModelPriceID(createdPrice.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, channelmodelpriceversion.StatusActive, createdVersion.Status)
	require.Equal(t, createdPrice.ReferenceID, createdVersion.ReferenceID)
	require.Equal(t, createdPrice.Price, createdVersion.Price)

	for _, modelID := range []string{"no-cost-model", "archived-model", "missing-model"} {
		exists, err := client.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(ch.ID),
				channelmodelprice.ModelID(modelID),
			).
			Exist(ctx)
		require.NoError(t, err)
		require.False(t, exists, "unexpected automatic price for %s", modelID)
	}

	noCostModel, err := client.Model.Query().Where(model.ModelID("no-cost-model")).Only(ctx)
	require.NoError(t, err)
	_, err = client.Model.UpdateOne(noCostModel).
		SetModelCard(&objects.ModelCard{Cost: objects.ModelCardCost{Input: 3}}).
		Save(ctx)
	require.NoError(t, err)

	updated, err = svc.SyncChannelModels(ctx, ch.ID, nil)
	require.NoError(t, err)
	require.Equal(t, 2, notifier.notifyCount)
	retriedPrice, err := client.ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(ch.ID),
			channelmodelprice.ModelID("no-cost-model"),
		).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "3", retriedPrice.Price.Items[0].Pricing.UsagePerUnit.String())
	retriedVersions, err := retriedPrice.QueryVersions().All(ctx)
	require.NoError(t, err)
	require.Len(t, retriedVersions, 1)
	require.Equal(t, retriedPrice.ReferenceID, retriedVersions[0].ReferenceID)
}

func TestChannelService_ModelSyncIgnoresProviderOnlyOrderChanges(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"data":[{"id":"model-a"},{"id":"model-b"}]}`))

			return
		}

		_, _ = w.Write([]byte(`{"data":[{"id":"model-b"},{"id":"model-a"}]}`))
	}))
	defer server.Close()

	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc.httpClient = httpclient.NewHttpClientWithClient(server.Client())
	notifier := &channelSyncNotifierSpy{}
	svc.channelNotifier = notifier
	previousAsyncReloadDisabled := asyncReloadDisabled
	asyncReloadDisabled = false
	t.Cleanup(func() {
		asyncReloadDisabled = previousAsyncReloadDisabled
	})

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Order-stable sync").
		SetBaseURL(server.URL).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"old-model"}).
		SetDefaultTestModel("old-model").
		Save(ctx)
	require.NoError(t, err)

	first, err := svc.SyncChannelModels(ctx, ch.ID, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"model-a", "model-b"}, first.SupportedModels)
	require.Equal(t, 1, notifier.notifyCount)

	second, err := svc.SyncChannelModels(ctx, ch.ID, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"model-a", "model-b"}, second.SupportedModels)
	require.Equal(t, 1, notifier.notifyCount)
}

func TestChannelService_PeriodicModelSyncNotifiesOnceForChangedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"batch-model"}]}`))
	}))
	defer server.Close()

	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc.httpClient = httpclient.NewHttpClientWithClient(server.Client())
	notifier := &channelSyncNotifierSpy{}
	svc.channelNotifier = notifier
	previousAsyncReloadDisabled := asyncReloadDisabled
	asyncReloadDisabled = false
	t.Cleanup(func() {
		asyncReloadDisabled = previousAsyncReloadDisabled
	})

	createModelLibraryEntry(t, ctx, client, "batch-model", model.StatusEnabled, objects.ModelCardCost{Input: 1})
	for i := 1; i <= 2; i++ {
		_, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName(fmt.Sprintf("Periodic sync %d", i)).
			SetBaseURL(server.URL).
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"old-model"}).
			SetDefaultTestModel("old-model").
			SetAutoSyncSupportedModels(true).
			SetStatus(channel.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)
	}

	svc.syncChannelModels(ctx)

	require.Equal(t, 1, notifier.notifyCount)
	require.Equal(t, live.EventForceRefresh, notifier.events[0].Type)
	priceCount, err := client.ChannelModelPrice.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, priceCount)
}

func TestPreserveManualModels(t *testing.T) {
	tests := []struct {
		name          string
		manualModels  []string
		fetchedModels []string
		expected      []string
	}{
		{
			name:          "manual models preserved when fetched is different",
			manualModels:  []string{"custom-model-1", "custom-model-2"},
			fetchedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			expected:      []string{"custom-model-1", "custom-model-2", "gpt-4", "gpt-3.5-turbo"},
		},
		{
			name:          "manual models preserved when no overlap",
			manualModels:  []string{"my-custom-model"},
			fetchedModels: []string{"claude-3-opus"},
			expected:      []string{"my-custom-model", "claude-3-opus"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeModelsForTest(tt.manualModels, tt.fetchedModels)

			for _, manualModel := range tt.manualModels {
				assert.Contains(t, result, manualModel,
					"Manual model %s should be preserved after sync", manualModel)
			}
		})
	}
}

func TestMergeManualAndFetched(t *testing.T) {
	tests := []struct {
		name          string
		manualModels  []string
		fetchedModels []string
		expected      []string
	}{
		{
			name:          "union of manual and fetched models",
			manualModels:  []string{"manual-model-a", "manual-model-b"},
			fetchedModels: []string{"fetched-model-x", "fetched-model-y"},
			expected:      []string{"manual-model-a", "manual-model-b", "fetched-model-x", "fetched-model-y"},
		},
		{
			name:          "empty manual models only fetched",
			manualModels:  []string{},
			fetchedModels: []string{"gpt-4", "claude-3"},
			expected:      []string{"gpt-4", "claude-3"},
		},
		{
			name:          "both lists have some models",
			manualModels:  []string{"model-1", "model-2"},
			fetchedModels: []string{"model-3", "model-4", "model-5"},
			expected:      []string{"model-1", "model-2", "model-3", "model-4", "model-5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeModelsForTest(tt.manualModels, tt.fetchedModels)

			require.ElementsMatch(t, tt.expected, result,
				"Merged result should contain union of manual and fetched models")
		})
	}
}

func TestEmptyProviderResponse(t *testing.T) {
	tests := []struct {
		name          string
		manualModels  []string
		fetchedModels []string
		expected      []string
	}{
		{
			name:          "manual models remain when provider returns empty",
			manualModels:  []string{"important-custom-model", "another-manual-model"},
			fetchedModels: []string{},
			expected:      []string{"important-custom-model", "another-manual-model"},
		},
		{
			name:          "no models when both are empty",
			manualModels:  []string{},
			fetchedModels: []string{},
			expected:      []string{},
		},
		{
			name:          "nil fetched models treated as empty",
			manualModels:  []string{"preserved-model"},
			fetchedModels: nil,
			expected:      []string{"preserved-model"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeModelsForTest(tt.manualModels, tt.fetchedModels)

			require.ElementsMatch(t, tt.expected, result,
				"Manual models should remain when provider returns empty response")
		})
	}
}

func TestDuplicateModels(t *testing.T) {
	tests := []struct {
		name          string
		manualModels  []string
		fetchedModels []string
		expected      []string
	}{
		{
			name:          "duplicates between manual and fetched are removed",
			manualModels:  []string{"gpt-4", "custom-model"},
			fetchedModels: []string{"gpt-4", "claude-3"},
			expected:      []string{"gpt-4", "custom-model", "claude-3"},
		},
		{
			name:          "duplicates within manual models are removed",
			manualModels:  []string{"model-a", "model-a", "model-b"},
			fetchedModels: []string{"model-c"},
			expected:      []string{"model-a", "model-b", "model-c"},
		},
		{
			name:          "duplicates within fetched models are removed",
			manualModels:  []string{"manual-model"},
			fetchedModels: []string{"fetched-a", "fetched-a", "fetched-b"},
			expected:      []string{"manual-model", "fetched-a", "fetched-b"},
		},
		{
			name:          "all unique no duplicates",
			manualModels:  []string{"model-1", "model-2"},
			fetchedModels: []string{"model-3", "model-4"},
			expected:      []string{"model-1", "model-2", "model-3", "model-4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeModelsForTest(tt.manualModels, tt.fetchedModels)

			uniqueResult := lo.Uniq(result)
			require.Equal(t, len(uniqueResult), len(result),
				"Result should not contain duplicates")

			require.ElementsMatch(t, tt.expected, result,
				"Result should contain deduplicated union of models")
		})
	}
}

func TestCaseSensitivity(t *testing.T) {
	tests := []struct {
		name          string
		manualModels  []string
		fetchedModels []string
		expected      []string
	}{
		{
			name:          "case sensitivity preserved - GPT-4 vs gpt-4",
			manualModels:  []string{"GPT-4"},
			fetchedModels: []string{"gpt-4", "GPT-4"},
			expected:      []string{"GPT-4", "gpt-4"},
		},
		{
			name:          "different cases are different models",
			manualModels:  []string{"Claude-3", "claude-3"},
			fetchedModels: []string{"CLAUDE-3"},
			expected:      []string{"Claude-3", "claude-3", "CLAUDE-3"},
		},
		{
			name:          "mixed case models preserved",
			manualModels:  []string{"MyCustomModel"},
			fetchedModels: []string{"mycustommodel", "MYCUSTOMMODEL"},
			expected:      []string{"MyCustomModel", "mycustommodel", "MYCUSTOMMODEL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeModelsForTest(tt.manualModels, tt.fetchedModels)

			require.ElementsMatch(t, tt.expected, result,
				"Model IDs should be treated as case-sensitive")

			for _, expectedModel := range tt.expected {
				assert.Contains(t, result, expectedModel,
					"Model %s should be present with exact case", expectedModel)
			}
		})
	}
}

func mergeModelsForTest(manualModels, fetchedModels []string) []string {
	return lo.Uniq(append(manualModels, fetchedModels...))
}
