package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
)

func TestUnsupportedImageFallbackMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		vision     bool
		delegation bool
		wantMarker bool
	}{
		{name: "enabled text-only model", enabled: true, wantMarker: true},
		{name: "disabled", enabled: false, wantMarker: false},
		{name: "native vision", enabled: true, vision: true, wantMarker: false},
		{name: "delegated vision", enabled: true, delegation: true, wantMarker: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &PersistenceState{SourceModel: &ent.Model{
				ModelID: "source-model",
				ModelCard: &objects.ModelCard{
					Vision: tt.vision,
					Modalities: objects.ModelCardModalities{
						Input: []string{"text"},
					},
				},
				Settings: &objects.ModelSettings{
					VisionDelegation: objects.VisionDelegation{Enabled: tt.delegation},
					UnsupportedImageFallback: objects.UnsupportedImageFallback{
						Enabled: lo.ToPtr(tt.enabled),
					},
				},
			}}
			request := requestWithImage("Describe this image")
			middleware := unsupportedImageFallback(&PersistentInboundTransformer{state: state})

			result, err := middleware.OnInboundLlmRequest(context.Background(), request)
			require.NoError(t, err)
			require.Same(t, request, result)
			if tt.wantMarker {
				require.False(t, detectRequestContentFeatures(result).hasImage)
				require.Contains(t, messageText(result.Messages[0]), unsupportedImageMarker)
				require.True(t, state.DisableRequestBodyPassThrough)
			} else {
				require.True(t, detectRequestContentFeatures(result).hasImage)
				require.False(t, state.DisableRequestBodyPassThrough)
			}
		})
	}
}

func TestReplaceImagesWithUnsupportedMarkerPreservesMessageShape(t *testing.T) {
	toolCallID := "tool-call-1"
	request := &llm.Request{Messages: []llm.Message{
		{
			Role: "user",
			Content: llm.MessageContent{
				Content: lo.ToPtr("[Image: source: /tmp/screenshot.png]\nWhat failed?"),
			},
		},
		{
			Role: "user",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "text", Text: lo.ToPtr("Keep this question")},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: "data:image/png;base64,AAAA"}},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"}},
			}},
		},
		{
			Role:       "tool",
			ToolCallID: &toolCallID,
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "text", Text: lo.ToPtr("[Image: source: /tmp/tool.png]")},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: "data:image/png;base64,BBBB"}},
			}},
		},
	}}

	require.True(t, replaceImagesWithUnsupportedMarker(request))
	require.False(t, detectRequestContentFeatures(request).hasImage)
	require.Equal(t, "What failed?", messageText(request.Messages[0]))
	require.Equal(t, "Keep this question\n[Unsupported Image]\n[Unsupported Image]", messageText(request.Messages[1]))
	require.Equal(t, toolCallID, *request.Messages[2].ToolCallID)
	require.Equal(t, unsupportedImageMarker, messageText(request.Messages[2]))
}

func TestIsUnsupportedImageInputError(t *testing.T) {
	unsupported := func(status int, code, message string) error {
		return pipeline.WrapUpstreamError(&llm.ResponseError{
			StatusCode: status,
			Detail:     llm.ErrorDetail{Code: code, Message: message},
		})
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "explicit rejection", err: unsupported(http.StatusBadRequest, "", "this model does not support image input"), want: true},
		{name: "unsupported vision", err: unsupported(http.StatusUnsupportedMediaType, "", "vision input is unsupported"), want: true},
		{name: "provider capability code", err: unsupported(http.StatusUnprocessableEntity, "image_input_not_supported", "invalid input"), want: true},
		{name: "chinese text only rejection", err: unsupported(http.StatusBadRequest, "", "该模型仅支持文本"), want: true},
		{name: "bare response error", err: &llm.ResponseError{StatusCode: http.StatusBadRequest, Detail: llm.ErrorDetail{Message: "image unsupported"}}},
		{name: "generic cannot process", err: unsupported(http.StatusUnprocessableEntity, "", "cannot process image")},
		{name: "invalid image url", err: unsupported(http.StatusBadRequest, "invalid_image_url", "image URL is not allowed")},
		{name: "malformed media", err: unsupported(http.StatusUnsupportedMediaType, "unsupported_media_type", "cannot process malformed image")},
		{name: "policy rejection", err: unsupported(http.StatusBadRequest, "content_policy_violation", "image content is not allowed")},
		{name: "unrelated bad request", err: unsupported(http.StatusBadRequest, "", "temperature is invalid")},
		{name: "authentication", err: unsupported(http.StatusUnauthorized, "", "image input is unsupported")},
		{name: "rate limit", err: unsupported(http.StatusTooManyRequests, "", "image input is unsupported")},
		{name: "server error", err: unsupported(http.StatusInternalServerError, "", "image input is unsupported")},
		{name: "timeout", err: pipeline.ErrNonStreamResponseTimeout},
		{name: "generic error", err: errors.New("image input is unsupported")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isUnsupportedImageInputError(tt.err))
		})
	}
}

func TestPrepareUnsupportedImageFallbackValidatesSelectorBeforeRewrite(t *testing.T) {
	state := &PersistenceState{}
	request := requestWithImage("Describe this image")
	outbound := &PersistentOutboundTransformer{state: state}

	err := outbound.PrepareFallback(t.Context(), request)
	require.ErrorContains(t, err, "candidate selector is unavailable")
	require.True(t, detectRequestContentFeatures(request).hasImage)
	require.False(t, state.DisableRequestBodyPassThrough)
}

func TestTextOnlyUnsupportedImageFallbackRunsBeforePrimaryCall(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary", "text-model", "Image input was unavailable.", 8, 4))},
	}, &biz.RetryPolicy{Enabled: false})
	updateSourceImageHandling(t, fixture, false, false, true)

	selector := fixture.orchestrator.channelSelector.(*visionDelegationTestSelector)
	selector.candidates["text-model"][0].Channel.Settings = &objects.ChannelSettings{PassThroughBody: lo.ToPtr(true)}

	_, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 1)
	require.NotContains(t, string(requests[0].Body), "image_url")
	require.Contains(t, string(requests[0].Body), unsupportedImageMarker)

	execution, err := fixture.client.RequestExecution.Query().Only(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, requestexecution.PurposePrimary, execution.Purpose)
	require.False(t, execution.PassThroughApplied)
}

func TestVisionDelegationUnsupportedImageFallsBackToPrimary(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{err: unsupportedImageHTTPError()},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary", "text-model", "Image input was unavailable.", 8, 4))},
	}, &biz.RetryPolicy{Enabled: false})
	updateSourceImageHandling(t, fixture, true, false, true)

	_, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 2)
	require.Contains(t, string(requests[0].Body), "image_url")
	require.NotContains(t, string(requests[1].Body), "image_url")
	require.Contains(t, string(requests[1].Body), unsupportedImageMarker)
	require.NotContains(t, string(requests[1].Body), visionEvidenceStart)

	executions, err := fixture.client.RequestExecution.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Len(t, executions, 2)
	require.ElementsMatch(t, []requestexecution.Purpose{
		requestexecution.PurposeVisionDelegation,
		requestexecution.PurposePrimary,
	}, lo.Map(executions, func(execution *ent.RequestExecution, _ int) requestexecution.Purpose {
		return execution.Purpose
	}))
}

func TestNativeVisionUpstreamUnsupportedImageRetriesWithMarker(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{err: unsupportedImageHTTPError()},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary", "text-model", "Image input was unavailable.", 8, 4))},
	}, &biz.RetryPolicy{Enabled: false})
	updateSourceImageHandling(t, fixture, false, true, true)

	_, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 2)
	require.Contains(t, string(requests[0].Body), "image_url")
	require.NotContains(t, string(requests[1].Body), "image_url")
	require.Contains(t, string(requests[1].Body), unsupportedImageMarker)
}

func TestNativeVisionStreamingUpstreamUnsupportedImageRetriesWithMarker(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{err: unsupportedImageHTTPError()},
		{streamEvents: []*httpclient.StreamEvent{
			{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"text-model","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)},
			{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"text-model","choices":[{"index":0,"delta":{"content":"Image input was unavailable."},"finish_reason":null}]}`)},
			{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"text-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":4,"total_tokens":12}}`)},
		}},
	}, &biz.RetryPolicy{Enabled: false})
	updateSourceImageHandling(t, fixture, false, true, true)

	result, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequestWithStream(t, true))
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletionStream)
	for result.ChatCompletionStream.Next() {
	}
	require.NoError(t, result.ChatCompletionStream.Err())

	requests := fixture.executor.Requests()
	require.Len(t, requests, 2)
	require.Contains(t, string(requests[0].Body), "image_url")
	require.NotContains(t, string(requests[1].Body), "image_url")
	require.Contains(t, string(requests[1].Body), unsupportedImageMarker)
}

func TestNativeVisionUnrelatedBadRequestDoesNotFallback(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{{err: &httpclient.Error{
		Method:     http.MethodPost,
		URL:        "https://allowed.example.com/v1/chat/completions",
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       []byte(`{"error":{"message":"temperature is invalid"}}`),
	}}}, &biz.RetryPolicy{Enabled: false})
	updateSourceImageHandling(t, fixture, false, true, true)

	_, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.Error(t, err)
	require.Len(t, fixture.executor.Requests(), 1)
}

func requestWithImage(question string) *llm.Request {
	return &llm.Request{Messages: []llm.Message{{
		Role: "user",
		Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
			{Type: "text", Text: lo.ToPtr(question)},
			{Type: "image_url", ImageURL: &llm.ImageURL{URL: "data:image/png;base64,AAAA"}},
		}},
	}}}
}

func unsupportedImageHTTPError() error {
	return &httpclient.Error{
		Method:     http.MethodPost,
		URL:        "https://allowed.example.com/v1/chat/completions",
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       []byte(`{"error":{"message":"this model does not support image input"}}`),
	}
}

func updateSourceImageHandling(
	t *testing.T,
	fixture *visionDelegationPipelineFixture,
	delegationEnabled bool,
	nativeVision bool,
	fallbackEnabled bool,
) {
	t.Helper()

	source, err := fixture.client.Model.Query().Where(model.ModelIDEQ("text-model")).Only(fixture.ctx)
	require.NoError(t, err)
	source.Settings.VisionDelegation.Enabled = delegationEnabled
	source.Settings.UnsupportedImageFallback.Enabled = lo.ToPtr(fallbackEnabled)
	source.ModelCard.Vision = nativeVision
	if nativeVision {
		source.ModelCard.Modalities.Input = []string{"text", "image"}
	} else {
		source.ModelCard.Modalities.Input = []string{"text"}
	}
	_, err = source.Update().SetSettings(source.Settings).SetModelCard(source.ModelCard).Save(fixture.ctx)
	require.NoError(t, err)
}
