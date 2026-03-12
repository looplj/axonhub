package longcat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestOutboundTransformer_TransformRequest_ForceMultipleContent(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.example.com", "test-key")
	require.NoError(t, err)

	tests := []struct {
		name     string
		content  llm.MessageContent
		wantText string
	}{
		{
			name:     "plain string content is converted to array",
			content:  llm.MessageContent{Content: lo.ToPtr("Hello!")},
			wantText: "Hello!",
		},
		{
			name:     "empty string content is converted to array",
			content:  llm.MessageContent{Content: lo.ToPtr("")},
			wantText: "",
		},
		{
			name:     "nil content gets empty text array",
			content:  llm.MessageContent{},
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &llm.Request{
				Model:  "LongCat-Flash-Omni-2603",
				Stream: lo.ToPtr(false),
				Messages: []llm.Message{
					{Role: "user", Content: tt.content},
				},
			}

			httpReq, err := tr.TransformRequest(context.Background(), req)
			require.NoError(t, err)

			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(httpReq.Body, &body))

			var messages []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body["messages"], &messages))

			// Content must be an array, not a string
			contentRaw := messages[0]["content"]
			require.True(t, len(contentRaw) > 0 && contentRaw[0] == '[',
				"expected array content, got: %s", string(contentRaw))

			var parts []map[string]string
			require.NoError(t, json.Unmarshal(contentRaw, &parts))
			require.Len(t, parts, 1)
			require.Equal(t, "text", parts[0]["type"])
			require.Equal(t, tt.wantText, parts[0]["text"])
		})
	}
}

func TestOutboundTransformer_TransformRequest_MultipleContentPreserved(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.example.com", "test-key")
	require.NoError(t, err)

	req := &llm.Request{
		Model:  "LongCat-Flash-Omni-2603",
		Stream: lo.ToPtr(false),
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("What is this?")},
						{Type: "image_url", ImageURL: &llm.ImageURL{URL: "https://example.com/img.png"}},
					},
				},
			},
		},
	}

	httpReq, err := tr.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(httpReq.Body, &body))

	var messages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body["messages"], &messages))

	// Content must remain an array
	contentRaw := messages[0]["content"]
	require.True(t, len(contentRaw) > 0 && contentRaw[0] == '[',
		"expected array content, got: %s", string(contentRaw))
}
