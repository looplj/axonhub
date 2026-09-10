package biz

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xcache/live"
	"github.com/looplj/axonhub/llm/httpclient"
)

func newTestChannelService(client *ent.Client) *ChannelService {
	mockSysSvc := &SystemService{
		AbstractService: &AbstractService{
			db: client,
		},
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	svc := &ChannelService{
		AbstractService: &AbstractService{
			db: client,
		},
		SystemService:             mockSysSvc,
		WebhookNotifier:           NewWebhookNotifier(mockSysSvc, httpclient.NewHttpClient()),
		channelPerfMetrics:        make(map[int]*channelMetrics),
		apiKeyErrorCounts:         make(map[int]map[string]map[int]int),
		apiKeyRuleActionsInFlight: make(map[int]map[string]bool),
		perfWindowSeconds:         600,
	}

	svc.enabledChannelsCache = live.NewCache(live.Options[[]*Channel]{
		Name:            "test_enabled_channels",
		InitialValue:    []*Channel{},
		RefreshInterval: time.Hour,
		RefreshFunc:     svc.reloadEnabledChannels,
		OnSwap:          svc.onEnabledChannelsSwap,
	})

	return svc
}

func createTestChannelWithAPIKeys(t *testing.T, client *ent.Client, ctx context.Context, name string, apiKeys []string) *ent.Channel {
	t.Helper()

	creds := objects.ChannelCredentials{
		APIKeys: apiKeys,
	}

	ch, err := client.Channel.Create().
		SetName(name).
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(creds).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

func withGlobalAutoDisable(t *testing.T, svc *ChannelService, ctx context.Context, ch *ent.Channel, rules []objects.APIKeyAutoDisableRule) {
	t.Helper()
	require.NoError(t, svc.SystemService.SetRetryPolicy(ctx, &RetryPolicy{
		AutoDisableChannel: AutoDisableChannel{Enabled: true, Rules: rules},
	}))
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})
}

func TestChannelService_GlobalAutoDisableAPIKey(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1", "key2", "key3"})
	withGlobalAutoDisable(t, svc, ctx, ch, []objects.APIKeyAutoDisableRule{{
		StatusCodes: []int{401},
		Times:       3,
		Action:      objects.APIKeyAutoDisableActionPermanent,
	}})

	perf := &PerformanceRecord{ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 401}

	_, acted := svc.evaluateAutoDisableForFailure(ctx, perf)
	require.False(t, acted)
	_, acted = svc.evaluateAutoDisableForFailure(ctx, perf)
	require.False(t, acted)
	_, acted = svc.evaluateAutoDisableForFailure(ctx, perf)
	require.True(t, acted)

	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updatedCh.DisabledAPIKeys, 1)
	require.Equal(t, "key1", updatedCh.DisabledAPIKeys[0].Key)
	require.Nil(t, updatedCh.DisabledAPIKeys[0].ExpiresAt)

	_, acted = svc.evaluateAutoDisableForFailure(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 500,
	})
	require.False(t, acted)

	_, acted = svc.evaluateAutoDisableForFailure(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key2", ResponseStatusCode: 401,
	})
	require.False(t, acted)
}

func TestChannelService_GlobalAutoDisableKeylessChannel(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel-no-keys", []string{})
	withGlobalAutoDisable(t, svc, ctx, ch, []objects.APIKeyAutoDisableRule{{
		StatusCodes: []int{401},
		Times:       2,
		Action:      objects.APIKeyAutoDisableActionPermanent,
	}})

	perf := &PerformanceRecord{ChannelID: ch.ID, ResponseStatusCode: 401}
	_, acted := svc.evaluateAutoDisableForFailure(ctx, perf)
	require.False(t, acted)

	_, acted = svc.evaluateAutoDisableForFailure(ctx, perf)
	require.True(t, acted)

	time.Sleep(100 * time.Millisecond)
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updatedCh.Status)
	require.NotNil(t, updatedCh.ErrorMessage)
	require.NotNil(t, updatedCh.AutoDisabledAt)
	require.Nil(t, updatedCh.AutoDisableExpiresAt)
}

func TestChannelService_markChannelUnavailable_RefreshesStaleLocalCacheWhenAlreadyDisabled(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	svc := newTestChannelService(client)
	defer svc.enabledChannelsCache.Stop()

	ch := createTestChannelWithAPIKeys(t, client, ctx, "stale-cache-channel", []string{"key1"})

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))
	require.NotNil(t, svc.GetEnabledChannel(ch.ID), "precondition: local cache should contain enabled channel")

	_, err := client.Channel.UpdateOneID(ch.ID).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage("disabled elsewhere").
		Save(ctx)
	require.NoError(t, err)

	svc.markChannelUnavailable(ctx, ch.ID, 401, 2, 2, nil, "")

	require.Nil(t, svc.GetEnabledChannel(ch.ID), "local cache should be refreshed even when DB row was already disabled")
}

func TestChannelService_DisableAllAPIKeysDisablesChannel(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	svc := newTestChannelService(client)

	// Create a channel with 2 API keys
	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel-2-keys", []string{"key1", "key2"})

	// Disable first key
	err := svc.DisableAPIKey(ctx, ch.ID, "key1", 401, "Test reason 1")
	require.NoError(t, err)

	// Verify channel is still enabled
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, updatedCh.Status)
	require.Len(t, updatedCh.DisabledAPIKeys, 1)

	// Disable second key - should disable the entire channel
	err = svc.DisableAPIKey(ctx, ch.ID, "key2", 401, "Test reason 2")
	require.NoError(t, err)

	// Verify channel is now disabled
	updatedCh, err = client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updatedCh.Status)
	require.Len(t, updatedCh.DisabledAPIKeys, 2)
	require.NotNil(t, updatedCh.ErrorMessage)
	require.NotNil(t, updatedCh.AutoDisabledAt)
	require.Nil(t, updatedCh.AutoDisableExpiresAt, "all-keys-unavailable must not set a channel expiry")
}

func TestChannelService_DisableAllAPIKeysNotifiesWebhook(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	type webhookRequest struct {
		body string
		err  error
	}
	notifications := make(chan webhookRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		notifications <- webhookRequest{body: string(body), err: err}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := WebhookNotifierConfig{
		Targets: []WebhookTarget{
			{
				Name:      "default",
				Enabled:   true,
				URL:       server.URL,
				TimeoutMs: 1000,
				Body:      `{"event":"{{.Event}}","channel_id":{{.Channel.ID}},"channel_name":"{{.Channel.Name}}","provider":"{{.Channel.Provider}}","base_url":"{{.Channel.BaseURL}}","status":"{{.Channel.Status}}","status_code":{{.Trigger.StatusCode}},"reason":"{{.Trigger.Reason}}"}`,
			},
		},
		Subscriptions: []WebhookSubscription{
			{Event: EventChannelAutoDisabled, TargetNames: []string{"default"}},
		},
	}

	svc := newTestChannelService(client)
	svc.SystemService = newTestSystemServiceWithWebhookConfig(t, client, cfg)
	svc.WebhookNotifier = NewWebhookNotifier(svc.SystemService, httpclient.NewHttpClient())

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel-webhook", []string{"key1", "key2"})

	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, "key1", 500, "first key failed"))

	select {
	case notification := <-notifications:
		t.Fatalf("received webhook before channel was disabled: %s", notification.body)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, "key2", 500, "second key failed"))

	select {
	case notification := <-notifications:
		require.NoError(t, notification.err)
		require.JSONEq(t, fmt.Sprintf(`{
			"event": "channel.auto_disabled",
			"channel_id": %d,
			"channel_name": "test-channel-webhook",
			"provider": "openai",
			"base_url": "https://api.openai.com",
			"status": "disabled",
			"status_code": 500,
			"reason": "All API keys disabled (last error: 500)"
		}`, ch.ID), notification.body)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel auto-disabled webhook")
	}
}

func TestChannelService_SuccessClearsErrorCounts(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1"})
	rule := objects.APIKeyAutoDisableRule{
		StatusCodes: []int{401},
		Times:       3,
		Action:      objects.APIKeyAutoDisableActionPermanent,
	}
	ruleKey := apiKeyRuleCounterKey("key1", autoDisableScopeGlobal, 0, rule)
	svc.apiKeyErrorCounts = map[int]map[string]map[int]int{
		ch.ID: {ruleKey: {0: 2}},
	}

	// Record a successful request
	perf := &PerformanceRecord{
		ChannelID:        ch.ID,
		APIKey:           "key1",
		Success:          true,
		RequestCompleted: true,
		EndTime:          time.Now(),
	}

	svc.IncrementChannelSelection(ch.ID)
	svc.RecordPerformance(ctx, perf)

	// Verify API key error counts are cleared
	svc.apiKeyErrorCountsLock.Lock()
	_, keyExists := svc.apiKeyErrorCounts[ch.ID][ruleKey]
	svc.apiKeyErrorCountsLock.Unlock()
	require.False(t, keyExists)
}

func TestChannelService_KeylessSuccessClearsOnlyKeylessCounters(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "mixed-identity", []string{"key1"})
	rule := objects.APIKeyAutoDisableRule{
		StatusCodes: []int{429},
		Times:       3,
		Action:      objects.APIKeyAutoDisableActionPermanent,
	}
	keyedKey := apiKeyRuleCounterKey("key1", autoDisableScopeGlobal, 0, rule)
	keylessKey := apiKeyRuleCounterKey(fmt.Sprintf("channel:%d", ch.ID), autoDisableScopeGlobal, 0, rule)
	svc.apiKeyErrorCounts = map[int]map[string]map[int]int{
		ch.ID: {
			keyedKey:   {0: 2},
			keylessKey: {0: 2},
		},
	}

	now := time.Now()
	svc.IncrementChannelSelection(ch.ID)
	svc.RecordPerformance(ctx, &PerformanceRecord{
		ChannelID: ch.ID, Success: true, RequestCompleted: true, StartTime: now, EndTime: now,
	})

	svc.apiKeyErrorCountsLock.Lock()
	defer svc.apiKeyErrorCountsLock.Unlock()
	require.NotContains(t, svc.apiKeyErrorCounts[ch.ID], keylessKey)
	require.Equal(t, 2, svc.apiKeyErrorCounts[ch.ID][keyedKey][0])
}

func TestChannelService_MultipleStatusCodes(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1", "key2"})
	withGlobalAutoDisable(t, svc, ctx, ch, []objects.APIKeyAutoDisableRule{
		{StatusCodes: []int{401}, Times: 2, Action: objects.APIKeyAutoDisableActionPermanent},
		{StatusCodes: []int{403}, Times: 1, Action: objects.APIKeyAutoDisableActionPermanent},
	})

	_, acted := svc.evaluateAutoDisableForFailure(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 401,
	})
	require.False(t, acted)
	_, acted = svc.evaluateAutoDisableForFailure(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 401,
	})
	require.True(t, acted)

	_, err := client.Channel.UpdateOneID(ch.ID).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{}).
		Save(ctx)
	require.NoError(t, err)
	svc.apiKeyErrorCounts = make(map[int]map[string]map[int]int)

	_, acted = svc.evaluateAutoDisableForFailure(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key2", ResponseStatusCode: 403,
	})
	require.True(t, acted)

	// Verify key2 is disabled
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updatedCh.DisabledAPIKeys, 1)
	require.Equal(t, "key2", updatedCh.DisabledAPIKeys[0].Key)
}

func TestChannelService_ConcurrentErrorTracking(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1", "key2", "key3"})
	withGlobalAutoDisable(t, svc, ctx, ch, []objects.APIKeyAutoDisableRule{{
		StatusCodes: []int{401},
		Times:       5,
		Action:      objects.APIKeyAutoDisableActionPermanent,
	}})

	// Simulate concurrent error reporting
	var wg sync.WaitGroup

	numGoroutines := 10

	for i := range numGoroutines {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			perf := &PerformanceRecord{
				ChannelID:          ch.ID,
				APIKey:             "key1",
				ResponseStatusCode: 401,
				Success:            false,
			}
			svc.evaluateAutoDisableForFailure(ctx, perf)
		}(i)
	}

	wg.Wait()

	// Verify counts are tracked correctly (should be at least 5 to trigger disable)
	// The key should be disabled since we had 10 errors and threshold is 5
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)

	// Should have disabled key1
	require.GreaterOrEqual(t, len(updatedCh.DisabledAPIKeys), 1)
}

func TestChannelService_DisableAPIKeyIdempotent(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1", "key2"})

	// Disable key1 first time
	err := svc.DisableAPIKey(ctx, ch.ID, "key1", 401, "Reason 1")
	require.NoError(t, err)

	// Disable key1 second time - should be idempotent
	err = svc.DisableAPIKey(ctx, ch.ID, "key1", 401, "Reason 2")
	require.NoError(t, err)

	// Verify only one entry
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updatedCh.DisabledAPIKeys, 1)
}

func TestChannelService_DisableAPIKeyNotFound(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1", "key2"})

	// Try to disable a key that doesn't exist - should be ignored
	err := svc.DisableAPIKey(ctx, ch.ID, "nonexistent-key", 401, "Reason")
	require.NoError(t, err)

	// Verify no keys are disabled
	updatedCh, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updatedCh.DisabledAPIKeys, 0)
}

func TestChannelService_DisableAPIKeyEmptyKey(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1"})

	// Try to disable an empty key - should return error
	err := svc.DisableAPIKey(ctx, ch.ID, "", 401, "Reason")
	require.Error(t, err)
}

func TestMatchesAPIKeyRule(t *testing.T) {
	tests := []struct {
		name string
		rule objects.APIKeyAutoDisableRule
		perf *PerformanceRecord
		want bool
	}{
		{
			name: "status code only",
			rule: objects.APIKeyAutoDisableRule{StatusCodes: []int{401}},
			perf: &PerformanceRecord{ResponseStatusCode: 401},
			want: true,
		},
		{
			name: "keyword only is case insensitive",
			rule: objects.APIKeyAutoDisableRule{KeywordPatterns: []string{"quota exceeded"}},
			perf: &PerformanceRecord{ResponseStatusCode: 503, ErrorMessage: "QUOTA EXCEEDED for this account"},
			want: true,
		},
		{
			name: "regular expression",
			rule: objects.APIKeyAutoDisableRule{KeywordPatterns: []string{`account (was )?disabled`}},
			perf: &PerformanceRecord{ResponseStatusCode: 400, ErrorMessage: "Account was disabled"},
			want: true,
		},
		{
			name: "invalid regular expression falls back to literal keyword",
			rule: objects.APIKeyAutoDisableRule{KeywordPatterns: []string{"quota["}},
			perf: &PerformanceRecord{ResponseStatusCode: 429, ErrorMessage: "Provider QUOTA[ exceeded"},
			want: true,
		},
		{
			name: "status and keyword must both match",
			rule: objects.APIKeyAutoDisableRule{StatusCodes: []int{401}, KeywordPatterns: []string{"invalid key"}},
			perf: &PerformanceRecord{ResponseStatusCode: 403, ErrorMessage: "invalid key"},
			want: false,
		},
		{
			name: "missing message does not match keyword",
			rule: objects.APIKeyAutoDisableRule{KeywordPatterns: []string{"quota"}},
			perf: &PerformanceRecord{ResponseStatusCode: 429},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, matchesAPIKeyRule(tt.rule, tt.perf))
		})
	}
}

func TestNormalizeAPIKeyAutoDisableRules(t *testing.T) {
	duration := 30
	policies := &objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
		{
			StatusCodes:            []int{429, 401, 429},
			KeywordPatterns:        []string{" quota ", "quota", ""},
			Times:                  2,
			Action:                 objects.APIKeyAutoDisableActionTemporary,
			DisableDurationMinutes: &duration,
		},
	}}

	require.NoError(t, NormalizeAPIKeyAutoDisableRules(policies))
	require.Equal(t, []int{401, 429}, policies.APIKeyAutoDisableRules[0].StatusCodes)
	require.Equal(t, []string{"quota"}, policies.APIKeyAutoDisableRules[0].KeywordPatterns)

	invalidDuration := 0
	tests := []objects.APIKeyAutoDisableRule{
		{Times: 0, Action: objects.APIKeyAutoDisableActionTemporary},
		{Times: 1, Action: objects.APIKeyAutoDisableActionTemporary},
		{StatusCodes: []int{99}, Times: 1, Action: objects.APIKeyAutoDisableActionTemporary},
		{Times: 1, Action: "unsupported"},
		{Times: 1, Action: objects.APIKeyAutoDisableActionTemporary, DisableDurationMinutes: &invalidDuration},
	}
	for _, rule := range tests {
		require.Error(t, NormalizeAPIKeyAutoDisableRules(&objects.ChannelPolicies{
			APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{rule},
		}))
	}
}

func TestChannelService_ChannelAPIKeyRuleTemporaryDisableAfterThreshold(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	duration := 30
	ch := createTestChannelWithAPIKeys(t, client, ctx, "temporary-rule", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes:            []int{429},
				Times:                  2,
				Action:                 objects.APIKeyAutoDisableActionTemporary,
				DisableDurationMinutes: &duration,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	perf := &PerformanceRecord{ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 429}
	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, perf)
	require.True(t, matched)
	require.False(t, acted)
	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, perf)
	require.True(t, matched)
	require.True(t, acted)

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updated.DisabledAPIKeys, 1)
	require.Equal(t, "key1", updated.DisabledAPIKeys[0].Key)
	require.NotNil(t, updated.DisabledAPIKeys[0].ExpiresAt)
	require.WithinDuration(t, time.Now().Add(30*time.Minute), *updated.DisabledAPIKeys[0].ExpiresAt, 5*time.Second)
}

func TestChannelService_ChannelAPIKeyRulePermanentActionKeepsLastKeyDisabled(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "permanent-rule", []string{"only-key"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes: []int{401},
				Times:       1,
				Action:      objects.APIKeyAutoDisableActionPermanentDelete,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "only-key", ResponseStatusCode: 401,
	})
	require.True(t, matched)
	require.True(t, acted)

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updated.Status)
	require.Equal(t, []string{"only-key"}, updated.Credentials.APIKeys)
	require.Len(t, updated.DisabledAPIKeys, 1)
	require.Nil(t, updated.DisabledAPIKeys[0].ExpiresAt)
}

func TestChannelService_ChannelAPIKeyRuleCountsAlternatingStatusesTogether(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	duration := 30
	ch := createTestChannelWithAPIKeys(t, client, ctx, "multi-status-rule", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes:            []int{401, 403},
				Times:                  2,
				Action:                 objects.APIKeyAutoDisableActionTemporary,
				DisableDurationMinutes: &duration,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 401,
	})
	require.True(t, matched)
	require.False(t, acted)

	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 403,
	})
	require.True(t, matched)
	require.True(t, acted)

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updated.DisabledAPIKeys, 1)
	require.Equal(t, "key1", updated.DisabledAPIKeys[0].Key)
}

func TestChannelService_ChannelAPIKeyRuleKeepsStreakAfterNonMatchingFailure(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	duration := 30
	ch := createTestChannelWithAPIKeys(t, client, ctx, "reset-non-match", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes:            []int{401},
				Times:                  2,
				Action:                 objects.APIKeyAutoDisableActionTemporary,
				DisableDurationMinutes: &duration,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 401,
	})
	require.True(t, matched)
	require.False(t, acted)

	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 500,
	})
	require.False(t, matched)
	require.False(t, acted)

	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 401,
	})
	require.True(t, matched)
	require.True(t, acted)
}

func TestChannelService_ChannelAPIKeyRuleKeepsIndependentCounters(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	duration := 30
	ch := createTestChannelWithAPIKeys(t, client, ctx, "reset-owned-failure", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes:            []int{401},
				Times:                  2,
				Action:                 objects.APIKeyAutoDisableActionTemporary,
				DisableDurationMinutes: &duration,
			},
			{
				StatusCodes:            []int{401, 403},
				Times:                  2,
				Action:                 objects.APIKeyAutoDisableActionTemporary,
				DisableDurationMinutes: &duration,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 403,
	})
	require.True(t, matched)
	require.False(t, acted)

	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 401,
	})
	require.True(t, matched)
	require.False(t, acted)

	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 403,
	})
	require.True(t, matched)
	require.True(t, acted)
}

func TestChannelService_ChannelAPIKeyRuleEditStartsNewStreak(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	duration := 30
	ch := createTestChannelWithAPIKeys(t, client, ctx, "edited-rule", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes:            []int{429},
				Times:                  2,
				Action:                 objects.APIKeyAutoDisableActionTemporary,
				DisableDurationMinutes: &duration,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	perf := &PerformanceRecord{ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 429}
	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, perf)
	require.True(t, matched)
	require.False(t, acted)

	editedDuration := 60
	ch, err = client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes:            []int{429},
				Times:                  2,
				Action:                 objects.APIKeyAutoDisableActionTemporary,
				DisableDurationMinutes: &editedDuration,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, perf)
	require.True(t, matched)
	require.False(t, acted)
}

func TestChannelService_ChannelAPIKeyRuleKeepsStreakWhenActionFails(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	duration := 30
	ch := createTestChannelWithAPIKeys(t, client, ctx, "failed-action", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes:            []int{429},
				Times:                  1,
				Action:                 objects.APIKeyAutoDisableActionTemporary,
				DisableDurationMinutes: &duration,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})
	require.NoError(t, client.Close())

	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 429,
	})
	require.True(t, matched)
	require.False(t, acted)

	svc.apiKeyErrorCountsLock.Lock()
	defer svc.apiKeyErrorCountsLock.Unlock()
	require.Len(t, svc.apiKeyErrorCounts[ch.ID], 1)
	for _, countsByStatus := range svc.apiKeyErrorCounts[ch.ID] {
		require.Equal(t, 1, countsByStatus[0])
	}
}

func TestChannelService_RemovedAPIKeyRuleDoesNotRestoreOldStreak(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	duration := 30
	rule := objects.APIKeyAutoDisableRule{
		StatusCodes:            []int{429},
		Times:                  2,
		Action:                 objects.APIKeyAutoDisableActionTemporary,
		DisableDurationMinutes: &duration,
	}
	ch := createTestChannelWithAPIKeys(t, client, ctx, "removed-rule", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{rule}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	perf := &PerformanceRecord{ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 429}
	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, perf)
	require.True(t, matched)
	require.False(t, acted)

	withoutRulesEntity := *ch
	withoutRulesEntity.Policies.APIKeyAutoDisableRules = nil
	withoutRules := buildChannel(&withoutRulesEntity, nil)
	svc.SetEnabledChannelsForTest([]*Channel{withoutRules})
	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, perf)
	require.False(t, matched)
	require.False(t, acted)

	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})
	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, perf)
	require.True(t, matched)
	require.True(t, acted)
}

func TestChannelService_ChannelAPIKeyRuleDoesNotStartConcurrentAction(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	duration := 30
	rule := objects.APIKeyAutoDisableRule{
		StatusCodes:            []int{429},
		Times:                  1,
		Action:                 objects.APIKeyAutoDisableActionTemporary,
		DisableDurationMinutes: &duration,
	}
	ch := createTestChannelWithAPIKeys(t, client, ctx, "concurrent-action", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{rule}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	ruleKey := apiKeyRuleCounterKey("key1", autoDisableScopeChannel, 0, rule)
	svc.apiKeyRuleActionsInFlight[ch.ID] = map[string]bool{ruleKey: false}

	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 429,
	})
	require.True(t, matched)
	require.False(t, acted)

	svc.apiKeyErrorCountsLock.Lock()
	require.Equal(t, 1, svc.apiKeyErrorCounts[ch.ID][ruleKey][0])
	streakReset, stillInFlight := svc.apiKeyRuleActionsInFlight[ch.ID][ruleKey]
	require.True(t, stillInFlight)
	require.False(t, streakReset)
	svc.apiKeyErrorCountsLock.Unlock()

	now := time.Now()
	svc.RecordPerformance(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", StartTime: now, EndTime: now,
		Success: true, RequestCompleted: true,
	})

	svc.apiKeyErrorCountsLock.Lock()
	defer svc.apiKeyErrorCountsLock.Unlock()
	require.NotContains(t, svc.apiKeyErrorCounts[ch.ID], ruleKey)
	streakReset, stillInFlight = svc.apiKeyRuleActionsInFlight[ch.ID][ruleKey]
	require.True(t, stillInFlight)
	require.True(t, streakReset)
}

func TestEffectiveAutoDisableMode(t *testing.T) {
	require.Equal(t, objects.APIKeyAutoDisableModeOff, objects.ChannelPolicies{APIKeyAutoDisableMode: objects.APIKeyAutoDisableModeOff}.EffectiveAutoDisableMode())
	require.Equal(t, objects.APIKeyAutoDisableModeOff, objects.ChannelPolicies{
		APIKeyAutoDisableMode:  objects.APIKeyAutoDisableModeOff,
		APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{Times: 1, Action: objects.APIKeyAutoDisableActionPermanent}},
	}.EffectiveAutoDisableMode())
	require.Equal(t, objects.APIKeyAutoDisableModeInherit, objects.ChannelPolicies{APIKeyAutoDisableMode: objects.APIKeyAutoDisableModeCustom}.EffectiveAutoDisableMode())
	require.Equal(t, objects.APIKeyAutoDisableModeCustom, objects.ChannelPolicies{
		APIKeyAutoDisableMode:  objects.APIKeyAutoDisableModeCustom,
		APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{Times: 1, Action: objects.APIKeyAutoDisableActionPermanent}},
	}.EffectiveAutoDisableMode())
	require.Equal(t, objects.APIKeyAutoDisableModeInherit, objects.ChannelPolicies{}.EffectiveAutoDisableMode())
	require.Equal(t, objects.APIKeyAutoDisableModeCustom, objects.ChannelPolicies{
		APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{Times: 1, Action: objects.APIKeyAutoDisableActionPermanent}},
	}.EffectiveAutoDisableMode())
}

func TestNormalizeAPIKeyAutoDisableRules_Mode(t *testing.T) {
	inherit := &objects.ChannelPolicies{
		APIKeyAutoDisableMode:  objects.APIKeyAutoDisableModeInherit,
		APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{Times: 1, Action: objects.APIKeyAutoDisableActionPermanent}},
	}
	require.NoError(t, NormalizeAPIKeyAutoDisableRules(inherit))
	require.Empty(t, inherit.APIKeyAutoDisableRules)

	emptyCustom := &objects.ChannelPolicies{APIKeyAutoDisableMode: objects.APIKeyAutoDisableModeCustom}
	require.NoError(t, NormalizeAPIKeyAutoDisableRules(emptyCustom))
	require.Equal(t, objects.APIKeyAutoDisableModeInherit, emptyCustom.APIKeyAutoDisableMode)

	off := &objects.ChannelPolicies{
		APIKeyAutoDisableMode:  objects.APIKeyAutoDisableModeOff,
		APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{Times: 1, Action: objects.APIKeyAutoDisableActionPermanent}},
	}
	require.NoError(t, NormalizeAPIKeyAutoDisableRules(off))
	require.Equal(t, objects.APIKeyAutoDisableModeOff, off.APIKeyAutoDisableMode)
	require.Len(t, off.APIKeyAutoDisableRules, 1)
}

func TestChannelService_InheritTemporaryDisable(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "inherit-temp", []string{"key1", "key2"})
	duration := 30
	withGlobalAutoDisable(t, svc, ctx, ch, []objects.APIKeyAutoDisableRule{{
		StatusCodes:            []int{429},
		Times:                  2,
		Action:                 objects.APIKeyAutoDisableActionTemporary,
		DisableDurationMinutes: &duration,
	}})

	now := time.Now()
	perf := &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 429,
		RequestCompleted: true, StartTime: now, EndTime: now,
	}
	svc.IncrementChannelSelection(ch.ID)
	svc.RecordPerformance(ctx, perf)
	svc.IncrementChannelSelection(ch.ID)
	svc.RecordPerformance(ctx, perf)

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updated.DisabledAPIKeys, 1)
	require.Equal(t, "key1", updated.DisabledAPIKeys[0].Key)
	require.NotNil(t, updated.DisabledAPIKeys[0].ExpiresAt)
	require.WithinDuration(t, time.Now().Add(30*time.Minute), *updated.DisabledAPIKeys[0].ExpiresAt, 5*time.Second)
	require.Nil(t, updated.AutoDisableExpiresAt)

	elapsed := time.Now().Add(-time.Minute)
	expired := updated.DisabledAPIKeys[0]
	expired.ExpiresAt = &elapsed
	_, err = client.Channel.UpdateOneID(ch.ID).SetDisabledAPIKeys([]objects.DisabledAPIKey{expired}).Save(ctx)
	require.NoError(t, err)
	svc.cleanupExpiredDisabledAPIKeys(ctx)

	recovered, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Empty(t, recovered.DisabledAPIKeys)
	require.Equal(t, channel.StatusEnabled, recovered.Status)
}

func TestChannelService_CustomAndGlobalCountersAreIndependent(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "overlay-counters", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{
			APIKeyAutoDisableMode: objects.APIKeyAutoDisableModeCustom,
			APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{
				KeywordPatterns:  []string{"quota"},
				Times:            2,
				Action:           objects.APIKeyAutoDisableActionUntilCron,
				DisableUntilCron: "0 0 * * *",
			}},
		}).
		Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.SystemService.SetRetryPolicy(ctx, &RetryPolicy{
		AutoDisableChannel: AutoDisableChannel{
			Enabled: true,
			Rules: []objects.APIKeyAutoDisableRule{{
				StatusCodes: []int{401},
				Times:       3,
				Action:      objects.APIKeyAutoDisableActionPermanent,
			}},
		},
	}))
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	quotaPerf := &PerformanceRecord{ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 429, ErrorMessage: "quota exceeded"}
	matched, acted := svc.evaluateAutoDisableForFailure(ctx, quotaPerf)
	require.True(t, matched)
	require.False(t, acted)

	authPerf := &PerformanceRecord{ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 401}
	matched, acted = svc.evaluateAutoDisableForFailure(ctx, authPerf)
	require.True(t, matched)
	require.False(t, acted)

	channelKey := apiKeyRuleCounterKey("key1", autoDisableScopeChannel, 0, ch.Policies.APIKeyAutoDisableRules[0])
	globalKey := apiKeyRuleCounterKey("key1", autoDisableScopeGlobal, 0, objects.APIKeyAutoDisableRule{
		StatusCodes: []int{401}, Times: 3, Action: objects.APIKeyAutoDisableActionPermanent,
	})
	svc.apiKeyErrorCountsLock.Lock()
	require.Equal(t, 1, svc.apiKeyErrorCounts[ch.ID][channelKey][0])
	require.Equal(t, 1, svc.apiKeyErrorCounts[ch.ID][globalKey][0])
	svc.apiKeyErrorCountsLock.Unlock()

	now := time.Now()
	svc.IncrementChannelSelection(ch.ID)
	svc.RecordPerformance(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", Success: true, RequestCompleted: true, StartTime: now, EndTime: now,
	})
	svc.apiKeyErrorCountsLock.Lock()
	defer svc.apiKeyErrorCountsLock.Unlock()
	require.NotContains(t, svc.apiKeyErrorCounts[ch.ID], channelKey)
	require.NotContains(t, svc.apiKeyErrorCounts[ch.ID], globalKey)
}

func TestChannelService_OffModeDoesNotCountOrDisable(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "off-mode", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{
			APIKeyAutoDisableMode: objects.APIKeyAutoDisableModeOff,
			APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{
				StatusCodes: []int{401},
				Times:       1,
				Action:      objects.APIKeyAutoDisableActionPermanent,
			}},
		}).
		Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.SystemService.SetRetryPolicy(ctx, &RetryPolicy{
		AutoDisableChannel: AutoDisableChannel{
			Enabled: true,
			Rules: []objects.APIKeyAutoDisableRule{{
				StatusCodes: []int{401},
				Times:       1,
				Action:      objects.APIKeyAutoDisableActionPermanent,
			}},
		},
	}))
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	matched, acted := svc.evaluateAutoDisableForFailure(ctx, &PerformanceRecord{
		ChannelID: ch.ID, APIKey: "key1", ResponseStatusCode: 401,
	})
	require.False(t, matched)
	require.False(t, acted)
	require.Empty(t, svc.apiKeyErrorCounts[ch.ID])

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Empty(t, updated.DisabledAPIKeys)
}

func TestChannelService_KeylessTemporaryDisableAndCleanup(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "keyless-temp", []string{})
	duration := 30
	withGlobalAutoDisable(t, svc, ctx, ch, []objects.APIKeyAutoDisableRule{{
		StatusCodes:            []int{429},
		Times:                  1,
		Action:                 objects.APIKeyAutoDisableActionTemporary,
		DisableDurationMinutes: &duration,
	}})

	now := time.Now()
	svc.IncrementChannelSelection(ch.ID)
	svc.RecordPerformance(ctx, &PerformanceRecord{
		ChannelID: ch.ID, ResponseStatusCode: 429, RequestCompleted: true, StartTime: now, EndTime: now,
	})

	disabled, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, disabled.Status)
	require.NotNil(t, disabled.AutoDisabledAt)
	require.NotNil(t, disabled.AutoDisableExpiresAt)
	require.WithinDuration(t, time.Now().Add(30*time.Minute), *disabled.AutoDisableExpiresAt, 5*time.Second)

	elapsed := time.Now().Add(-time.Minute)
	_, err = client.Channel.UpdateOneID(ch.ID).SetAutoDisableExpiresAt(elapsed).Save(ctx)
	require.NoError(t, err)
	svc.cleanupExpiredDisabledAPIKeys(ctx)

	recovered, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, recovered.Status)
	require.Nil(t, recovered.AutoDisabledAt)
	require.Nil(t, recovered.AutoDisableExpiresAt)
	require.Nil(t, recovered.ErrorMessage)

	manual := createTestChannelWithAPIKeys(t, client, ctx, "manual-disabled", []string{})
	_, err = client.Channel.UpdateOneID(manual.ID).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage("operator").
		SetAutoDisableExpiresAt(elapsed).
		Save(ctx)
	require.NoError(t, err)
	svc.cleanupExpiredDisabledAPIKeys(ctx)
	still, err := client.Channel.Get(ctx, manual.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, still.Status)
	require.Nil(t, still.AutoDisabledAt)
}
