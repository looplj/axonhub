package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/search"
	"github.com/looplj/axonhub/llm/streams"
)

func TestSearchOrchestrator_Process_NonStreaming(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()
	ctx = ent.NewContext(ctx, client)

	project := createTestProject(t, ctx, client)

	// Create a search_tavily channel; outbound transformer isn't used because we mock executor responses.
	ch, err := client.Channel.Create().
		SetType(channel.TypeSearchTavily).
		SetName("Tavily Search").
		SetBaseURL("https://api.tavily.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"__search", "__tavily_search"}).
		SetDefaultTestModel("__search").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	outbound := &fakeSearchOutbound{}
	bizChannel := &biz.Channel{Channel: ch, Outbound: outbound}
	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "__search")}

	executor := &mockExecutor{
		response: &httpclient.Response{
			StatusCode: 200,
			Body:       []byte(`{}`),
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:   channelSelector,
		Inbound:           search.NewInboundTransformer(),
		RequestService:    requestService,
		ChannelService:    channelService,
		SystemService:     systemService,
		UsageLogService:   usageLogService,
		PipelineFactory:   pipeline.NewFactory(executor),
		ModelMapper:       NewModelMapper(),
		connectionTracker: NewDefaultConnectionTracker(64),
	}

	reqBody, err := json.Marshal(map[string]any{"query": "hello", "model": "__search"})
	require.NoError(t, err)

	httpRequest := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/search",
		Path:    "/v1/search",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	ctx = contexts.WithProjectID(ctx, project.ID)

	result, err := orchestrator.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Nil(t, result.ChatCompletionStream)

	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	assert.Equal(t, "__search", requests[0].ModelID)
	assert.Equal(t, request.StatusCompleted, requests[0].Status)

	execs, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, execs, 1)
	assert.Equal(t, ch.ID, execs[0].ChannelID)
}

type fakeSearchOutbound struct{}

func (t *fakeSearchOutbound) APIFormat() llm.APIFormat { return llm.APIFormatTavilySearch }

func (t *fakeSearchOutbound) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         "https://api.tavily.com/search",
		Headers:     http.Header{"Content-Type": []string{"application/json"}},
		Body:        []byte(`{}`),
		RequestType: llm.RequestTypeSearch.String(),
		APIFormat:   llm.APIFormatTavilySearch.String(),
	}, nil
}

func (t *fakeSearchOutbound) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return &llm.Response{
		RequestType: llm.RequestTypeSearch,
		APIFormat:   llm.APIFormatTavilySearch,
		Search: &llm.SearchResponse{
			Query:        "hello",
			Results:      []llm.SearchResult{},
			ResponseTime: 0.1,
		},
		Usage: &llm.Usage{Quantity: 1},
	}, nil
}

func (t *fakeSearchOutbound) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, assert.AnError
}

func (t *fakeSearchOutbound) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return &llm.ResponseError{StatusCode: http.StatusBadGateway, Detail: llm.ErrorDetail{Message: "upstream error", Type: "api_error"}}
}

func (t *fakeSearchOutbound) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, assert.AnError
}
