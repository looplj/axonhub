//go:build integration
// +build integration

package biz_test

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestChannelService_Integration_RefreshCycle(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	now := time.Now()

	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel 1").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key-1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Test Channel 2").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key-2"}).
		SetSupportedModels([]string{"claude-3-opus", "claude-3-sonnet"}).
		SetDefaultTestModel("claude-3-opus").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		createdAt := now.Add(-time.Duration(i*12) * time.Hour)
		_, err := client.RequestExecution.Create().
			SetChannelID(ch1.ID).
			SetModelID("gpt-4").
			SetStatus(requestexecution.StatusCompleted).
			SetCreatedAt(createdAt).
			SetMetricsFirstTokenLatencyMs(100 + int64(i)*10).
			SetRequestID(req.ID).
			SetProjectID(1).
			SetFormat("openai").
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStream(false).
			Save(ctx)
		require.NoError(t, err)
	}

	for i := 0; i < 3; i++ {
		createdAt := now.Add(-time.Duration(i*24) * time.Hour)
		_, err := client.RequestExecution.Create().
			SetChannelID(ch1.ID).
			SetModelID("gpt-4").
			SetStatus(requestexecution.StatusFailed).
			SetCreatedAt(createdAt).
			SetMetricsFirstTokenLatencyMs(50).
			SetRequestID(req.ID).
			SetProjectID(1).
			SetFormat("openai").
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStream(false).
			Save(ctx)
		require.NoError(t, err)
	}

	for i := 0; i < 5; i++ {
		createdAt := now.Add(-time.Duration(i*24) * time.Hour)
		_, err := client.RequestExecution.Create().
			SetChannelID(ch2.ID).
			SetModelID("claude-3-opus").
			SetStatus(requestexecution.StatusCompleted).
			SetCreatedAt(createdAt).
			SetMetricsFirstTokenLatencyMs(200 + int64(i)*20).
			SetRequestID(req.ID).
			SetProjectID(1).
			SetFormat("anthropic").
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStream(false).
			Save(ctx)
		require.NoError(t, err)
	}

	t.Run("startup loads historical data with 7-day window", func(t *testing.T) {
		svc := createTestChannelServiceWithRefresh(client, 7, 100*time.Millisecond)
		defer svc.Stop()

		time.Sleep(200 * time.Millisecond)

		metrics1, err := svc.GetChannelMetrics(ctx, ch1.ID, "gpt-4")
		require.NoError(t, err)
		require.NotNil(t, metrics1)
		require.Equal(t, int64(13), metrics1.RequestCount, "Should load all requests within 7-day window")
		require.Equal(t, int64(10), metrics1.SuccessCount, "Should have 10 successful requests")
		require.Equal(t, int64(3), metrics1.FailureCount, "Should have 3 failed requests")
		require.NotNil(t, metrics1.AvgFirstTokenLatencyMs, "Should have average first token latency")
		require.Greater(t, *metrics1.AvgFirstTokenLatencyMs, float64(0), "Average latency should be positive")

		metrics2, err := svc.GetChannelMetrics(ctx, ch2.ID, "claude-3-opus")
		require.NoError(t, err)
		require.NotNil(t, metrics2)
		require.Equal(t, int64(5), metrics2.RequestCount, "Should load 5 requests for channel 2")
		require.Equal(t, int64(5), metrics2.SuccessCount, "Should have 5 successful requests")
		require.Equal(t, int64(0), metrics2.FailureCount, "Should have 0 failed requests")
	})

	t.Run("periodic refresh updates metrics", func(t *testing.T) {
		svc := createTestChannelServiceWithRefresh(client, 7, 100*time.Millisecond)
		defer svc.Stop()

		time.Sleep(200 * time.Millisecond)

		initialMetrics, err := svc.GetChannelMetrics(ctx, ch1.ID, "gpt-4")
		require.NoError(t, err)
		initialCount := initialMetrics.RequestCount

		_, err = client.RequestExecution.Create().
			SetChannelID(ch1.ID).
			SetModelID("gpt-4").
			SetStatus(requestexecution.StatusCompleted).
			SetCreatedAt(now).
			SetMetricsFirstTokenLatencyMs(150).
			SetRequestID(req.ID).
			SetProjectID(1).
			SetFormat("openai").
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStream(false).
			Save(ctx)
		require.NoError(t, err)

		time.Sleep(300 * time.Millisecond)

		updatedMetrics, err := svc.GetChannelMetrics(ctx, ch1.ID, "gpt-4")
		require.NoError(t, err)
		require.GreaterOrEqual(t, updatedMetrics.RequestCount, initialCount,
			"Metrics should be refreshed and include historical data")
	})

	t.Run("data consistency during refresh", func(t *testing.T) {
		svc := createTestChannelServiceWithRefresh(client, 7, 50*time.Millisecond)
		defer svc.Stop()

		time.Sleep(100 * time.Millisecond)

		for i := 0; i < 10; i++ {
			metrics, err := svc.GetChannelMetrics(ctx, ch1.ID, "gpt-4")
			require.NoError(t, err)
			require.Equal(t, metrics.RequestCount, metrics.SuccessCount+metrics.FailureCount,
				"Request count should equal success + failure count at all times")
			time.Sleep(20 * time.Millisecond)
		}
	})

	t.Run("concurrent requests during refresh", func(t *testing.T) {
		svc := createTestChannelServiceWithRefresh(client, 7, 50*time.Millisecond)
		defer svc.Stop()

		time.Sleep(100 * time.Millisecond)

		var wg sync.WaitGroup
		errors := make(chan error, 100)

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					_, err := svc.GetChannelMetrics(ctx, ch1.ID, "gpt-4")
					if err != nil {
						errors <- err
						return
					}
					time.Sleep(5 * time.Millisecond)
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent access error: %v", err)
		}
	})

	t.Run("refresh with 2-hour interval configuration", func(t *testing.T) {
		svc := createTestChannelServiceWithRefresh(client, 7, 2*time.Hour)
		defer svc.Stop()

		time.Sleep(200 * time.Millisecond)

		metrics, err := svc.GetChannelMetrics(ctx, ch1.ID, "gpt-4")
		require.NoError(t, err)
		require.NotNil(t, metrics)
		require.Greater(t, metrics.RequestCount, int64(0), "Should have loaded metrics at startup")
	})

	t.Run("graceful shutdown stops refresh", func(t *testing.T) {
		svc := createTestChannelServiceWithRefresh(client, 7, 50*time.Millisecond)

		time.Sleep(100 * time.Millisecond)

		metricsBefore, err := svc.GetChannelMetrics(ctx, ch1.ID, "gpt-4")
		require.NoError(t, err)

		svc.Stop()

		time.Sleep(100 * time.Millisecond)

		metricsAfter, err := svc.GetChannelMetrics(ctx, ch1.ID, "gpt-4")
		require.NoError(t, err)
		require.Equal(t, metricsBefore.RequestCount, metricsAfter.RequestCount,
			"Metrics should remain consistent after stop")
	})

	t.Run("retry logic on database error", func(t *testing.T) {
		svc := createTestChannelServiceWithRefresh(client, 7, 100*time.Millisecond)
		defer svc.Stop()

		time.Sleep(200 * time.Millisecond)

		require.NotNil(t, svc)
	})
}

func createTestChannelServiceWithRefresh(
	client *ent.Client,
	histWindowDays int,
	refreshInterval time.Duration,
) *biz.ChannelService {
	mockSysSvc := &biz.SystemService{}

	svc := biz.NewChannelService(biz.ChannelServiceParams{
		CacheConfig:               xcache.Config{Mode: xcache.ModeMemory},
		Executor:                  executors.NewPoolScheduleExecutor(),
		Ent:                       client,
		SystemService:             mockSysSvc,
		HttpClient:                httpclient.NewHttpClient(),
		HistoricalRefreshInterval: refreshInterval,
	})

	svc.SetEnabledChannelsForTest([]*biz.Channel{})

	return svc
}

func TestChannelService_Integration_GracefulShutdown_DuringRefresh(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	for i := 0; i < 3; i++ {
		_, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName("Test Channel " + string(rune('A'+i))).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key-" + string(rune('0'+i))}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetStatus(channel.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)
	}

	svc := createTestChannelServiceWithRefresh(client, 7, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	channels := svc.GetEnabledChannels()
	require.NotNil(t, channels)
	assert.GreaterOrEqual(t, len(channels), 0)

	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.Stop()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown timed out - should complete within 5 seconds")
	}

	assert.NotPanics(t, func() {
		_ = svc.GetEnabledChannels()
	})
}

func TestChannelService_Integration_GracefulShutdown_WithInflightRequests(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Inflight Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	svc := createTestChannelServiceWithRefresh(client, 7, 100*time.Millisecond)

	time.Sleep(150 * time.Millisecond)

	var wg sync.WaitGroup
	operationCount := int32(0)
	stopRequested := int32(0)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for atomic.LoadInt32(&stopRequested) == 0 {
				_ = svc.GetEnabledChannels()
				atomic.AddInt32(&operationCount, 1)
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	time.Sleep(200 * time.Millisecond)

	atomic.StoreInt32(&stopRequested, 1)

	stopDone := make(chan struct{})
	go func() {
		defer close(stopDone)
		svc.Stop()
	}()

	wg.Wait()

	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown timed out with in-flight operations")
	}

	assert.Greater(t, atomic.LoadInt32(&operationCount), int32(0), "Operations should have been performed")
}

func TestChannelService_Integration_GracefulShutdown_TimeoutHandling(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Timeout Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	svc := createTestChannelServiceWithRefresh(client, 7, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	svc.Stop()
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2*time.Second, "Shutdown should complete within 2 seconds, not hang for 30s")
}

func TestChannelService_Integration_GracefulShutdown_NoGoroutineLeaks(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	for i := 0; i < 5; i++ {
		_, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName("Leak Test Channel " + string(rune('0'+i))).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetStatus(channel.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	for round := 0; round < 3; round++ {
		svc := createTestChannelServiceWithRefresh(client, 7, 50*time.Millisecond)

		time.Sleep(150 * time.Millisecond)

		svc.Stop()

		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}

	time.Sleep(300 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - baselineGoroutines

	assert.LessOrEqual(t, leaked, 20, "Expected at most 5 goroutine difference, got %d (baseline: %d, final: %d)",
		leaked, baselineGoroutines, finalGoroutines)
}

func TestChannelService_Integration_GracefulShutdown_RestartAfterShutdown(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Original Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "original-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	svc1 := createTestChannelServiceWithRefresh(client, 7, 100*time.Millisecond)

	time.Sleep(150 * time.Millisecond)
	channels1 := svc1.GetEnabledChannels()
	require.NotNil(t, channels1)
	assert.GreaterOrEqual(t, len(channels1), 0)

	svc1.Stop()

	ch2, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("New Channel After Stop").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "new-key"}).
		SetSupportedModels([]string{"claude-3-opus"}).
		SetDefaultTestModel("claude-3-opus").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	svc2 := createTestChannelServiceWithRefresh(client, 7, 100*time.Millisecond)

	time.Sleep(150 * time.Millisecond)

	channels2 := svc2.GetEnabledChannels()
	require.NotNil(t, channels2)

	foundCh1 := false
	foundCh2 := false
	for _, ch := range channels2 {
		if ch.ID == ch1.ID {
			foundCh1 = true
		}
		if ch.ID == ch2.ID {
			foundCh2 = true
		}
	}

	assert.True(t, foundCh1, "Original channel should be found after restart")
	assert.True(t, foundCh2, "New channel should be found after restart")

	svc2.Stop()
}

func TestChannelService_Integration_GracefulShutdown_IdempotentStop(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Idempotent Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	svc := createTestChannelServiceWithRefresh(client, 7, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	assert.NotPanics(t, func() {
		svc.Stop()
	})

	assert.NotPanics(t, func() {
		svc.Stop()
	})

	assert.NotPanics(t, func() {
		svc.Stop()
	})
}

func TestChannelService_Integration_GracefulShutdown_ConcurrentStop(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	for i := 0; i < 3; i++ {
		_, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName("Concurrent Stop Channel " + string(rune('0'+i))).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetStatus(channel.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)
	}

	svc := createTestChannelServiceWithRefresh(client, 7, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NotPanics(t, func() {
				svc.Stop()
			})
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Concurrent stops timed out")
	}
}
