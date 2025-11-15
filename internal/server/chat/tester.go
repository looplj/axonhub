package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/pipeline/stream"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/internal/server/biz"
)

// TestChannelProcessor handles channel testing functionality.
// It is stateless and can be reused across multiple test requests.
type TestChannelProcessor struct {
	channelService *biz.ChannelService
	requestService *biz.RequestService
	systemService  *biz.SystemService
	httpClient     *httpclient.HttpClient
}

// NewTestChannelProcessor creates a new TestChannelProcessor.
func NewTestChannelProcessor(
	channelService *biz.ChannelService,
	requestService *biz.RequestService,
	systemService *biz.SystemService,
	httpClient *httpclient.HttpClient,
) *TestChannelProcessor {
	return &TestChannelProcessor{
		channelService: channelService,
		requestService: requestService,
		systemService:  systemService,
		httpClient:     httpClient,
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
func (processor *TestChannelProcessor) TestChannel(
	ctx context.Context,
	channelID objects.GUID,
	modelID *string,
	proxyConfig *objects.ProxyConfig,
) (*TestChannelResult, error) {
	// Create ChatCompletionProcessor for this test request
	chatProcessor := &ChatCompletionProcessor{
		ChannelSelector: NewSpecifiedChannelSelector(processor.channelService, channelID),
		Inbound:         openai.NewInboundTransformer(),
		RequestService:  processor.requestService,
		PipelineFactory: pipeline.NewFactory(processor.httpClient),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
		SystemService: processor.systemService,
		Proxy:         proxyConfig,
	}

	// Create a simple test request
	testModel := lo.FromPtr(modelID)
	if testModel == "" {
		channels, err := chatProcessor.ChannelSelector.Select(ctx, &llm.Request{})
		if err != nil {
			return nil, err
		}

		if len(channels) == 0 {
			return nil, fmt.Errorf("%w: no channels available", biz.ErrInvalidModel)
		}

		testModel = channels[0].DefaultTestModel
	}

	llmRequest := &llm.Request{
		Model: testModel,
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello, this is a test message. Please respond with 'Test successful'."),
				},
			},
		},
		MaxTokens: lo.ToPtr(int64(256)),
		Stream:    lo.ToPtr(false),
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

	if err != nil {
		return &TestChannelResult{
			Latency: latency,
			Success: false,
			Message: lo.ToPtr(""),
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	response, err := xjson.To[llm.Response](rawResponse.ChatCompletion.Body)
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
