package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/pipeline/stream"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/internal/server/biz"
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
	proxy *objects.ProxyConfig,
) (*TestChannelResult, error) {
	inbound := openai.NewInboundTransformer()
	// Create ChatCompletionOrchestrator for this test request
	chatProcessor := &ChatCompletionOrchestrator{
		channelSelector: NewSpecifiedChannelSelector(processor.channelService, channelID),
		RequestService:  processor.requestService,
		ChannelService:  processor.channelService,
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
	testModel := lo.FromPtr(modelID)
	if testModel == "" {
		channels, err := chatProcessor.channelSelector.Select(ctx, &llm.Request{})
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
