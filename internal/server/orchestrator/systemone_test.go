package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/typesafe"
)

type sequenceMockExecutor struct {
	responses []*httpclient.Response
	errors    []error
	requests  []*httpclient.Request
	calls     int
}

func (m *sequenceMockExecutor) Do(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
	m.requests = append(m.requests, req)
	idx := m.calls
	m.calls++
	if idx < len(m.errors) && m.errors[idx] != nil {
		return nil, m.errors[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return nil, fmt.Errorf("no more mock responses")
}

func (m *sequenceMockExecutor) DoStream(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("streaming not supported for systemone executor")
}

func TestChatCompletionOrchestrator_Process_SystemOne_Success(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	project := createTestProject(t, ctx, client)
	ctx = contexts.WithProjectID(ctx, project.ID)

	ch, err := client.Channel.Create().
		SetType(channel.TypeTypesafe).
		SetName("Test TypeSafe Channel").
		SetBaseURL("https://api.typesafe.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-typesafe-test-key"}).
		SetSupportedModels([]string{"jev-latest", "jev-preview", "jev-1.13.0"}).
		SetDefaultTestModel("jev-latest").
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	mockRespBody := []byte(`{
		"model": "jev-latest",
		"answers": {
			"urgent": {
				"type": "noul",
				"noul": 0.96
			}
		},
		"usage": {
			"input_tokens": 120,
			"output_tokens": 0
		}
	}`)

	executor := &mockExecutor{
		response: &httpclient.Response{
			StatusCode: 200,
			Body:       mockRespBody,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	outbound, err := typesafe.NewOutboundTransformerWithConfig(&typesafe.Config{
		BaseURL:        ch.BaseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("sk-typesafe-test-key"),
	})
	require.NoError(t, err)

	bizChannel := &biz.Channel{
		Channel:  ch,
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "jev-latest")}

	orch := &ChatCompletionOrchestrator{
		channelSelector:       channelSelector,
		Inbound:               typesafe.NewSystemOneInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	reqBody := []byte(`{
		"model": "jev-latest",
		"state": "The server CPU usage is 98% and memory is saturated.",
		"questions": {
			"urgent": {
				"type": "noul",
				"instructions": "Is this situation critical and requires immediate pager duty alert?"
			}
		}
	}`)

	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		Body:   reqBody,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	result, err := orch.Process(ctx, httpReq)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Equal(t, http.StatusOK, result.ChatCompletion.StatusCode)

	var respBody map[string]any
	err = json.Unmarshal(result.ChatCompletion.Body, &respBody)
	require.NoError(t, err)
	require.Equal(t, "jev-latest", respBody["model"])
	answers, ok := respBody["answers"].(map[string]any)
	require.True(t, ok)
	urgentAnswer, ok := answers["urgent"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "noul", urgentAnswer["type"])
	require.Equal(t, 0.96, urgentAnswer["noul"])

	usage, ok := respBody["usage"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(120), usage["input_tokens"])
	require.Equal(t, float64(0), usage["output_tokens"])
}

func TestChatCompletionOrchestrator_Process_SystemOne_PassThroughBody(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	passThroughTrue := true
	ch, err := client.Channel.Create().
		SetType(channel.TypeTypesafe).
		SetName("Test TypeSafe Channel PassThrough").
		SetBaseURL("https://api.typesafe.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-typesafe-test-key"}).
		SetSupportedModels([]string{"jev-latest"}).
		SetDefaultTestModel("jev-latest").
		SetSettings(&objects.ChannelSettings{
			PassThroughBody: &passThroughTrue,
		}).
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	mockRespBody := []byte(`{
		"model": "jev-latest",
		"answers": {
			"check": {
				"type": "noul",
				"noul": 0.88
			}
		}
	}`)

	executor := &mockExecutor{
		response: &httpclient.Response{
			StatusCode: 200,
			Body:       mockRespBody,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	outbound, err := typesafe.NewOutboundTransformerWithConfig(&typesafe.Config{
		BaseURL:        ch.BaseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("sk-typesafe-test-key"),
	})
	require.NoError(t, err)

	bizChannel := &biz.Channel{
		Channel:  ch,
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "jev-latest")}

	orch := &ChatCompletionOrchestrator{
		channelSelector:       channelSelector,
		Inbound:               typesafe.NewSystemOneInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	requestBody := []byte(`{"model":"jev-latest","state":"important context","questions":{"check":{"instructions":"OK?","type":"noul"}}}`)
	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		Body:   requestBody,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	result, err := orch.Process(ctx, httpReq)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Equal(t, http.StatusOK, result.ChatCompletion.StatusCode)
	require.True(t, executor.requestCalled)
	require.JSONEq(t, string(requestBody), string(executor.lastRequest.Body))
}

func TestChatCompletionOrchestrator_Process_SystemOne_EmptyResponseDetection(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeTypesafe).
		SetName("Test TypeSafe Channel").
		SetBaseURL("https://api.typesafe.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-typesafe-test-key"}).
		SetSupportedModels([]string{"jev-latest"}).
		SetDefaultTestModel("jev-latest").
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	err = systemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		EmptyResponseDetection: true,
	})
	require.NoError(t, err)

	mockRespBody := []byte(`{
		"model": "jev-latest",
		"answers": {
			"decision": {
				"type": "choice",
				"choice": "approve",
				"confidence": 0.99
			}
		}
	}`)

	executor := &mockExecutor{
		response: &httpclient.Response{
			StatusCode: 200,
			Body:       mockRespBody,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	outbound, err := typesafe.NewOutboundTransformerWithConfig(&typesafe.Config{
		BaseURL:        ch.BaseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("sk-typesafe-test-key"),
	})
	require.NoError(t, err)

	bizChannel := &biz.Channel{
		Channel:  ch,
		Outbound: outbound,
	}

	channelSelector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{bizChannel}, "jev-latest")}

	orch := &ChatCompletionOrchestrator{
		channelSelector:       channelSelector,
		Inbound:               typesafe.NewSystemOneInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
	}

	reqBody := []byte(`{
		"model": "jev-latest",
		"state": "Transaction amount is $5.",
		"questions": {
			"decision": {
				"type": "choice",
				"instructions": "Approve or decline?"
			}
		}
	}`)

	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		Body:   reqBody,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	result, err := orch.Process(ctx, httpReq)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Equal(t, http.StatusOK, result.ChatCompletion.StatusCode)
}

func TestChatCompletionOrchestrator_Process_SystemOne_Failover_429(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	// Channel 1: will return 429
	ch1, err := client.Channel.Create().
		SetType(channel.TypeTypesafe).
		SetName("Channel 1").
		SetBaseURL("https://api.typesafe.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-key-1"}).
		SetSupportedModels([]string{"jev-latest"}).
		SetDefaultTestModel("jev-latest").
		Save(ctx)
	require.NoError(t, err)

	// Channel 2: will return 200
	ch2, err := client.Channel.Create().
		SetType(channel.TypeTypesafe).
		SetName("Channel 2").
		SetBaseURL("https://api.typesafe.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-key-2"}).
		SetSupportedModels([]string{"jev-latest"}).
		SetDefaultTestModel("jev-latest").
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	seqExec := &sequenceMockExecutor{
		responses: []*httpclient.Response{
			{
				StatusCode: 429,
				Body:       []byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit"}}`),
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
			},
			{
				StatusCode: 200,
				Body:       []byte(`{"model":"jev-latest","answers":{"q":{"type":"noul","noul":1.0}}}`),
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
			},
		},
	}

	outbound1, _ := typesafe.NewOutboundTransformerWithConfig(&typesafe.Config{
		BaseURL:        ch1.BaseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("sk-key-1"),
	})
	outbound2, _ := typesafe.NewOutboundTransformerWithConfig(&typesafe.Config{
		BaseURL:        ch2.BaseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("sk-key-2"),
	})

	bizCh1 := &biz.Channel{Channel: ch1, Outbound: outbound1}
	bizCh2 := &biz.Channel{Channel: ch2, Outbound: outbound2}

	candidates := channelsToTestCandidates([]*biz.Channel{bizCh1, bizCh2}, "jev-latest")
	channelSelector := &staticChannelSelector{candidates: candidates}

	orch := &ChatCompletionOrchestrator{
		channelSelector:       channelSelector,
		Inbound:               typesafe.NewSystemOneInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(seqExec),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
	}

	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		Body:   []byte(`{"model":"jev-latest","state":"test","questions":{"q":{"type":"noul","instructions":"test"}}}`),
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	result, err := orch.Process(ctx, httpReq)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Equal(t, http.StatusOK, result.ChatCompletion.StatusCode)
	require.Equal(t, 2, seqExec.calls)
	require.Len(t, seqExec.requests, 2)
	if seqExec.requests[0].Auth != nil {
		require.Equal(t, "sk-key-1", seqExec.requests[0].Auth.APIKey)
	} else {
		require.Equal(t, "Bearer sk-key-1", seqExec.requests[0].Headers.Get("Authorization"))
	}
	if seqExec.requests[1].Auth != nil {
		require.Equal(t, "sk-key-2", seqExec.requests[1].Auth.APIKey)
	} else {
		require.Equal(t, "Bearer sk-key-2", seqExec.requests[1].Headers.Get("Authorization"))
	}
}

func TestChatCompletionOrchestrator_Process_SystemOne_NoRetry_401(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeTypesafe).
		SetName("Channel 1").
		SetBaseURL("https://api.typesafe.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-invalid-key"}).
		SetSupportedModels([]string{"jev-latest"}).
		SetDefaultTestModel("jev-latest").
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	seqExec := &sequenceMockExecutor{
		responses: []*httpclient.Response{
			{
				StatusCode: 401,
				Body:       []byte(`{"error":{"message":"Invalid API key","type":"authentication_error"}}`),
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
			},
		},
	}

	outbound, _ := typesafe.NewOutboundTransformerWithConfig(&typesafe.Config{
		BaseURL:        ch.BaseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("sk-invalid-key"),
	})

	bizCh := &biz.Channel{Channel: ch, Outbound: outbound}
	candidates := channelsToTestCandidates([]*biz.Channel{bizCh}, "jev-latest")
	channelSelector := &staticChannelSelector{candidates: candidates}

	orch := &ChatCompletionOrchestrator{
		channelSelector:       channelSelector,
		Inbound:               typesafe.NewSystemOneInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(seqExec),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
	}

	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		Body:   []byte(`{"model":"jev-latest","state":"test","questions":{"q":{"type":"noul","instructions":"test"}}}`),
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	result, err := orch.Process(ctx, httpReq)
	// 401 is a non-retryable error, returned directly as error and transformed into HTTP 401
	require.Error(t, err)
	require.Nil(t, result.ChatCompletion)
	require.Equal(t, 1, seqExec.calls)

	clientErr := orch.Inbound.TransformError(ctx, err)
	require.NotNil(t, clientErr)
	require.Equal(t, http.StatusUnauthorized, clientErr.StatusCode)
}
