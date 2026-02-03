package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// newTestChannelServiceForChannels creates a minimal channel service for testing.
func newTestChannelServiceForChannels(client *ent.Client) *biz.ChannelService {
	systemService := newTestSystemService(client)

	return biz.NewChannelService(biz.ChannelServiceParams{
		Executor:      executors.NewPoolScheduleExecutor(),
		Ent:           client,
		SystemService: systemService,
	})
}

// newTestModelService creates a minimal model service for testing.
func newTestModelService(client *ent.Client) *biz.ModelService {
	return biz.NewModelService(biz.ModelServiceParams{
		Ent: client,
	})
}

// newTestLoadBalancedSelector creates a load-balanced selector with ModelService for testing.
func newTestLoadBalancedSelector(
	channelService *biz.ChannelService,
	client *ent.Client,
	systemService *biz.SystemService,
	requestService *biz.RequestService,
	connectionTracker *DefaultConnectionTracker,
) CandidateSelector {
	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(requestService),
		NewErrorAwareStrategy(channelService),
		NewWeightRoundRobinStrategy(channelService),
		NewConnectionAwareStrategy(channelService, connectionTracker),
	}
	loadBalancer := NewLoadBalancer(systemService, nil, strategies...)

	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)

	return WithLoadBalancedSelector(baseSelector, loadBalancer, systemService)
}

// newTestSystemService creates a minimal system service for testing.
func newTestSystemService(client *ent.Client) *biz.SystemService {
	return biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
}

// newTestRequestServiceForChannels creates a minimal request service for testing.
func newTestRequestServiceForChannels(client *ent.Client, systemService *biz.SystemService) *biz.RequestService {
	dataStorageService := &biz.DataStorageService{
		AbstractService: &biz.AbstractService{},
		SystemService:   systemService,
		Cache:           xcache.NewFromConfig[ent.DataStorage](xcache.Config{Mode: xcache.ModeMemory}),
	}
	channelService := biz.NewChannelServiceForTest(client)
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	return biz.NewRequestService(client, systemService, usageLogService, dataStorageService)
}

// setupTest creates a test context and ent client for testing.
func setupTest(t *testing.T) (context.Context, *ent.Client) {
	t.Helper()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx = ent.NewContext(ctx, client)

	return ctx, client
}

// createTestChannels creates multiple test channels for testing.
func createTestChannels(t *testing.T, ctx context.Context, client *ent.Client) []*ent.Channel {
	t.Helper()

	channels := make([]*ent.Channel, 0)

	// Channel 1: High weight, healthy
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("High Weight Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key-1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(100).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch1)

	// Channel 2: Medium weight, healthy
	ch2, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Medium Weight Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key-2"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch2)

	// Channel 3: Low weight, healthy
	ch3, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Low Weight Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key-3"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(25).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch3)

	// Channel 4: Disabled channel
	ch4, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Disabled Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key-4"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(75).
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch4)

	return channels
}

// getCandidateNameByID returns the channel name from a candidate list by channel ID.
func getCandidateNameByID(result []*ChannelModelsCandidate, channelID int) string {
	for _, c := range result {
		if c.Channel.ID == channelID {
			return c.Channel.Name
		}
	}

	return "unknown"
}

// mockExecutor implements pipeline.Executor for testing.
type mockExecutor struct {
	response      *httpclient.Response
	streamEvents  []*httpclient.StreamEvent
	err           error
	requestCalled bool
	lastRequest   *httpclient.Request
}

func (m *mockExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	m.requestCalled = true

	m.lastRequest = request
	if m.err != nil {
		return nil, m.err
	}

	return m.response, nil
}

func (m *mockExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	m.requestCalled = true

	m.lastRequest = request
	if m.err != nil {
		return nil, m.err
	}

	return streams.SliceStream(m.streamEvents), nil
}

// setupTestServices creates all necessary services for integration testing.
func setupTestServices(t *testing.T, client *ent.Client) (*biz.ChannelService, *biz.RequestService, *biz.SystemService, *biz.UsageLogService) {
	t.Helper()

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: cacheConfig,
		Ent:         client,
	})

	dataStorageService := &biz.DataStorageService{
		AbstractService: &biz.AbstractService{},
		SystemService:   systemService,
		Cache:           xcache.NewFromConfig[ent.DataStorage](cacheConfig),
	}

	channelService := biz.NewChannelServiceForTest(client)
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)
	requestService := biz.NewRequestService(client, systemService, usageLogService, dataStorageService)

	channelService = biz.NewChannelServiceForTest(client)

	return channelService, requestService, systemService, usageLogService
}

// createTestChannel creates a test channel in the database.
func createTestChannel(t *testing.T, ctx context.Context, client *ent.Client) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test OpenAI Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-api-key"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-3.5-turbo").
		Save(ctx)
	require.NoError(t, err)

	return ch
}

// createTestProject creates a test project in the database.
func createTestProject(t *testing.T, ctx context.Context, client *ent.Client) *ent.Project {
	t.Helper()

	project, err := client.Project.Create().
		SetName("Test Project").
		Save(ctx)
	require.NoError(t, err)

	return project
}

// buildMockOpenAIResponse creates a mock OpenAI chat completion response.
func buildMockOpenAIResponse(id, model, content string, promptTokens, completionTokens int) []byte {
	resp := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": 1234567890,
		"model":   model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}

	body, _ := json.Marshal(resp)

	return body
}

// buildTestRequest creates a test HTTP request for chat completion.
func buildTestRequest(model, content string, stream bool) *httpclient.Request {
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
		"stream": stream,
	}

	body, _ := json.Marshal(reqBody)

	return &httpclient.Request{
		Method: "POST",
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: body,
	}
}

// staticChannelSelector is a simple channel selector for testing.
type staticChannelSelector struct {
	candidates []*ChannelModelsCandidate
}

func (s *staticChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	return s.candidates, nil
}

// channelsToTestCandidates creates candidates from channels for testing.
func channelsToTestCandidates(channels []*biz.Channel, model string) []*ChannelModelsCandidate {
	candidates := make([]*ChannelModelsCandidate, 0, len(channels))
	for _, ch := range channels {
		entries := ch.GetModelEntries()

		entry, ok := entries[model]
		if !ok {
			continue
		}

		candidates = append(candidates, &ChannelModelsCandidate{
			Channel:  ch,
			Priority: 0,
			Models:   []biz.ChannelModelEntry{entry},
		})
	}

	return candidates
}

// newTestOrchestrator creates a ChatCompletionOrchestrator for testing.
func newTestOrchestrator(
	t *testing.T,
	channelSelector CandidateSelector,
	client *ent.Client,
	executor pipeline.Executor,
) *ChatCompletionOrchestrator {
	t.Helper()

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:   channelSelector,
		Inbound:           openai.NewInboundTransformer(),
		RequestService:    requestService,
		ChannelService:    channelService,
		PromptProvider:    &stubPromptProvider{},
		SystemService:     systemService,
		UsageLogService:   usageLogService,
		PipelineFactory:   pipeline.NewFactory(executor),
		ModelMapper:       NewModelMapper(),
		connectionTracker: NewDefaultConnectionTracker(1024),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	return orchestrator
}
