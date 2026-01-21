package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// TestChannelOrchestrator handles channel testing functionality.
// It is stateless and can be reused across multiple test requests.
type TestChannelOrchestrator struct {
	channelService  *biz.ChannelService
	requestService  *biz.RequestService
	systemService   *biz.SystemService
	usageLogService *biz.UsageLogService
	httpClient      *httpclient.HttpClient
}

// NewTestChannelOrchestrator creates a new TestChannelOrchestrator.
func NewTestChannelOrchestrator(
	channelService *biz.ChannelService,
	requestService *biz.RequestService,
	systemService *biz.SystemService,
	usageLogService *biz.UsageLogService,
	httpClient *httpclient.HttpClient,
) *TestChannelOrchestrator {
	return &TestChannelOrchestrator{
		channelService:  channelService,
		requestService:  requestService,
		systemService:   systemService,
		usageLogService: usageLogService,
		httpClient:      httpClient,
	}
}

// TestChannelRequest represents a channel test request.
type TestChannelRequest struct {
	ChannelID objects.GUID
	ModelID   *string
}

// TestChannelResult represents the result of a channel test.
type TestChannelResult struct {
	Latency float64
	Success bool
	Message *string
	Error   *string
}

// TestChannel tests a specific channel with a simple request.
func (processor *TestChannelOrchestrator) TestChannel(
	ctx context.Context,
	channelID objects.GUID,
	modelID *string,
	proxy *httpclient.ProxyConfig,
) (*TestChannelResult, error) {
	inbound := openai.NewInboundTransformer()
	// Create ChatCompletionOrchestrator for this test request
	chatProcessor := &ChatCompletionOrchestrator{
		channelSelector: NewSpecifiedChannelSelector(processor.channelService, channelID),
		RequestService:  processor.requestService,
		ChannelService:  processor.channelService,
		PromptProvider:  &stubPromptProvider{},
		PipelineFactory: pipeline.NewFactory(processor.httpClient),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
		Inbound:              inbound,
		SystemService:        processor.systemService,
		UsageLogService:      processor.usageLogService,
		proxy:                proxy,
		ModelMapper:          nil,
		selectedChannelIds:   []int{},
		adaptiveLoadBalancer: nil,
		weightedLoadBalancer: nil,
		connectionTracker:    nil,
	}

	// Create a simple test request
	selectedChannel, err := processor.channelService.GetChannel(ctx, channelID.ID)
	if err != nil {
		return nil, err
	}
	testModel := lo.FromPtr(modelID)
	if testModel == "" {
		testModel = selectedChannel.DefaultTestModel
	}

	testStream := true
	if selectedChannel.Settings != nil && selectedChannel.Settings.TestStream != nil {
		testStream = *selectedChannel.Settings.TestStream
	}

	llmRequest := &llm.Request{
		Model: testModel,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: lo.ToPtr("You are a helpful assistant."),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: lo.ToPtr("Hello world, I'm AxonHub."),
						},
						{
							Type: "text",
							Text: lo.ToPtr("Please tell me who you are?"),
						},
					},
				},
			},
		},
		MaxCompletionTokens: lo.ToPtr(int64(1)),
		Stream:              lo.ToPtr(testStream),
	}

	body, err := json.Marshal(llmRequest)
	if err != nil {
		return nil, err
	}

	// Measure latency
	startTime := time.Now()
	rawResponse, err := chatProcessor.Process(ctx, &httpclient.Request{
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: body,
	})

	latency := time.Since(startTime).Seconds()
	rawErr := inbound.TransformError(ctx, err)
	message := gjson.GetBytes(rawErr.Body, "error.message").String()

	//nolint:nilerr // Checked.
	if err != nil {
		return &TestChannelResult{
			Latency: latency,
			Success: false,
			Message: lo.ToPtr(""),
			Error:   lo.ToPtr(message),
		}, nil
	}

	var responseBody []byte
	if rawResponse.ChatCompletion != nil {
		responseBody = rawResponse.ChatCompletion.Body
	} else if rawResponse.ChatCompletionStream != nil {
		var chunks []*httpclient.StreamEvent
		for rawResponse.ChatCompletionStream.Next() {
			if event := rawResponse.ChatCompletionStream.Current(); event != nil {
				chunks = append(chunks, event)
			}
		}
		streamErr := rawResponse.ChatCompletionStream.Err()
		closeErr := rawResponse.ChatCompletionStream.Close()
		if streamErr != nil {
			return &TestChannelResult{
				Latency: latency,
				Success: false,
				Message: lo.ToPtr(""),
				Error:   lo.ToPtr(streamErr.Error()),
			}, nil
		}
		if closeErr != nil {
			return &TestChannelResult{
				Latency: latency,
				Success: false,
				Message: lo.ToPtr(""),
				Error:   lo.ToPtr(closeErr.Error()),
			}, nil
		}
		var aggErr error
		responseBody, _, aggErr = inbound.AggregateStreamChunks(ctx, chunks)
		if aggErr != nil {
			return &TestChannelResult{
				Latency: latency,
				Success: false,
				Message: lo.ToPtr(""),
				Error:   lo.ToPtr(aggErr.Error()),
			}, nil
		}
	} else {
		return &TestChannelResult{
			Latency: latency,
			Success: false,
			Message: lo.ToPtr(""),
			Error:   lo.ToPtr("No response body"),
		}, nil
	}

	response, err := xjson.To[llm.Response](responseBody)
	if err != nil {
		return &TestChannelResult{
			Latency: latency,
			Success: false,
			Message: lo.ToPtr(""),
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	if len(response.Choices) == 0 {
		return &TestChannelResult{
			Latency: latency,
			Success: false,
			Message: lo.ToPtr(""),
			Error:   lo.ToPtr("No message in response"),
		}, nil
	}

	return &TestChannelResult{
		Latency: latency,
		Success: true,
		Message: response.Choices[0].Message.Content.Content,
		Error:   nil,
	}, nil
}
