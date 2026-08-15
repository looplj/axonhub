package orchestrator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/intercept"
	"github.com/looplj/axonhub/internal/ent/model"
	requestent "github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestCollectVisionDelegationInputAndReplaceEvidence(t *testing.T) {
	png := base64TestImage("same-image")
	request := &llm.Request{
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("Read the screenshot")},
					{Type: "image_url", ImageURL: &llm.ImageURL{URL: png}},
					{Type: "image_url", ImageURL: &llm.ImageURL{URL: "c2FtZS1pbWFnZQ==", MIMEType: "image/png"}},
					{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/chart.png#fragment"}},
				}},
			},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("What is the total?")}},
		},
	}

	input, err := collectVisionDelegationInput(request)
	require.NoError(t, err)
	require.Len(t, input.images, 2)
	require.Equal(t, visionImagePosition{message: 0, part: 3}, input.evidenceImage)
	require.Equal(t, []visionContextTurn{
		{Role: "user", Text: "Read the screenshot"},
		{Role: "user", Text: "What is the total?"},
	}, input.visualContext)
	require.Equal(t, "https://example.com/chart.png", input.images[1].ImageURL.URL)

	messages := buildVisionDelegationMessages(input)
	require.Len(t, messages, 2)
	require.Equal(t, "system", messages[0].Role)
	require.Contains(t, lo.FromPtr(messages[0].Content.Content), "untrusted data")
	require.Contains(t, messageText(messages[1]), "Inspect every supplied image directly")
	require.Contains(t, messageText(messages[1]), "Read the screenshot")
	require.Contains(t, messageText(messages[1]), "What is the total?")
	require.Len(t, messages[1].Content.MultipleContent, 6)

	replaceImagesWithVisionEvidence(request, input.evidenceImage, "The table total is 42.")
	serialized, err := json.Marshal(request)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "image_url")
	evidenceCount := 0
	for _, message := range request.Messages {
		evidenceCount += strings.Count(messageText(message), visionEvidenceStart)
	}
	require.Equal(t, 1, evidenceCount)
	require.Contains(t, string(serialized), "The table total is 42.")
}

func TestReplaceImagesWithVisionEvidenceUsesLatestImagePosition(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{
			Role: "user",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "text", Text: lo.ToPtr("What is in the top right?")},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: base64TestImage("same-image")}},
			}},
		},
		{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("The badge is red.")}},
		{
			Role: "user",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "text", Text: lo.ToPtr("What is in the top right?")},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: base64TestImage("same-image")}},
			}},
		},
	}}

	input, err := collectVisionDelegationInput(request)
	require.NoError(t, err)
	require.Equal(t, visionImagePosition{message: 2, part: 1}, input.evidenceImage)

	replaceImagesWithVisionEvidence(request, input.evidenceImage, "The badge is red.")
	require.NotContains(t, messageText(request.Messages[1]), visionEvidenceStart)
	require.Contains(t, messageText(request.Messages[3]), visionEvidenceStart)
	require.NotContains(t, messageText(request.Messages[1]), "image_url")
	require.NotContains(t, messageText(request.Messages[3]), "image_url")
}

func TestReplaceImagesWithVisionEvidenceInjectsPolicyAndStripsMarkers(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("You are a helpful assistant.")}},
		{
			Role: "user",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "text", Text: lo.ToPtr("What is in the top right?")},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: base64TestImage("same-image")}},
				{Type: "text", Text: lo.ToPtr("[Image: source: /Users/tester/error.png]")},
				{Type: "text", Text: lo.ToPtr("Mentioning [Image: source: inline] inside prose is kept")},
			}},
		},
	}}

	input, err := collectVisionDelegationInput(request)
	require.NoError(t, err)

	replaceImagesWithVisionEvidence(request, input.evidenceImage, "The badge is red.")

	serialized, err := json.Marshal(request)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "image_url")
	require.NotContains(t, string(serialized), "/Users/tester/error.png")
	require.NotContains(t, string(serialized), "untrusted visual evidence")
	require.Contains(t, string(serialized), "inline] inside prose is kept")

	require.Equal(t, "system", request.Messages[0].Role)
	require.Contains(t, messageText(request.Messages[0]), "already been inspected by AxonHub")
	require.Contains(t, messageText(request.Messages[0]), "AXONHUB_VISION_EVIDENCE")
	require.Equal(t, "system", request.Messages[1].Role)
	require.Equal(t, "You are a helpful assistant.", messageText(request.Messages[1]))
}

func TestStripImageSourceMarkerPreservesFollowingText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standalone marker",
			input:    "[Image: source: /tmp/error.png]",
			expected: "",
		},
		{
			name:     "marker followed by a question",
			input:    "[Image: source: /tmp/error.png]\nPlease describe the error",
			expected: "Please describe the error",
		},
		{
			name:     "marker and same line question",
			input:    "[Image: source: /tmp/error.png] Please describe the error",
			expected: "Please describe the error",
		},
		{
			name:     "unclosed marker",
			input:    "[Image: source: /tmp/error.png",
			expected: "[Image: source: /tmp/error.png",
		},
		{
			name:     "empty source is not a marker",
			input:    "[Image: source: ]\nPlease describe the error",
			expected: "[Image: source: ]\nPlease describe the error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, stripImageSourceMarker(tt.input))
		})
	}
}

func TestVisionImageSourceMarkerCleanupHandlesScalarContent(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr(
			"[Image: source: /tmp/error.png]\nPlease describe the error",
		)}},
		{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr(
			"[Image: source: /tmp/standalone.png]",
		)}},
	}}

	replaceImagesWithVisionEvidence(request, visionImagePosition{message: -1, part: -1}, "The screenshot shows error E102.")
	require.Equal(t, "Please describe the error", messageText(request.Messages[1]))
	require.Equal(t, "", messageText(request.Messages[2]))

	serialized, err := json.Marshal(request)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "/tmp/error.png")
	require.NotContains(t, string(serialized), "/tmp/standalone.png")
}

func TestRemoveVisionImagesCleansScalarSourceMarkers(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr(
			"[Image: source: /tmp/error.png]\nPlease describe the error",
		)}},
		{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr(
			"[Image: source: /tmp/unclosed.png",
		)}},
	}}

	require.True(t, removeVisionImages(request))
	require.Equal(t, "Please describe the error", messageText(request.Messages[0]))
	require.Equal(t, "[Image: source: /tmp/unclosed.png", messageText(request.Messages[1]))

	serialized, err := json.Marshal(request)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "/tmp/error.png")
	require.Contains(t, string(serialized), "/tmp/unclosed.png")
}

func TestVisionDelegationContextFiltersClientNoiseAndUsesPreviousAssistant(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{
			Role:    "assistant",
			Content: llm.MessageContent{Content: lo.ToPtr("Please upload a screenshot of the complete error message.")},
		},
		{
			Role: "user",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "text", Text: lo.ToPtr("<system-reminder>secret client instructions</system-reminder>")},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: base64TestImage("error-screenshot")}},
				{Type: "text", Text: lo.ToPtr("[Image #1]\nRead the exact error text and line number.")},
				{Type: "text", Text: lo.ToPtr("[Image: source: /tmp/error.png]")},
			}},
		},
	}}

	input, err := collectVisionDelegationInput(request)
	require.NoError(t, err)
	require.Equal(t, []visionContextTurn{
		{Role: "assistant", Text: "Please upload a screenshot of the complete error message."},
		{Role: "user", Text: "Read the exact error text and line number."},
	}, input.visualContext)

	messages := buildVisionDelegationMessages(input)
	serialized, err := json.Marshal(messages)
	require.NoError(t, err)
	require.Contains(t, string(serialized), "Please upload a screenshot")
	require.Contains(t, string(serialized), "Read the exact error text")
	require.NotContains(t, string(serialized), "secret client instructions")
	require.NotContains(t, string(serialized), "/tmp/error.png")
}

func TestVisionDelegationContextUsesPreviousAssistantForImageOnlyMessage(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{
			Role:    "assistant",
			Content: llm.MessageContent{Content: lo.ToPtr("Please upload a screenshot of the complete error message.")},
		},
		{
			Role: "user",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: base64TestImage("error-screenshot")}},
			}},
		},
	}}

	input, err := collectVisionDelegationInput(request)
	require.NoError(t, err)
	require.Equal(t, []visionContextTurn{{
		Role: "assistant",
		Text: "Please upload a screenshot of the complete error message.",
	}}, input.visualContext)

	serialized, err := json.Marshal(buildVisionDelegationMessages(input))
	require.NoError(t, err)
	require.Contains(t, string(serialized), "Please upload a screenshot of the complete error message.")
}

func TestNormalizeVisionEvidence(t *testing.T) {
	responseWithMessage := func(message *llm.Message) *llm.Response {
		return &llm.Response{Choices: []llm.Choice{{Message: message}}}
	}

	tests := []struct {
		name          string
		response      *llm.Response
		expected      string
		expectedError error
	}{
		{
			name: "strips leading reasoning blocks",
			response: responseWithMessage(&llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{Content: lo.ToPtr(
					"<think>First I should inspect the image.</think>\n" +
						"<analysis>Internal OCR planning.</analysis>\n" +
						"The screenshot shows error E102 on line 42.",
				)},
			}),
			expected: "The screenshot shows error E102 on line 42.",
		},
		{
			name: "rejects tool calls even with text",
			response: responseWithMessage(&llm.Message{
				Role:    "assistant",
				Content: llm.MessageContent{Content: lo.ToPtr("I need to inspect the local image first.")},
				ToolCalls: []llm.ToolCall{{
					Type:     "function",
					Function: llm.FunctionCall{Name: "read_file", Arguments: `{\"path\":\"/tmp/image.png\"}`},
				}},
			}),
			expectedError: errInvalidVisionDelegationResponse,
		},
		{
			name: "rejects plan without visual facts",
			response: responseWithMessage(&llm.Message{
				Role:    "assistant",
				Content: llm.MessageContent{Content: lo.ToPtr("I need to use the image-reading tool and then crop the file.")},
			}),
			expectedError: errInvalidVisionDelegationResponse,
		},
		{
			name: "rejects reasoning-only response",
			response: responseWithMessage(&llm.Message{
				Role:    "assistant",
				Content: llm.MessageContent{Content: lo.ToPtr("<think>I should inspect the image.</think>")},
			}),
			expectedError: errEmptyVisionDelegationResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence, err := normalizeVisionEvidence(tt.response)
			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
				require.Empty(t, evidence)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, evidence)
		})
	}
}

func TestReplaceImagesWithVisionEvidenceKeepsLaterToolMessageValid(t *testing.T) {
	png := base64TestImage("same-image")
	toolCallID := "call_image_result"
	request := &llm.Request{
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
					{Type: "image_url", ImageURL: &llm.ImageURL{URL: png}},
				}},
			},
			{
				Role:       "tool",
				ToolCallID: &toolCallID,
				Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
					{Type: "image_url", ImageURL: &llm.ImageURL{URL: png}},
				}},
			},
		},
	}

	input, err := collectVisionDelegationInput(request)
	require.NoError(t, err)
	require.Len(t, input.images, 1)

	replaceImagesWithVisionEvidence(request, input.evidenceImage, "The image shows a poster.")
	require.Equal(t, "", lo.FromPtr(request.Messages[2].Content.Content))
	require.Nil(t, request.Messages[2].Content.MultipleContent)

	serialized, err := json.Marshal(request)
	require.NoError(t, err)
	var payload struct {
		Messages []struct {
			Content any `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(serialized, &payload))
	require.Equal(t, "", payload.Messages[2].Content)
}

func TestCollectVisionDelegationInputRejectsFileID(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{{
		Role: "user",
		Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{{
			Type:     "image_url",
			ImageURL: &llm.ImageURL{FileID: "file_123"},
		}}},
	}}}

	_, err := collectVisionDelegationInput(request)
	require.ErrorContains(t, err, "file_id")
}

func TestCollectVisionDelegationInputIgnoresHistoricalImages(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{
			Role: "user",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
				{Type: "text", Text: lo.ToPtr("Describe this image")},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: base64TestImage("historical")}},
			}},
		},
		{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("It is a chart.")}},
		{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("What is my model?")}},
	}}

	input, err := collectVisionDelegationInput(request)
	require.NoError(t, err)
	require.Empty(t, input.images)

	require.True(t, removeVisionImages(request))
	serialized, err := json.Marshal(request)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "image_url")
	require.Contains(t, string(serialized), "Describe this image")
}

func TestCollectVisionDelegationInputIgnoresHistoricalFileID(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{Role: "user", Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{{
			Type: "text", Text: lo.ToPtr("Describe this image")}, {
			Type: "image_url", ImageURL: &llm.ImageURL{FileID: "file_historical"},
		}}}},
		{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("It is a chart.")}},
		{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("What is my model?")}},
	}}

	input, err := collectVisionDelegationInput(request)
	require.NoError(t, err)
	require.Empty(t, input.images)
}

func TestCollectVisionDelegationInputSupportsAssistantPrefill(t *testing.T) {
	request := &llm.Request{Messages: []llm.Message{
		{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Earlier question")}},
		{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Earlier answer")}},
		{Role: "user", Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
			{Type: "text", Text: lo.ToPtr("Describe this image")},
			{Type: "image_url", ImageURL: &llm.ImageURL{URL: base64TestImage("current")}},
		}}},
		{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("The image shows")}},
	}}

	input, err := collectVisionDelegationInput(request)
	require.NoError(t, err)
	require.Len(t, input.images, 1)
	require.Equal(t, visionImagePosition{message: 2, part: 1}, input.evidenceImage)
}

func TestVisionDelegationPipelinePersistsSeparateExecutionsAndUsage(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-1", "vision-model", "The screenshot shows total 42.", 7, 3))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-1", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})
	ctx := fixture.ctx
	client := fixture.client
	allowedRow := fixture.allowedRow
	executor := fixture.executor
	httpRequest := buildVisionTestRequest(t)
	result, err := fixture.orchestrator.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Contains(t, string(result.ChatCompletion.Body), "The total is 42")

	requests := executor.Requests()
	require.Len(t, requests, 2)
	require.Contains(t, requests[0].URL, "allowed.example.com")
	require.Equal(t, 1, strings.Count(string(requests[0].Body), base64TestImage("same-image")))
	require.Equal(t, 1, strings.Count(string(requests[0].Body), "https://example.com/chart.png"))
	require.Contains(t, string(requests[0].Body), `"max_completion_tokens":4096`)
	require.NotContains(t, string(requests[0].Body), `"reasoning_effort"`)
	require.NotContains(t, string(requests[0].Body), `"User-Agent"`)
	require.NotContains(t, string(requests[1].Body), "image_url")
	require.Contains(t, string(requests[1].Body), "AXONHUB_VISION_EVIDENCE")
	require.Contains(t, string(requests[1].Body), "already been inspected by AxonHub")
	require.NotContains(t, string(requests[1].Body), "untrusted visual evidence")

	dbRequests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, dbRequests, 1)
	require.Equal(t, requestent.StatusCompleted, dbRequests[0].Status)
	require.Equal(t, allowedRow.ID, dbRequests[0].ChannelID)
	require.Contains(t, string(dbRequests[0].RequestBody), "image_url")

	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, executions, 2)
	purposeByExecutionID := make(map[int]requestexecution.Purpose, len(executions))
	for _, execution := range executions {
		purposeByExecutionID[execution.ID] = execution.Purpose
		require.Equal(t, dbRequests[0].ID, execution.RequestID)
		require.Equal(t, allowedRow.ID, execution.ChannelID)
	}
	require.ElementsMatch(t, []requestexecution.Purpose{
		requestexecution.PurposePrimary,
		requestexecution.PurposeVisionDelegation,
	}, lo.Values(purposeByExecutionID))

	usageLogs, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, usageLogs, 2)
	for _, usageLog := range usageLogs {
		require.NotZero(t, usageLog.RequestExecutionID)
		_, ok := purposeByExecutionID[usageLog.RequestExecutionID]
		require.True(t, ok)
		require.Equal(t, dbRequests[0].ID, usageLog.RequestID)
	}
}

func TestVisionDelegationFollowsPrimaryReasoningAndUserAgent(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-1", "vision-model", "The screenshot shows total 42.", 7, 3))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-1", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})

	request := buildVisionTestRequest(t)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(request.Body, &payload))
	payload["reasoning_effort"] = "high"
	request.Body, _ = json.Marshal(payload)
	request.Headers.Set("User-Agent", "claude-cli/2.1.170 (external, cli)")

	_, err := fixture.orchestrator.Process(fixture.ctx, request)
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 2)
	require.Contains(t, string(requests[0].Body), `"max_completion_tokens":8192`)
	require.Contains(t, string(requests[0].Body), `"reasoning_effort":"high"`)
	require.Equal(t, "claude-cli/2.1.170 (external, cli)", requests[0].Headers.Get("User-Agent"))
	require.NotEqual(t, "axonhub/1.0", requests[0].Headers.Get("User-Agent"))
}

func TestVisionDelegationMaxCompletionTokensByReasoningEffort(t *testing.T) {
	tests := []struct {
		effort string
		want   int64
	}{
		{effort: "", want: visionDelegationMaxTokensWithoutReasoning},
		{effort: "none", want: visionDelegationMaxTokensWithoutReasoning},
		{effort: "low", want: visionDelegationMaxTokensWithoutReasoning},
		{effort: "medium", want: visionDelegationMaxTokensWithReasoning},
		{effort: "high", want: visionDelegationMaxTokensWithReasoning},
		{effort: "xhigh", want: visionDelegationMaxTokensWithReasoning},
		{effort: "max", want: visionDelegationMaxTokensWithReasoning},
	}

	for _, tt := range tests {
		t.Run(tt.effort, func(t *testing.T) {
			require.Equal(t, tt.want, visionDelegationMaxCompletionTokens(tt.effort))
		})
	}
}

func TestVisionDelegationUsesEachModelsAssociationPriority(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-1", "vision-model", "The screenshot shows total 42.", 7, 3))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-1", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})

	disallowedRow, err := fixture.client.Channel.Query().
		Where(channel.NameEQ("Disallowed")).
		Only(fixture.ctx)
	require.NoError(t, err)

	visionModel, err := fixture.client.Model.Query().
		Where(model.ModelIDEQ("vision-model")).
		Only(fixture.ctx)
	require.NoError(t, err)
	_, err = visionModel.Update().
		SetSettings(&objects.ModelSettings{Associations: []*objects.ModelAssociation{
			visionChannelAssociationWithPriority(fixture.allowedRow.ID, "vision-model", 0),
			visionChannelAssociationWithPriority(disallowedRow.ID, "vision-model", 10),
		}}).
		Save(fixture.ctx)
	require.NoError(t, err)

	textModel, err := fixture.client.Model.Query().
		Where(model.ModelIDEQ("text-model")).
		Only(fixture.ctx)
	require.NoError(t, err)
	_, err = textModel.Update().
		SetSettings(&objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				visionChannelAssociationWithPriority(disallowedRow.ID, "text-model", 0),
				visionChannelAssociationWithPriority(fixture.allowedRow.ID, "text-model", 10),
			},
			VisionDelegation: objects.VisionDelegation{
				Enabled:       true,
				TargetModelID: lo.ToPtr("vision-model"),
			},
		}).
		Save(fixture.ctx)
	require.NoError(t, err)

	key, ok := contexts.GetAPIKey(fixture.ctx)
	require.True(t, ok)
	key, err = key.Update().
		SetProfiles(&objects.APIKeyProfiles{
			ActiveProfile: "default",
			Profiles: []objects.APIKeyProfile{{
				Name:       "default",
				ModelIDs:   []string{"text-model"},
				ChannelIDs: []int{fixture.allowedRow.ID, disallowedRow.ID},
			}},
		}).
		Save(fixture.ctx)
	require.NoError(t, err)
	fixture.ctx = contexts.WithAPIKey(fixture.ctx, key)

	fixture.orchestrator.channelSelector = NewDefaultSelector(
		fixture.orchestrator.ChannelService,
		fixture.orchestrator.ModelService,
		fixture.orchestrator.SystemService,
	)

	_, err = fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 2)
	require.Contains(t, requests[0].URL, "allowed.example.com", "vision request must use the target model's highest-priority association")
	require.Contains(t, requests[1].URL, "disallowed.example.com", "primary request must keep the source model's association priority")

	executions, err := fixture.client.RequestExecution.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Len(t, executions, 2)
	for _, execution := range executions {
		switch execution.Purpose {
		case requestexecution.PurposeVisionDelegation:
			require.Equal(t, fixture.allowedRow.ID, execution.ChannelID)
		case requestexecution.PurposePrimary:
			require.Equal(t, disallowedRow.ID, execution.ChannelID)
		default:
			t.Fatalf("unexpected execution purpose: %s", execution.Purpose)
		}
	}
}

func TestVisionDelegationSelectsPrimaryRouteAfterImageRewrite(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-1", "vision-model", "The screenshot shows total 42.", 7, 3))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-1", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})

	textModel, err := fixture.client.Model.Query().
		Where(model.ModelIDEQ("text-model")).
		Only(fixture.ctx)
	require.NoError(t, err)
	settings := textModel.Settings
	settings.Associations[0].When = &objects.ModelAssociationWhen{
		Enabled: true,
		Condition: &objects.Condition{
			Type:     objects.ConditionTypeCondition,
			Field:    objects.ModelAssociationConditionFieldHasImage,
			Operator: "eq",
			Value:    false,
		},
	}
	_, err = textModel.Update().SetSettings(settings).Save(fixture.ctx)
	require.NoError(t, err)

	fixture.orchestrator.channelSelector = NewDefaultSelector(
		fixture.orchestrator.ChannelService,
		fixture.orchestrator.ModelService,
		fixture.orchestrator.SystemService,
	)

	_, err = fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 2)
	require.Contains(t, string(requests[0].Body), "image_url")
	require.NotContains(t, string(requests[1].Body), "image_url")
	require.Contains(t, string(requests[1].Body), "AXONHUB_VISION_EVIDENCE")
}

func TestVisionDelegationRejectsImpossiblePrimaryRouteBeforeVisionCall(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, nil, &biz.RetryPolicy{Enabled: false})

	textModel, err := fixture.client.Model.Query().
		Where(model.ModelIDEQ("text-model")).
		Only(fixture.ctx)
	require.NoError(t, err)
	settings := textModel.Settings
	settings.Associations[0].When = &objects.ModelAssociationWhen{
		Enabled: true,
		Condition: &objects.Condition{
			Type:     objects.ConditionTypeCondition,
			Field:    objects.ModelAssociationConditionFieldHasImage,
			Operator: "eq",
			Value:    true,
		},
	}
	_, err = textModel.Update().SetSettings(settings).Save(fixture.ctx)
	require.NoError(t, err)

	fixture.orchestrator.channelSelector = NewDefaultSelector(
		fixture.orchestrator.ChannelService,
		fixture.orchestrator.ModelService,
		fixture.orchestrator.SystemService,
	)

	_, err = fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.ErrorIs(t, err, biz.ErrInvalidModel)
	require.Empty(t, fixture.executor.Requests(), "an impossible primary route must fail before paid vision delegation")

	executionCount, err := fixture.client.RequestExecution.Query().Count(fixture.ctx)
	require.NoError(t, err)
	require.Zero(t, executionCount)
	usageCount, err := fixture.client.UsageLog.Query().Count(fixture.ctx)
	require.NoError(t, err)
	require.Zero(t, usageCount)
}

func TestImageRequestReusesPreloadedSourceModelWithoutDelegation(t *testing.T) {
	tests := []struct {
		name        string
		updateModel func(*objects.ModelSettings, *objects.ModelCard)
	}{
		{
			name: "delegation disabled",
			updateModel: func(settings *objects.ModelSettings, _ *objects.ModelCard) {
				settings.VisionDelegation.Enabled = false
			},
		},
		{
			name: "native vision model",
			updateModel: func(settings *objects.ModelSettings, card *objects.ModelCard) {
				settings.VisionDelegation.Enabled = false
				card.Vision = true
				card.Modalities.Input = []string{"text", "image"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
				{response: visionHTTPResponse(buildMockOpenAIResponse("primary-1", "text-model", "The total is 42.", 11, 5))},
			}, &biz.RetryPolicy{Enabled: false})

			textModel, err := fixture.client.Model.Query().
				Where(model.ModelIDEQ("text-model")).
				Only(fixture.ctx)
			require.NoError(t, err)
			settings := textModel.Settings
			card := textModel.ModelCard
			tt.updateModel(settings, card)
			_, err = textModel.Update().
				SetSettings(settings).
				SetModelCard(card).
				Save(fixture.ctx)
			require.NoError(t, err)

			fixture.orchestrator.channelSelector = NewDefaultSelector(
				fixture.orchestrator.ChannelService,
				fixture.orchestrator.ModelService,
				fixture.orchestrator.SystemService,
			)

			var modelQueries atomic.Int64
			fixture.client.Intercept(intercept.Func(func(_ context.Context, query intercept.Query) error {
				if query.Type() == ent.TypeModel {
					modelQueries.Add(1)
				}
				return nil
			}))

			_, err = fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
			require.NoError(t, err)
			require.EqualValues(t, 1, modelQueries.Load(), "image request must reuse the preloaded source model")

			requests := fixture.executor.Requests()
			require.Len(t, requests, 1)
			require.Contains(t, string(requests[0].Body), "image_url")
		})
	}
}

func TestImageRequestLegacyChannelFallbackReusesNotFoundModelResolution(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-1", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})

	textModel, err := fixture.client.Model.Query().
		Where(model.ModelIDEQ("text-model")).
		Only(fixture.ctx)
	require.NoError(t, err)
	require.NoError(t, fixture.client.Model.DeleteOne(textModel).Exec(fixture.ctx))
	require.NoError(t, fixture.orchestrator.SystemService.SetModelSettings(fixture.ctx, biz.SystemModelSettings{
		FallbackToChannelsOnModelNotFound: true,
	}))

	fixture.orchestrator.channelSelector = NewDefaultSelector(
		fixture.orchestrator.ChannelService,
		fixture.orchestrator.ModelService,
		fixture.orchestrator.SystemService,
	)

	var modelQueries atomic.Int64
	fixture.client.Intercept(intercept.Func(func(_ context.Context, query intercept.Query) error {
		if query.Type() == ent.TypeModel {
			modelQueries.Add(1)
		}
		return nil
	}))

	_, err = fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)
	require.EqualValues(t, 1, modelQueries.Load(), "legacy image fallback must reuse the NotFound model resolution")

	requests := fixture.executor.Requests()
	require.Len(t, requests, 1)
	require.Contains(t, string(requests[0].Body), "image_url")

	executions, err := fixture.client.RequestExecution.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)
	require.Equal(t, requestexecution.PurposePrimary, executions[0].Purpose)
}

func TestVisionDelegationDisablesRequestBodyPassThrough(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-1", "vision-model", "The screenshot shows total 42.", 7, 3))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-1", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})

	selector, ok := fixture.orchestrator.channelSelector.(*visionDelegationTestSelector)
	require.True(t, ok)
	selector.candidates["text-model"][0].Channel.Settings = &objects.ChannelSettings{
		PassThroughBody: lo.ToPtr(true),
	}

	_, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 2)
	require.NotContains(t, string(requests[1].Body), "image_url")
	require.Contains(t, string(requests[1].Body), "AXONHUB_VISION_EVIDENCE")

	primaryExecution, err := fixture.client.RequestExecution.Query().
		Where(requestexecution.PurposeEQ(requestexecution.PurposePrimary)).
		Only(fixture.ctx)
	require.NoError(t, err)
	require.False(t, primaryExecution.PassThroughApplied)
}

func TestVisionDelegationDoesNotRepeatHistoricalImagesOnTextFollowUp(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-1", "text-model", "I am the text model.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})

	selector, ok := fixture.orchestrator.channelSelector.(*visionDelegationTestSelector)
	require.True(t, ok)
	selector.candidates["text-model"][0].Channel.Settings = &objects.ChannelSettings{
		PassThroughBody: lo.ToPtr(true),
	}

	messages := []map[string]any{
		{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "What is in the top right?"},
				{"type": "image_url", "image_url": map[string]any{"url": base64TestImage("historical")}},
			},
		},
		{"role": "assistant", "content": "There is a red badge in the top right."},
		{"role": "user", "content": "What model are you?"},
	}

	_, err := fixture.orchestrator.Process(
		fixture.ctx,
		buildVisionTestRequestWithMessages(t, messages, false),
	)
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 1)
	require.NotContains(t, string(requests[0].Body), "image_url")
	require.NotContains(t, string(requests[0].Body), visionEvidenceStart)
	require.Contains(t, string(requests[0].Body), "What model are you?")

	executions, err := fixture.client.RequestExecution.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)
	require.Equal(t, requestexecution.PurposePrimary, executions[0].Purpose)
	require.False(t, executions[0].PassThroughApplied)
}

func TestVisionDelegationFailuresAbortPrimaryRequest(t *testing.T) {
	tests := []struct {
		name           string
		result         visionExecutorResult
		retryPolicy    *biz.RetryPolicy
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "upstream failure",
			result:         visionExecutorResult{err: visionUpstreamError()},
			retryPolicy:    &biz.RetryPolicy{Enabled: false},
			expectedStatus: http.StatusBadGateway,
			expectedCode:   "vision_delegation_failed",
		},
		{
			name: "empty response",
			result: visionExecutorResult{response: visionHTTPResponse(
				buildMockOpenAIResponse("vision-empty", "vision-model", "", 7, 0),
			)},
			retryPolicy:    &biz.RetryPolicy{Enabled: false},
			expectedStatus: http.StatusBadGateway,
			expectedCode:   "vision_delegation_failed",
		},
		{
			name:           "timeout",
			result:         visionExecutorResult{waitForContext: true},
			retryPolicy:    &biz.RetryPolicy{Enabled: false, NonStreamResponseTimeoutSeconds: 1},
			expectedStatus: http.StatusGatewayTimeout,
			expectedCode:   "vision_delegation_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{tt.result}, tt.retryPolicy)

			_, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
			require.Error(t, err)

			var responseErr *llm.ResponseError
			require.ErrorAs(t, err, &responseErr)
			require.Equal(t, tt.expectedStatus, responseErr.StatusCode)
			require.Equal(t, tt.expectedCode, responseErr.Detail.Code)
			require.Len(t, fixture.executor.Requests(), 1, "primary model must not run after vision delegation fails")

			requests, queryErr := fixture.client.Request.Query().All(fixture.ctx)
			require.NoError(t, queryErr)
			require.Len(t, requests, 1)
			require.Equal(t, requestent.StatusFailed, requests[0].Status)
		})
	}
}

func TestVisionDelegationRunsOnceAcrossPrimaryRetries(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-1", "vision-model", "The screenshot shows total 42.", 7, 3))},
		{err: visionUpstreamError()},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-2", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{
		Enabled:                 true,
		MaxSingleChannelRetries: 1,
		RetryDelayMs:            0,
	})

	result, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 3)
	visionCalls := 0
	for _, request := range requests {
		if strings.Contains(string(request.Body), "image_url") {
			visionCalls++
		}
	}
	require.Equal(t, 1, visionCalls)
	require.Contains(t, string(requests[1].Body), "AXONHUB_VISION_EVIDENCE")
	require.Contains(t, string(requests[2].Body), "AXONHUB_VISION_EVIDENCE")
}

func TestVisionDelegationRepeatsVisionAcrossClientRequests(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-1", "vision-model", "Evidence 1", 7, 3))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-1", "text-model", "The total is 42.", 11, 5))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-2", "vision-model", "Evidence 2", 7, 3))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-2", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})

	_, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)
	_, err = fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 4)
	require.Contains(t, string(requests[0].Body), "image_url")
	require.Contains(t, string(requests[1].Body), "AXONHUB_VISION_EVIDENCE")
	require.Contains(t, string(requests[2].Body), "image_url")
	require.Contains(t, string(requests[3].Body), "AXONHUB_VISION_EVIDENCE")
	require.Contains(t, string(requests[1].Body), "Evidence 1")
	require.Contains(t, string(requests[3].Body), "Evidence 2")

	executions, err := fixture.client.RequestExecution.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Len(t, executions, 4)
	visionExecutions := 0
	for _, execution := range executions {
		if execution.Purpose == requestexecution.PurposeVisionDelegation {
			visionExecutions++
		}
	}
	require.Equal(t, 2, visionExecutions)
}

func TestVisionDelegationFailureDoesNotAffectNextRequest(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{err: visionUpstreamError()},
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-2", "vision-model", "The screenshot shows total 42.", 7, 3))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary-2", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})
	_, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.Error(t, err)
	_, err = fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 3)
	require.Contains(t, string(requests[0].Body), "image_url")
	require.Contains(t, string(requests[1].Body), "image_url")
}

func TestVisionDelegationInvalidEvidenceDoesNotAffectNextRequest(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse(
			"vision-plan",
			"vision-model",
			"I need to use the image-reading tool and crop the file first.",
			7,
			3,
		))},
		{response: visionHTTPResponse(buildMockOpenAIResponse(
			"vision-valid",
			"vision-model",
			"The screenshot shows total 42.",
			7,
			3,
		))},
		{response: visionHTTPResponse(buildMockOpenAIResponse("primary", "text-model", "The total is 42.", 11, 5))},
	}, &biz.RetryPolicy{Enabled: false})
	_, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.Error(t, err)
	var responseErr *llm.ResponseError
	require.ErrorAs(t, err, &responseErr)
	require.Equal(t, http.StatusBadGateway, responseErr.StatusCode)
	require.Equal(t, "vision_delegation_failed", responseErr.Detail.Code)

	_, err = fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequest(t))
	require.NoError(t, err)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 3)
	require.Contains(t, string(requests[0].Body), "image_url")
	require.Contains(t, string(requests[1].Body), "image_url")
	require.Contains(t, string(requests[2].Body), "The screenshot shows total 42.")
}

func TestVisionDelegationSupportsStreamingPrimaryRequest(t *testing.T) {
	fixture := newVisionDelegationPipelineFixture(t, []visionExecutorResult{
		{response: visionHTTPResponse(buildMockOpenAIResponse("vision-1", "vision-model", "The screenshot shows total 42.", 7, 3))},
		{streamEvents: []*httpclient.StreamEvent{
			{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"text-model","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)},
			{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"text-model","choices":[{"index":0,"delta":{"content":"The total is 42."},"finish_reason":null}]}`)},
			{Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"text-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`)},
		}},
	}, &biz.RetryPolicy{Enabled: false})

	result, err := fixture.orchestrator.Process(fixture.ctx, buildVisionTestRequestWithStream(t, true))
	require.NoError(t, err)
	require.Nil(t, result.ChatCompletion)
	require.NotNil(t, result.ChatCompletionStream)

	chunkCount := 0
	for result.ChatCompletionStream.Next() {
		chunkCount++
	}
	require.NoError(t, result.ChatCompletionStream.Err())
	require.Positive(t, chunkCount)

	requests := fixture.executor.Requests()
	require.Len(t, requests, 2)
	require.NotContains(t, string(requests[1].Body), "image_url")
	require.Contains(t, string(requests[1].Body), "AXONHUB_VISION_EVIDENCE")

	executions, err := fixture.client.RequestExecution.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Len(t, executions, 2)
	for _, execution := range executions {
		switch execution.Purpose {
		case requestexecution.PurposeVisionDelegation:
			require.False(t, execution.Stream)
		case requestexecution.PurposePrimary:
			require.True(t, execution.Stream)
		default:
			t.Fatalf("unexpected execution purpose: %s", execution.Purpose)
		}
	}
}

type visionDelegationPipelineFixture struct {
	ctx          context.Context
	client       *ent.Client
	allowedRow   *ent.Channel
	orchestrator *ChatCompletionOrchestrator
	executor     *queuedVisionExecutor
}

func newVisionDelegationPipelineFixture(
	t *testing.T,
	results []visionExecutorResult,
	retryPolicy *biz.RetryPolicy,
) *visionDelegationPipelineFixture {
	t.Helper()

	ctx := authz.WithTestBypass(context.Background())
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx = ent.NewContext(ctx, client)

	project := createTestProject(t, ctx, client)
	allowedRow := createVisionOrchestratorChannel(t, ctx, client, "Allowed", "https://allowed.example.com/v1")
	disallowedRow := createVisionOrchestratorChannel(t, ctx, client, "Disallowed", "https://disallowed.example.com/v1")
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	if retryPolicy != nil {
		require.NoError(t, systemService.SetRetryPolicy(ctx, retryPolicy))
	}

	allowedChannel := buildVisionOrchestratorChannel(t, allowedRow)
	disallowedChannel := buildVisionOrchestratorChannel(t, disallowedRow)
	channelService.SetEnabledChannelsForTest([]*biz.Channel{allowedChannel, disallowedChannel})
	modelService := biz.NewModelService(biz.ModelServiceParams{
		ChannelService: channelService,
		SystemService:  systemService,
		Ent:            client,
	})

	visionSettings := &objects.ModelSettings{Associations: []*objects.ModelAssociation{
		visionChannelAssociation(allowedRow.ID, "vision-model"),
		visionChannelAssociation(disallowedRow.ID, "vision-model"),
	}}
	_, err := client.Model.Create().
		SetDeveloper("test").
		SetModelID("vision-model").
		SetType(model.TypeChat).
		SetName("Vision Model").
		SetIcon("test").
		SetGroup("test").
		SetModelCard(&objects.ModelCard{Vision: true, Modalities: objects.ModelCardModalities{Input: []string{"text", "image"}}}).
		SetSettings(visionSettings).
		SetStatus(model.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Model.Create().
		SetDeveloper("test").
		SetModelID("text-model").
		SetType(model.TypeChat).
		SetName("Text Model").
		SetIcon("test").
		SetGroup("test").
		SetModelCard(&objects.ModelCard{Modalities: objects.ModelCardModalities{Input: []string{"text"}}}).
		SetSettings(&objects.ModelSettings{
			Associations: []*objects.ModelAssociation{visionChannelAssociation(allowedRow.ID, "text-model")},
			VisionDelegation: objects.VisionDelegation{
				Enabled:       true,
				TargetModelID: lo.ToPtr("vision-model"),
			},
		}).
		SetStatus(model.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	user, err := client.User.Create().SetEmail("vision@example.com").SetPassword("password").Save(ctx)
	require.NoError(t, err)
	key, err := client.APIKey.Create().
		SetName("Vision Test Key").
		SetKey("ah-vision-test").
		SetProjectID(project.ID).
		SetUserID(user.ID).
		SetProfiles(&objects.APIKeyProfiles{
			ActiveProfile: "default",
			Profiles: []objects.APIKeyProfile{{
				Name:       "default",
				ModelIDs:   []string{"text-model"},
				ChannelIDs: []int{allowedRow.ID},
			}},
		}).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusEnabled, key.Status)

	selector := &visionDelegationTestSelector{candidates: map[string][]*ChannelModelsCandidate{
		"text-model":   {visionCandidate(allowedChannel, "text-model")},
		"vision-model": {visionCandidate(disallowedChannel, "vision-model"), visionCandidate(allowedChannel, "vision-model")},
	}}
	executor := &queuedVisionExecutor{results: results}
	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:       selector,
		Inbound:               openai.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		ModelService:          modelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares:           []pipeline.Middleware{stream.EnsureUsage()},
	}

	ctx = contexts.WithProjectID(ctx, project.ID)
	ctx = contexts.WithAPIKey(ctx, key)

	return &visionDelegationPipelineFixture{
		ctx:          ctx,
		client:       client,
		allowedRow:   allowedRow,
		orchestrator: orchestrator,
		executor:     executor,
	}
}

func visionUpstreamError() error {
	return &httpclient.Error{
		Method:     http.MethodPost,
		URL:        "https://allowed.example.com/v1/chat/completions",
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       []byte(`{"error":{"message":"upstream failed"}}`),
	}
}

func base64TestImage(value string) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte(value))
}

func createVisionOrchestratorChannel(t *testing.T, ctx context.Context, client *ent.Client, name, baseURL string) *ent.Channel {
	t.Helper()

	created, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetBaseURL(baseURL).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"text-model", "vision-model"}).
		SetDefaultTestModel("text-model").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return created
}

func buildVisionOrchestratorChannel(t *testing.T, row *ent.Channel) *biz.Channel {
	t.Helper()

	outbound, err := openai.NewOutboundTransformer(row.BaseURL, row.Credentials.APIKey)
	require.NoError(t, err)

	return &biz.Channel{Channel: row, Outbound: outbound}
}

func visionChannelAssociation(channelID int, modelID string) *objects.ModelAssociation {
	return visionChannelAssociationWithPriority(channelID, modelID, 0)
}

func visionChannelAssociationWithPriority(channelID int, modelID string, priority int) *objects.ModelAssociation {
	return &objects.ModelAssociation{
		Type:     "channel_model",
		Priority: priority,
		ChannelModel: &objects.ChannelModelAssociation{
			ChannelID: channelID,
			ModelID:   modelID,
		},
	}
}

func visionCandidate(ch *biz.Channel, modelID string) *ChannelModelsCandidate {
	return &ChannelModelsCandidate{
		Channel: ch,
		Models: []biz.ChannelModelEntry{{
			RequestModel: modelID,
			ActualModel:  modelID,
			Source:       "direct",
		}},
	}
}

func visionHTTPResponse(body []byte) *httpclient.Response {
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
	}
}

func buildVisionTestRequest(t *testing.T) *httpclient.Request {
	return buildVisionTestRequestWithStream(t, false)
}

func buildVisionTestRequestWithStream(t *testing.T, stream bool) *httpclient.Request {
	return buildVisionTestRequestWithQuestionAndStream(t, "What is the total?", stream)
}

func buildVisionTestRequestWithQuestionAndStream(t *testing.T, question string, stream bool) *httpclient.Request {
	t.Helper()

	messages := []map[string]any{{
		"role": "user",
		"content": []map[string]any{
			{"type": "text", "text": question},
			{"type": "image_url", "image_url": map[string]any{"url": base64TestImage("same-image")}},
			{"type": "image_url", "image_url": map[string]any{"url": base64TestImage("same-image")}},
			{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/chart.png"}},
		},
	}}

	return buildVisionTestRequestWithMessages(t, messages, stream)
}

func buildVisionTestRequestWithMessages(t *testing.T, messages []map[string]any, stream bool) *httpclient.Request {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"model":    "text-model",
		"messages": messages,
		"stream":   stream,
	})
	require.NoError(t, err)

	return &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: body,
	}
}

type visionDelegationTestSelector struct {
	candidates map[string][]*ChannelModelsCandidate
}

func (s *visionDelegationTestSelector) Select(_ context.Context, request *llm.Request) ([]*ChannelModelsCandidate, error) {
	return s.candidates[request.Model], nil
}

type queuedVisionExecutor struct {
	mu       sync.Mutex
	results  []visionExecutorResult
	requests []*httpclient.Request
}

type visionExecutorResult struct {
	response       *httpclient.Response
	err            error
	waitForContext bool
	streamEvents   []*httpclient.StreamEvent
}

func (e *queuedVisionExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	result, err := e.take(request)
	if err != nil {
		return nil, err
	}

	if result.waitForContext {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	return result.response, result.err
}

func (e *queuedVisionExecutor) take(request *httpclient.Request) (visionExecutorResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	requestCopy := *request
	requestCopy.Body = append([]byte(nil), request.Body...)
	e.requests = append(e.requests, &requestCopy)
	index := len(e.requests) - 1
	if index >= len(e.results) {
		return visionExecutorResult{}, errors.New("unexpected executor call")
	}

	return e.results[index], nil
}

func (e *queuedVisionExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	result, err := e.take(request)
	if err != nil {
		return nil, err
	}
	if result.waitForContext {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if result.err != nil {
		return nil, result.err
	}

	return streams.SliceStream(result.streamEvents), nil
}

func (e *queuedVisionExecutor) Requests() []*httpclient.Request {
	e.mu.Lock()
	defer e.mu.Unlock()

	return append([]*httpclient.Request(nil), e.requests...)
}
