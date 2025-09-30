package openai

import (
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestTransformRequest_RoutesToResponsesAPI_WhenImageToolPresent(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Generate a beautiful sunset over mountains")},
		}},
		Tools: []llm.Tool{{
			Type: "image_generation",
			ImageGeneration: &llm.ImageGeneration{
				OutputFormat: "png",
				Size:         "1024x1024",
			},
		}},
		Modalities: []string{"image"},
	}

	hreq, err := ot.TransformRequest(t.Context(), req)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/responses", hreq.URL)
}

func TestTransformRequest_RoutesToResponsesAPI_WhenModelIsImageCapable(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	req := &llm.Request{
		Model: "gpt-image-1",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Create an image of a futuristic city")},
		}},
		Modalities: []string{"image"},
	}

	hreq, err := ot.TransformRequest(t.Context(), req)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/responses", hreq.URL)
}

func TestTransformRequest_RoutesToChatCompletions_WhenTextOnly(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	req := &llm.Request{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
		}},
	}

	hreq, err := ot.TransformRequest(t.Context(), req)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", hreq.URL)
}

func TestTransformResponse_FromResponsesAPI_ImageResult(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	body := []byte(`{
		"id":"resp_123",
		"object":"response",
		"created_at": 1730000000,
		"model":"gpt-image-1",
		"status":"completed",
		"output":[
			{
				"id":"item_123",
				"type":"image_generation_call",
				"result":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		]
	}`)

	// Add metadata to indicate this is a responses API call
	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			Metadata: map[string]string{
				"outbound_format_type": "openai/responses",
			},
		},
	}

	resp, err := ot.TransformResponse(t.Context(), httpResp)
	assert.NoError(t, err)

	if assert.Len(t, resp.Choices, 1) {
		choice := resp.Choices[0]
		assert.NotNil(t, choice.Message)

		parts := choice.Message.Content.MultipleContent
		if assert.NotEmpty(t, parts) {
			assert.Equal(t, "image_url", parts[0].Type)

			if assert.NotNil(t, parts[0].ImageURL) {
				assert.Contains(t, parts[0].ImageURL.URL, "data:image/png;base64,")
			}
		}
	}
}

func TestTransformResponse_FromResponsesAPI_ImageResult_WithMeta(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	body := []byte(`{
		"id":"resp_123",
		"object":"response",
		"created_at": 1730000000,
		"model":"gpt-image-1",
		"status":"completed",
		"output":[
			{
				"id":"item_123",
				"type":"image_generation_call",
				"result":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		],
		"usage":{
			"input_tokens":10,
			"output_tokens":0,
			"total_tokens":10
		}
	}`)

	// Add metadata to indicate this is a responses API call
	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			Metadata: map[string]string{
				"outbound_format_type": "openai/responses",
			},
		},
	}

	resp, err := ot.TransformResponse(t.Context(), httpResp)
	assert.NoError(t, err)

	// Test that meta information is preserved
	assert.Equal(t, "resp_123", resp.ID)
	assert.Equal(t, "gpt-image-1", resp.Model)
	assert.Equal(t, int64(1730000000), resp.Created)
	assert.NotNil(t, resp.Usage)
	assert.Equal(t, 10, resp.Usage.PromptTokens)
	assert.Equal(t, 0, resp.Usage.CompletionTokens)
	assert.Equal(t, 10, resp.Usage.TotalTokens)
}

func TestTransformResponse_DebugResponsesAPI(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	// Simple debugging test to understand the response structure
	body := []byte(`{
		"id":"resp_123",
		"object":"response",
		"created_at": 1730000000,
		"model":"gpt-image-1",
		"status":"completed",
		"output":[
			{
				"id":"item_123",
				"type":"image_generation_call",
				"result":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		]
	}`)

	// Add metadata to indicate this is a responses API call
	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			Metadata: map[string]string{
				"outbound_format_type": "openai/responses",
			},
		},
	}

	resp, err := ot.TransformResponse(t.Context(), httpResp)
	assert.NoError(t, err)

	t.Logf("Response ID: %s", resp.ID)
	t.Logf("Response Model: %s", resp.Model)
	t.Logf("Response Created: %d", resp.Created)
	t.Logf("Number of Choices: %d", len(resp.Choices))
	t.Logf("Response Object: %s", resp.Object)

	if len(resp.Choices) > 0 {
		t.Logf("First Choice Index: %d", resp.Choices[0].Index)
		t.Logf("First Choice Message: %+v", resp.Choices[0].Message)

		if resp.Choices[0].Message != nil {
			t.Logf("Message Content: %+v", resp.Choices[0].Message.Content)
		}
	}
}

func TestTransformRequest_BuildsCorrectMetadata(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Generate a beautiful sunset over mountains")},
		}},
		Tools: []llm.Tool{{
			Type: "image_generation",
			ImageGeneration: &llm.ImageGeneration{
				OutputFormat: "jpeg",
				Size:         "1024x1024",
			},
		}},
		Modalities: []string{"image"},
	}

	hreq, err := ot.buildResponsesAPIRequest(t.Context(), req)
	assert.NoError(t, err)
	assert.NotNil(t, hreq)
	assert.Equal(t, "openai/responses", hreq.Metadata["outbound_format_type"])
}
