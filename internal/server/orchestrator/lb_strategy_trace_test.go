package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestTraceAwareStrategy_Name(t *testing.T) {
	mockProvider := &mockTraceProvider{
		lastSuccessChannel: make(map[int]int),
	}
	strategy := NewTraceAwareStrategy(mockProvider)
	assert.Equal(t, "TraceAware", strategy.Name())
}

func TestTraceAwareStrategy_Score_NoTrace(t *testing.T) {
	ctx := context.Background()

	mockProvider := &mockTraceProvider{
		lastSuccessChannel: make(map[int]int),
	}
	strategy := NewTraceAwareStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 0.0, score, "Should return 0 when no trace in context")
}

func TestTraceAwareStrategy_Score_WithMockTrace(t *testing.T) {
	ctx := context.Background()

	// Mock: trace-123 last succeeded on channel 1
	mockProvider := &mockTraceProvider{
		lastSuccessChannel: map[int]int{
			123: 1,
		},
	}
	strategy := NewTraceAwareStrategy(mockProvider)

	// Add trace ID to context
	ctx = contexts.WithTrace(ctx, &ent.Trace{ID: 123})

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 1000.0, score, "Should return max boost for last successful channel")
}

func TestTraceAwareStrategy_Score_WithMockDifferentChannel(t *testing.T) {
	ctx := context.Background()

	// Mock: trace-456 last succeeded on channel 1
	mockProvider := &mockTraceProvider{
		lastSuccessChannel: map[int]int{
			456: 1,
		},
	}
	strategy := NewTraceAwareStrategy(mockProvider)

	// Add trace ID to context
	ctx = contexts.WithTrace(ctx, &ent.Trace{ID: 456})

	// Test with channel 2 (different from last success)
	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 2, Name: "test2"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 0.0, score, "Should return 0 for channels that weren't last successful")
}

func TestTraceAwareStrategy_Score_WithLastSuccessChannel(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create project
	project, err := client.Project.Create().
		SetName("test").
		Save(ctx)
	require.NoError(t, err)

	// Create channel
	ch, err := client.Channel.Create().
		SetName("test").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key"}}).
		Save(ctx)
	require.NoError(t, err)

	// Create trace
	trace, err := client.Trace.Create().
		SetProjectID(project.ID).
		SetTraceID("test-trace-123").
		Save(ctx)
	require.NoError(t, err)

	// Create a successful request in this trace
	_, err = client.Request.Create().
		SetProjectID(project.ID).
		SetTraceID(trace.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetStatus("completed").
		SetSource("api").
		SetRequestBody([]byte(`{"model":"gpt-4","messages":[]}`)).
		Save(ctx)
	require.NoError(t, err)

	// Add trace ID to context and ent client
	ctx = contexts.WithTrace(ctx, &ent.Trace{ID: trace.ID})
	ctx = ent.NewContext(ctx, client)

	requestService := newTestRequestService(client)
	strategy := NewTraceAwareStrategy(requestService)

	channel := &biz.Channel{Channel: ch}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 1000.0, score, "Should return max boost for last successful channel")
}

func TestTraceAwareStrategy_Score_DifferentChannel(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	project, err := client.Project.Create().
		SetName("test").
		Save(ctx)
	require.NoError(t, err)

	ch1, err := client.Channel.Create().
		SetName("ch1").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key-1"}}).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetName("ch2").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key-2"}}).
		Save(ctx)
	require.NoError(t, err)

	trace, err := client.Trace.Create().
		SetProjectID(project.ID).
		SetTraceID("test-trace-456").
		Save(ctx)
	require.NoError(t, err)

	// Create successful request with ch1
	_, err = client.Request.Create().
		SetProjectID(project.ID).
		SetTraceID(trace.ID).
		SetChannelID(ch1.ID).
		SetModelID("gpt-4").
		SetStatus("completed").
		SetSource("api").
		SetRequestBody([]byte(`{"model":"gpt-4","messages":[]}`)).
		Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithTrace(ctx, &ent.Trace{ID: trace.ID})

	requestService := newTestRequestService(client)
	strategy := NewTraceAwareStrategy(requestService)

	// Test with ch2 (different channel)
	channel := &biz.Channel{Channel: ch2}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 0.0, score, "Should return 0 for channels that weren't last successful")
}

func TestTraceAwareStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name               string
		traceID            int
		channelID          int
		lastSuccessChannel int
		hasTrace           bool
	}{
		{
			name:     "no trace",
			hasTrace: false,
		},
		{
			name:               "matching channel",
			traceID:            123,
			channelID:          1,
			lastSuccessChannel: 1,
			hasTrace:           true,
		},
		{
			name:               "different channel",
			traceID:            123,
			channelID:          2,
			lastSuccessChannel: 1,
			hasTrace:           true,
		},
		{
			name:               "no last success",
			traceID:            456,
			channelID:          1,
			lastSuccessChannel: 0,
			hasTrace:           true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockProvider := &mockTraceProvider{
				lastSuccessChannel: map[int]int{},
			}
			if tc.lastSuccessChannel > 0 {
				mockProvider.lastSuccessChannel[tc.traceID] = tc.lastSuccessChannel
			}

			strategy := NewTraceAwareStrategy(mockProvider)

			testCtx := ctx
			if tc.hasTrace {
				testCtx = contexts.WithTrace(ctx, &ent.Trace{ID: tc.traceID})
			}

			channel := &biz.Channel{
				Channel: &ent.Channel{ID: tc.channelID, Name: "test"},
			}

			score := strategy.Score(testCtx, channel)
			debugScore, _ := strategy.ScoreWithDebug(testCtx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for %s", tc.name)
		})
	}
}
