package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	responses "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type encryptedReasoningSequenceExecutor struct {
	steps    []executorStep
	stepIdx  int
	requests [][]byte
}

func (e *encryptedReasoningSequenceExecutor) Do(_ context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	e.requests = append(e.requests, append([]byte(nil), request.Body...))
	if e.stepIdx >= len(e.steps) {
		return nil, errors.New("no more steps available")
	}
	step := e.steps[e.stepIdx]
	e.stepIdx++
	if step.err != nil {
		return nil, step.err
	}
	return step.resp, nil
}

func (e *encryptedReasoningSequenceExecutor) DoStream(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, errors.New("streaming not supported by encrypted reasoning test executor")
}

func TestChatCompletionOrchestrator_Process_RecoversOnceFromInvalidEncryptedReasoning(t *testing.T) {
	ctx, client, project, channelRow, orchestrator, executor := newEncryptedReasoningRetryHarness(t, 0, 1, []executorStep{
		{err: encryptedReasoningProviderError("", "The encrypted content for item rs_client_replay could not be verified. Reason: Encrypted content item_id did not match the target item id.")},
		{resp: recoveredResponsesResponse()},
	})
	defer client.Close()

	result, err := orchestrator.Process(ctx, encryptedReasoningRequest())
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Contains(t, string(result.ChatCompletion.Body), `"x_recovery_passthrough":true`,
		"sanitizing the retried request must not disable same-protocol response pass-through")
	require.Len(t, executor.requests, 2)

	first := decodeResponsesRequest(t, executor.requests[0])
	require.Contains(t, responsesItemTypes(first.Input.Items), "reasoning")
	require.Contains(t, responsesItemTypes(first.Input.Items), "compaction")
	require.Contains(t, responsesItemTypes(first.Input.Items), "compaction_summary")

	second := decodeResponsesRequest(t, executor.requests[1])
	reasoning := findResponsesItem(t, second.Input.Items, "reasoning")
	require.Empty(t, reasoning.ID)
	require.Nil(t, reasoning.EncryptedContent)
	require.Len(t, reasoning.Summary, 1)
	require.Equal(t, "I inspected the repository.", reasoning.Summary[0].Text)
	require.NotContains(t, responsesItemTypes(second.Input.Items), "compaction")
	require.NotContains(t, responsesItemTypes(second.Input.Items), "compaction_summary")
	require.NotContains(t, string(executor.requests[1]), "rs_replay_legacy")
	require.NotContains(t, string(executor.requests[1]), "gAAAA_replay_legacy")
	require.NotContains(t, string(executor.requests[1]), "cmp_replay_legacy")
	require.NotContains(t, string(executor.requests[1]), "gAAAA_compaction_legacy")

	toolCall := findResponsesItem(t, second.Input.Items, "function_call")
	require.Equal(t, "fc_replay_legacy", toolCall.ID)
	require.Equal(t, "call_replay_legacy", toolCall.CallID)
	require.Equal(t, "read_file", toolCall.Name)

	toolResult := findResponsesItem(t, second.Input.Items, "function_call_output")
	require.Equal(t, "fco_replay_legacy", toolResult.ID)
	require.Equal(t, "call_replay_legacy", toolResult.CallID)
	require.NotNil(t, toolResult.Output)
	require.Equal(t, "contents of the requested file", lo.FromPtr(toolResult.Output.Text))

	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	require.Equal(t, channelRow.ID, requests[0].ChannelID)
	require.Equal(t, project.ID, requests[0].ProjectID)
}

func TestChatCompletionOrchestrator_Process_RecoversEncryptedReasoningWhenRetryPolicyDisabled(t *testing.T) {
	ctx, client, _, _, orchestrator, executor := newEncryptedReasoningRetryHarness(t, 0, 0, []executorStep{
		{err: encryptedReasoningProviderError("invalid_encrypted_content", "The encrypted content could not be decrypted or parsed.")},
		{resp: recoveredResponsesResponse()},
	})
	defer client.Close()

	require.NoError(t, orchestrator.SystemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled:              false,
		LoadBalancerStrategy: biz.LoadBalancerStrategyAdaptive,
	}))

	result, err := orchestrator.Process(ctx, encryptedReasoningRequest())
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Len(t, executor.requests, 2, "provider-directed recovery must not consume or depend on ordinary retry budget")

	recovered := decodeResponsesRequest(t, executor.requests[1])
	reasoning := findResponsesItem(t, recovered.Input.Items, "reasoning")
	require.Empty(t, reasoning.ID)
	require.Nil(t, reasoning.EncryptedContent)
	require.NotContains(t, responsesItemTypes(recovered.Input.Items), "compaction")
	require.NotContains(t, responsesItemTypes(recovered.Input.Items), "compaction_summary")
}

func TestChatCompletionOrchestrator_Process_DoesNotLoopEncryptedReasoningRecovery(t *testing.T) {
	ctx, client, _, _, orchestrator, executor := newEncryptedReasoningRetryHarness(t, 0, 2, []executorStep{
		{err: encryptedReasoningProviderError("thinking_signature_invalid", "thinking_signature_invalid")},
		{err: encryptedReasoningProviderError("invalid_encrypted_content", "The encrypted content could not be decrypted or parsed.")},
		{resp: recoveredResponsesResponse()},
	})
	defer client.Close()

	_, err := orchestrator.Process(ctx, encryptedReasoningRequest())
	require.Error(t, err)
	require.Len(t, executor.requests, 2, "encrypted reasoning recovery must run at most once per request")

	second := decodeResponsesRequest(t, executor.requests[1])
	reasoning := findResponsesItem(t, second.Input.Items, "reasoning")
	require.Empty(t, reasoning.ID)
	require.Nil(t, reasoning.EncryptedContent)
	require.NotContains(t, responsesItemTypes(second.Input.Items), "compaction")
}

func TestChatCompletionOrchestrator_Process_DropsOpaqueReasoningBeforeCrossChannelRetry(t *testing.T) {
	ctx := authz.WithTestBypass(context.Background())
	client := enttest.NewEntClient(t, "sqlite3", "file:encrypted_reasoning_cross_channel?mode=memory&_fk=0")
	ctx = ent.NewContext(ctx, client)
	defer client.Close()

	_ = createTestProject(t, ctx, client)
	firstChannel, err := client.Channel.Create().
		SetType(channel.TypeOpenaiResponses).
		SetName("Encrypted Reasoning First Channel").
		SetBaseURL("https://first.example.test/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-5.6"}).
		SetDefaultTestModel("gpt-5.6").
		SetSettings(&objects.ChannelSettings{PassThroughBody: lo.ToPtr(true)}).
		Save(ctx)
	require.NoError(t, err)
	secondChannel, err := client.Channel.Create().
		SetType(channel.TypeOpenaiResponses).
		SetName("Encrypted Reasoning Second Channel").
		SetBaseURL("https://second.example.test/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-5.6"}).
		SetDefaultTestModel("gpt-5.6").
		SetSettings(&objects.ChannelSettings{PassThroughBody: lo.ToPtr(true)}).
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	require.NoError(t, systemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled:                 true,
		MaxChannelRetries:       1,
		MaxSingleChannelRetries: 0,
		RetryDelayMs:            0,
		LoadBalancerStrategy:    biz.LoadBalancerStrategyAdaptive,
	}))

	firstOutbound, err := responses.NewOutboundTransformer(firstChannel.BaseURL, firstChannel.Credentials.APIKey)
	require.NoError(t, err)
	secondOutbound, err := responses.NewOutboundTransformer(secondChannel.BaseURL, secondChannel.Credentials.APIKey)
	require.NoError(t, err)
	selector := &staticChannelSelector{candidates: []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: firstChannel, Outbound: firstOutbound}, Models: []biz.ChannelModelEntry{{RequestModel: "gpt-5.6", ActualModel: "gpt-5.6"}}},
		{Channel: &biz.Channel{Channel: secondChannel, Outbound: secondOutbound}, Models: []biz.ChannelModelEntry{{RequestModel: "gpt-5.6", ActualModel: "gpt-5.6"}}},
	}}
	executor := &encryptedReasoningSequenceExecutor{steps: []executorStep{
		{err: &httpclient.Error{StatusCode: http.StatusInternalServerError, Body: []byte(`{"error":{"message":"temporary upstream failure","type":"server_error"}}`)}},
		{resp: recoveredResponsesResponse()},
	}}
	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:       selector,
		Inbound:               responses.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
	}

	result, err := orchestrator.Process(ctx, encryptedReasoningRequest())
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Contains(t, string(result.ChatCompletion.Body), `"x_recovery_passthrough":true`,
		"cross-channel request sanitization must not disable the next channel's response pass-through")
	require.Len(t, executor.requests, 2)
	second := decodeResponsesRequest(t, executor.requests[1])
	reasoning := findResponsesItem(t, second.Input.Items, "reasoning")
	require.Empty(t, reasoning.ID)
	require.Nil(t, reasoning.EncryptedContent)
	require.Len(t, reasoning.Summary, 1)
	require.Equal(t, "I inspected the repository.", reasoning.Summary[0].Text)
	require.NotContains(t, responsesItemTypes(second.Input.Items), "compaction")
	require.NotContains(t, responsesItemTypes(second.Input.Items), "compaction_summary")
	require.NotContains(t, string(executor.requests[1]), "gAAAA_replay_legacy")

	toolCall := findResponsesItem(t, second.Input.Items, "function_call")
	require.Equal(t, "fc_replay_legacy", toolCall.ID)
	require.Equal(t, "call_replay_legacy", toolCall.CallID)
	require.Equal(t, "read_file", toolCall.Name)

	toolResult := findResponsesItem(t, second.Input.Items, "function_call_output")
	require.Equal(t, "fco_replay_legacy", toolResult.ID)
	require.Equal(t, "call_replay_legacy", toolResult.CallID)
	require.NotNil(t, toolResult.Output)
	require.Equal(t, "contents of the requested file", lo.FromPtr(toolResult.Output.Text))
}

func TestChatCompletionOrchestrator_Process_DoesNotRecoverForUnrelatedBadRequest(t *testing.T) {
	ctx, client, _, _, orchestrator, executor := newEncryptedReasoningRetryHarness(t, 0, 1, []executorStep{
		{err: encryptedReasoningProviderError("invalid_tool", "A tool definition is invalid.")},
		{resp: recoveredResponsesResponse()},
	})
	defer client.Close()

	_, err := orchestrator.Process(ctx, encryptedReasoningRequest())
	require.Error(t, err)
	require.Len(t, executor.requests, 1, "ordinary 400 errors must not trigger encrypted-state recovery")
}

func TestChatCompletionOrchestrator_Process_DoesNotRecoverSummaryOnlyReasoning(t *testing.T) {
	ctx, client, _, _, orchestrator, executor := newEncryptedReasoningRetryHarness(t, 0, 0, []executorStep{
		{err: encryptedReasoningProviderError("invalid_encrypted_content", "The encrypted content could not be decrypted or parsed.")},
		{resp: recoveredResponsesResponse()},
	})
	defer client.Close()

	_, err := orchestrator.Process(ctx, summaryOnlyReasoningRequest())
	require.Error(t, err)
	require.Len(t, executor.requests, 1, "a Responses presence marker without id or ciphertext is not opaque state")
}

func TestPersistentOutboundTransformer_CanRecoverRequiresUpstreamProviderError(t *testing.T) {
	state := &PersistenceState{
		LlmRequest: &llm.Request{Messages: []llm.Message{{
			ReasoningContent:        lo.ToPtr("visible summary"),
			ReasoningSignature:      lo.ToPtr("opaque encrypted state"),
			ResponseReasoningItemID: lo.ToPtr("rs_provider"),
		}}},
	}
	outbound := &PersistentOutboundTransformer{state: state}
	providerError := &llm.ResponseError{
		StatusCode: http.StatusBadRequest,
		Detail: llm.ErrorDetail{
			Code:    "invalid_encrypted_content",
			Message: "The encrypted content could not be decrypted or parsed.",
		},
	}

	require.False(t, outbound.CanRecover(providerError), "local errors must not trigger provider-state recovery")
	require.True(t, outbound.CanRecover(pipeline.WrapUpstreamError(providerError)))
}

func TestPersistentOutboundTransformer_PrepareForRetry_DropsOpaqueStateOnActualModelSwitch(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "same-channel-multi-model",
		},
		Outbound: &mockTransformer{},
	}
	request := &llm.Request{Messages: []llm.Message{{
		Role:                    "assistant",
		ReasoningContent:        lo.ToPtr("visible summary"),
		ReasoningSignature:      lo.ToPtr("gAAAA_model_bound"),
		ResponseReasoningItemID: lo.ToPtr("rs_model_bound"),
		Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{{
			Type: "compaction",
			Compact: &llm.CompactContent{
				ID:               "cmp_model_bound",
				EncryptedContent: "gAAAA_compaction_model_bound",
			},
		}}},
	}}}

	processor := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: channel,
				Models: []biz.ChannelModelEntry{
					{RequestModel: "gpt-5.6", ActualModel: "gpt-5.6"},
					{RequestModel: "gpt-5.6", ActualModel: "gpt-5.6-alt"},
				},
			},
			CurrentModelIndex: 0,
			LlmRequest:        request,
			RequestExec:       &ent.RequestExecution{ID: 1},
		},
	}

	require.NoError(t, processor.PrepareForRetry(ctx))
	require.Equal(t, 1, processor.state.CurrentModelIndex)
	require.True(t, processor.state.OpaqueReasoningStateDropped,
		"ActualModel advance is an issuer boundary and must drop opaque reasoning state")
	require.Nil(t, processor.state.LlmRequest.Messages[0].ReasoningSignature)
	require.NotNil(t, processor.state.LlmRequest.Messages[0].ResponseReasoningItemID)
	require.Empty(t, *processor.state.LlmRequest.Messages[0].ResponseReasoningItemID)
	require.Equal(t, "visible summary", lo.FromPtr(processor.state.LlmRequest.Messages[0].ReasoningContent))
	require.Empty(t, processor.state.LlmRequest.Messages[0].Content.MultipleContent,
		"compaction blobs must be stripped with opaque reasoning state")
}

func TestPersistentOutboundTransformer_PrepareForRetry_KeepsOpaqueStateOnSameModelRetry(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "same-channel-same-model",
		},
		Outbound: &mockTransformer{},
	}
	request := &llm.Request{Messages: []llm.Message{{
		Role:                    "assistant",
		ReasoningContent:        lo.ToPtr("visible summary"),
		ReasoningSignature:      lo.ToPtr("gAAAA_same_model"),
		ResponseReasoningItemID: lo.ToPtr("rs_same_model"),
	}}}

	processor := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: channel,
				Models: []biz.ChannelModelEntry{
					{RequestModel: "gpt-5.6", ActualModel: "gpt-5.6"},
				},
			},
			CurrentModelIndex: 0,
			LlmRequest:        request,
			RequestExec:       &ent.RequestExecution{ID: 1},
		},
	}

	require.NoError(t, processor.PrepareForRetry(ctx))
	require.Zero(t, processor.state.CurrentModelIndex)
	require.False(t, processor.state.OpaqueReasoningStateDropped)
	require.Equal(t, "gAAAA_same_model", lo.FromPtr(processor.state.LlmRequest.Messages[0].ReasoningSignature))
	require.Equal(t, "rs_same_model", lo.FromPtr(processor.state.LlmRequest.Messages[0].ResponseReasoningItemID))
}

func TestPersistentOutboundTransformer_PrepareForRecovery_StripsCompactInputOpaqueState(t *testing.T) {
	// /responses/compact carries conversation state in Compact.Input. Provider
	// rejection of opaque reasoning must clean that slice the same way as chat
	// Messages without inventing a separate compact-only recovery path.
	request := &llm.Request{
		RequestType: llm.RequestTypeCompact,
		APIFormat:   llm.APIFormatOpenAIResponseCompact,
		Compact: &llm.CompactRequest{
			Input: []llm.Message{
				{
					Role:                    "assistant",
					ReasoningContent:        lo.ToPtr("compact visible summary"),
					ReasoningSignature:      lo.ToPtr("gAAAA_compact_input"),
					ResponseReasoningItemID: lo.ToPtr("rs_compact_input"),
				},
				{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("continue after compact")},
				},
			},
		},
	}
	processor := &PersistentOutboundTransformer{
		state: &PersistenceState{
			LlmRequest: request,
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "compact-channel"}},
				Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-5.6", ActualModel: "gpt-5.6"}},
			},
		},
	}

	require.True(t, processor.HasRecoverableRequestState())
	require.NoError(t, processor.PrepareForRecovery(context.Background()))
	require.True(t, processor.state.EncryptedReasoningRecoveryUsed)
	require.True(t, processor.state.OpaqueReasoningStateDropped)
	require.Nil(t, processor.state.LlmRequest.Compact.Input[0].ReasoningSignature)
	require.NotNil(t, processor.state.LlmRequest.Compact.Input[0].ResponseReasoningItemID)
	require.Empty(t, *processor.state.LlmRequest.Compact.Input[0].ResponseReasoningItemID)
	require.Equal(t, "compact visible summary", lo.FromPtr(processor.state.LlmRequest.Compact.Input[0].ReasoningContent))
	require.Equal(t, "continue after compact", lo.FromPtr(processor.state.LlmRequest.Compact.Input[1].Content.Content))
}

func TestStripOpaqueReasoningState_PreservesNonOpaqueResponsesPresence(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{{
		Role:                    "assistant",
		ResponseReasoningItemID: lo.ToPtr(""),
	}}}

	require.False(t, hasOpaqueReasoningState(request))
	require.False(t, stripOpaqueReasoningState(request))
	require.NotNil(t, request.Messages[0].ResponseReasoningItemID,
		"cleanup must not remove a Responses reasoning item that has no id, ciphertext, or compaction state")
	require.Empty(t, *request.Messages[0].ResponseReasoningItemID)
}

func TestIsEncryptedReasoningFailure_RecognizesNarrowProviderSignals(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		errorType string
		message   string
		expected  bool
	}{
		{name: "structured invalid encrypted content code", code: "invalid_encrypted_content", expected: true},
		{name: "structured thinking signature type", errorType: "thinking_signature_invalid", expected: true},
		{name: "message-only invalid encrypted content code", message: "invalid_encrypted_content", expected: true},
		{name: "item id mismatch text", message: "The encrypted content could not be verified because item_id did not match the target item id", expected: true},
		{name: "ordinary invalid request", code: "invalid_tool", message: "A tool definition is invalid", expected: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, isEncryptedReasoningFailure(test.code, test.errorType, test.message))
		})
	}
}

func newEncryptedReasoningRetryHarness(
	t *testing.T,
	maxChannelRetries int,
	maxSameChannelRetries int,
	steps []executorStep,
) (context.Context, *ent.Client, *ent.Project, *ent.Channel, *ChatCompletionOrchestrator, *encryptedReasoningSequenceExecutor) {
	t.Helper()

	ctx := authz.WithTestBypass(context.Background())
	client := enttest.NewEntClient(t, "sqlite3", "file:encrypted_reasoning_retry?mode=memory&_fk=0")
	ctx = ent.NewContext(ctx, client)
	project := createTestProject(t, ctx, client)
	channelRow, err := client.Channel.Create().
		SetType(channel.TypeOpenaiResponses).
		SetName("Encrypted Reasoning Channel").
		SetBaseURL("https://responses.example.test/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-5.6"}).
		SetDefaultTestModel("gpt-5.6").
		SetSettings(&objects.ChannelSettings{PassThroughBody: lo.ToPtr(true)}).
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	require.NoError(t, systemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled:                 true,
		MaxChannelRetries:       maxChannelRetries,
		MaxSingleChannelRetries: maxSameChannelRetries,
		RetryDelayMs:            0,
		LoadBalancerStrategy:    biz.LoadBalancerStrategyAdaptive,
	}))

	outbound, err := responses.NewOutboundTransformer(channelRow.BaseURL, channelRow.Credentials.APIKey)
	require.NoError(t, err)
	selector := &staticChannelSelector{candidates: []*ChannelModelsCandidate{{
		Channel:  &biz.Channel{Channel: channelRow, Outbound: outbound},
		Priority: 0,
		Models:   []biz.ChannelModelEntry{{RequestModel: "gpt-5.6", ActualModel: "gpt-5.6"}},
	}}}
	executor := &encryptedReasoningSequenceExecutor{steps: steps}
	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:       selector,
		Inbound:               responses.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
	}

	return contexts.WithProjectID(ctx, project.ID), client, project, channelRow, orchestrator, executor
}

func encryptedReasoningRequest() *httpclient.Request {
	return &httpclient.Request{
		Method: "POST",
		URL:    "/v1/responses",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{
			"model":"gpt-5.6",
			"stream":false,
			"input":[
				{"type":"message","role":"user","content":[{"type":"input_text","text":"inspect the repository"}]},
				{"id":"rs_replay_legacy","type":"reasoning","summary":[{"type":"summary_text","text":"I inspected the repository."}],"encrypted_content":"gAAAA_replay_legacy"},
				{"id":"fc_replay_legacy","type":"function_call","call_id":"call_replay_legacy","name":"read_file","arguments":"{\"path\":\"README.md\"}"},
				{"id":"fco_replay_legacy","type":"function_call_output","call_id":"call_replay_legacy","output":"contents of the requested file"},
				{"id":"cmp_replay_legacy","type":"compaction","encrypted_content":"gAAAA_compaction_legacy","created_by":"model"},
				{"id":"cmps_replay_legacy","type":"compaction_summary","encrypted_content":"gAAAA_compaction_summary_legacy","created_by":"model"},
				{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
			],
			"reasoning":{"summary":"auto"}
		}`),
	}
}

func summaryOnlyReasoningRequest() *httpclient.Request {
	return &httpclient.Request{
		Method: "POST",
		URL:    "/v1/responses",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{
			"model":"gpt-5.6",
			"stream":false,
			"input":[
				{"type":"message","role":"user","content":[{"type":"input_text","text":"inspect the repository"}]},
				{"type":"reasoning","summary":[{"type":"summary_text","text":"I inspected the repository."}]},
				{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
			]
		}`),
	}
}

func encryptedReasoningProviderError(code, message string) error {
	return &httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"error":{"message":"` + message + `","type":"invalid_request_error","code":"` + code + `"}}`),
	}
}

func recoveredResponsesResponse() *httpclient.Response {
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"id":"resp_recovered",
			"object":"response",
			"created_at":1784548800,
			"status":"completed",
			"model":"gpt-5.6",
			"x_recovery_passthrough":true,
			"output":[{"id":"msg_recovered","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Recovered.","annotations":[]}]}],
			"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`),
	}
}

func decodeResponsesRequest(t *testing.T, body []byte) responses.Request {
	t.Helper()
	var request responses.Request
	require.NoError(t, json.Unmarshal(body, &request))
	return request
}

func responsesItemTypes(items []responses.Item) []string {
	types := make([]string, len(items))
	for index, item := range items {
		types[index] = item.Type
	}
	return types
}

func findResponsesItem(t *testing.T, items []responses.Item, itemType string) responses.Item {
	t.Helper()
	for _, item := range items {
		if item.Type == itemType {
			return item
		}
	}
	t.Fatalf("missing Responses item type %q in %s", itemType, strings.Join(responsesItemTypes(items), ", "))
	return responses.Item{}
}
