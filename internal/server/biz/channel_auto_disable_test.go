package biz

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func newTestChannelService(client *ent.Client) *ChannelService {
	mockSysSvc := &SystemService{
		AbstractService: &AbstractService{
			db: client,
		},
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	return &ChannelService{
		AbstractService: &AbstractService{
			db: client,
		},
		SystemService:      mockSysSvc,
		channelPerfMetrics: make(map[int]*channelMetrics),
		channelErrorCounts: make(map[int]map[int]int),
		apiKeyErrorCounts:  make(map[int]map[string]map[int]int),
		perfWindowSeconds:  600,
	}
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

func TestChannelService_checkAndHandleAPIKeyError(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	svc := newTestChannelService(client)

	// Create a channel with multiple API keys
	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1", "key2", "key3"})

	tests := []struct {
		name             string
		policy           *RetryPolicy
		perf             *PerformanceRecord
		expectedDisabled bool
		setupFunc        func()
	}{
		{
			name: "first error - should not disable",
			policy: &RetryPolicy{
				AutoDisableChannel: AutoDisableChannel{
					Enabled: true,
					Statuses: []AutoDisableChannelStatus{
						{Status: 401, Times: 3},
					},
				},
			},
			perf: &PerformanceRecord{
				ChannelID:       ch.ID,
				APIKey:          "key1",
				ErrorStatusCode: 401,
				Success:         false,
			},
			expectedDisabled: false,
			setupFunc: func() {
				svc.apiKeyErrorCounts = make(map[int]map[string]map[int]int)
			},
		},
		{
			name: "second error - should not disable",
			policy: &RetryPolicy{
				AutoDisableChannel: AutoDisableChannel{
					Enabled: true,
					Statuses: []AutoDisableChannelStatus{
						{Status: 401, Times: 3},
					},
				},
			},
			perf: &PerformanceRecord{
				ChannelID:       ch.ID,
				APIKey:          "key1",
				ErrorStatusCode: 401,
				Success:         false,
			},
			expectedDisabled: false,
			setupFunc: func() {
				svc.apiKeyErrorCounts = map[int]map[string]map[int]int{
					ch.ID: {"key1": {401: 1}},
				}
			},
		},
		{
			name: "third error - should disable API key",
			policy: &RetryPolicy{
				AutoDisableChannel: AutoDisableChannel{
					Enabled: true,
					Statuses: []AutoDisableChannelStatus{
						{Status: 401, Times: 3},
					},
				},
			},
			perf: &PerformanceRecord{
				ChannelID:       ch.ID,
				APIKey:          "key1",
				ErrorStatusCode: 401,
				Success:         false,
			},
			expectedDisabled: true,
			setupFunc: func() {
				// Reset channel state first
				_, err := client.Channel.UpdateOneID(ch.ID).
					SetDisabledAPIKeys([]objects.DisabledAPIKey{}).
					SetStatus(channel.StatusEnabled).
					ClearErrorMessage().
					Save(ctx)
				require.NoError(t, err)

				svc.apiKeyErrorCounts = map[int]map[string]map[int]int{
					ch.ID: {"key1": {401: 2}},
				}
			},
		},
		{
			name: "different status code - should not disable",
			policy: &RetryPolicy{
				AutoDisableChannel: AutoDisableChannel{
					Enabled: true,
					Statuses: []AutoDisableChannelStatus{
						{Status: 401, Times: 3},
					},
				},
			},
			perf: &PerformanceRecord{
				ChannelID:       ch.ID,
				APIKey:          "key1",
				ErrorStatusCode: 500,
				Success:         false,
			},
			expectedDisabled: false,
			setupFunc: func() {
				svc.apiKeyErrorCounts = map[int]map[string]map[int]int{
					ch.ID: {"key1": {401: 2}},
				}
			},
		},
		{
			name: "different API key - should not disable",
			policy: &RetryPolicy{
				AutoDisableChannel: AutoDisableChannel{
					Enabled: true,
					Statuses: []AutoDisableChannelStatus{
						{Status: 401, Times: 3},
					},
				},
			},
			perf: &PerformanceRecord{
				ChannelID:       ch.ID,
				APIKey:          "key2",
				ErrorStatusCode: 401,
				Success:         false,
			},
			expectedDisabled: false,
			setupFunc: func() {
				svc.apiKeyErrorCounts = map[int]map[string]map[int]int{
					ch.ID: {"key1": {401: 2}},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			result := svc.checkAndHandleAPIKeyError(ctx, tt.perf, tt.policy)
			require.Equal(t, tt.expectedDisabled, result)

			if tt.expectedDisabled {
				// Verify API key is disabled
				updatedCh, err := client.Channel.Get(ctx, ch.ID)
				require.NoError(t, err)
				require.Len(t, updatedCh.DisabledAPIKeys, 1)
				require.Equal(t, tt.perf.APIKey, updatedCh.DisabledAPIKeys[0].Key)

				// Verify error counts are cleared for this API key
				svc.apiKeyErrorCountsLock.Lock()
				_, exists := svc.apiKeyErrorCounts[ch.ID][tt.perf.APIKey]
				svc.apiKeyErrorCountsLock.Unlock()
				require.False(t, exists)
			}
		})
	}
}

func TestChannelService_checkAndHandleChannelError(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	svc := newTestChannelService(client)

	// Create a channel without API keys (single key scenario)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel-no-keys", []string{})

	tests := []struct {
		name             string
		policy           *RetryPolicy
		perf             *PerformanceRecord
		expectedDisabled bool
		setupFunc        func()
	}{
		{
			name: "first error - should not disable",
			policy: &RetryPolicy{
				AutoDisableChannel: AutoDisableChannel{
					Enabled: true,
					Statuses: []AutoDisableChannelStatus{
						{Status: 401, Times: 2},
					},
				},
			},
			perf: &PerformanceRecord{
				ChannelID:       ch.ID,
				ErrorStatusCode: 401,
				Success:         false,
			},
			expectedDisabled: false,
			setupFunc: func() {
				svc.channelErrorCounts = make(map[int]map[int]int)
				// Reset channel status
				_, err := client.Channel.UpdateOneID(ch.ID).
					SetStatus(channel.StatusEnabled).
					ClearErrorMessage().
					Save(ctx)
				require.NoError(t, err)
			},
		},
		{
			name: "second error - should disable channel",
			policy: &RetryPolicy{
				AutoDisableChannel: AutoDisableChannel{
					Enabled: true,
					Statuses: []AutoDisableChannelStatus{
						{Status: 401, Times: 2},
					},
				},
			},
			perf: &PerformanceRecord{
				ChannelID:       ch.ID,
				ErrorStatusCode: 401,
				Success:         false,
			},
			expectedDisabled: true,
			setupFunc: func() {
				// Reset channel status
				_, err := client.Channel.UpdateOneID(ch.ID).
					SetStatus(channel.StatusEnabled).
					ClearErrorMessage().
					Save(ctx)
				require.NoError(t, err)

				svc.channelErrorCounts = map[int]map[int]int{
					ch.ID: {401: 1},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			result := svc.checkAndHandleChannelError(ctx, tt.perf, tt.policy)
			require.Equal(t, tt.expectedDisabled, result)

			if tt.expectedDisabled {
				// Give goroutine time to complete (markChannelUnavailable uses xcontext.DetachWithTimeout)
				time.Sleep(100 * time.Millisecond)

				// Verify channel is disabled
				updatedCh, err := client.Channel.Get(ctx, ch.ID)
				require.NoError(t, err)
				require.Equal(t, channel.StatusDisabled, updatedCh.Status)
				require.NotNil(t, updatedCh.ErrorMessage)

				// Verify error counts are cleared
				svc.channelErrorCountsLock.Lock()
				_, exists := svc.channelErrorCounts[ch.ID]
				svc.channelErrorCountsLock.Unlock()
				require.False(t, exists)
			}
		})
	}
}

func TestChannelService_DisableAllAPIKeysDisablesChannel(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
}

func TestChannelService_SuccessClearsErrorCounts(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1"})

	// Set up some error counts
	svc.channelErrorCounts = map[int]map[int]int{
		ch.ID: {401: 2, 500: 1},
	}
	svc.apiKeyErrorCounts = map[int]map[string]map[int]int{
		ch.ID: {"key1": {401: 2}},
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

	// Verify channel error counts are cleared
	svc.channelErrorCountsLock.Lock()
	_, channelExists := svc.channelErrorCounts[ch.ID]
	svc.channelErrorCountsLock.Unlock()
	require.False(t, channelExists)

	// Verify API key error counts are cleared
	svc.apiKeyErrorCountsLock.Lock()
	_, keyExists := svc.apiKeyErrorCounts[ch.ID]["key1"]
	svc.apiKeyErrorCountsLock.Unlock()
	require.False(t, keyExists)
}

func TestChannelService_MultipleStatusCodes(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1", "key2"})

	policy := &RetryPolicy{
		AutoDisableChannel: AutoDisableChannel{
			Enabled: true,
			Statuses: []AutoDisableChannelStatus{
				{Status: 401, Times: 2},
				{Status: 403, Times: 1},
			},
		},
	}

	// Test 401 - needs 2 times
	svc.apiKeyErrorCounts = map[int]map[string]map[int]int{
		ch.ID: {"key1": {401: 1}},
	}

	perf401 := &PerformanceRecord{
		ChannelID:       ch.ID,
		APIKey:          "key1",
		ErrorStatusCode: 401,
		Success:         false,
	}

	result := svc.checkAndHandleAPIKeyError(ctx, perf401, policy)
	require.True(t, result)

	// Reset for 403 test
	_, err := client.Channel.UpdateOneID(ch.ID).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{}).
		Save(ctx)
	require.NoError(t, err)

	svc.apiKeyErrorCounts = make(map[int]map[string]map[int]int)

	// Test 403 - needs only 1 time
	perf403 := &PerformanceRecord{
		ChannelID:       ch.ID,
		APIKey:          "key2",
		ErrorStatusCode: 403,
		Success:         false,
	}

	result = svc.checkAndHandleAPIKeyError(ctx, perf403, policy)
	require.True(t, result)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1", "key2", "key3"})

	policy := &RetryPolicy{
		AutoDisableChannel: AutoDisableChannel{
			Enabled: true,
			Statuses: []AutoDisableChannelStatus{
				{Status: 401, Times: 5},
			},
		},
	}

	// Simulate concurrent error reporting
	var wg sync.WaitGroup

	numGoroutines := 10

	for i := range numGoroutines {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			perf := &PerformanceRecord{
				ChannelID:       ch.ID,
				APIKey:          "key1",
				ErrorStatusCode: 401,
				Success:         false,
			}
			svc.checkAndHandleAPIKeyError(ctx, perf, policy)
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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "test-channel", []string{"key1"})

	// Try to disable an empty key - should return error
	err := svc.DisableAPIKey(ctx, ch.ID, "", 401, "Reason")
	require.Error(t, err)
}
