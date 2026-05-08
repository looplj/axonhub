package ollama

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestTransformRequestPreservesImageURLParts(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL: "http://localhost:11434",
	})
	require.NoError(t, err)

	req, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model: "qwen3.5:4b-q4_K_M",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: lo.ToPtr("OCR this image."),
						},
						{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: "data:image/png;base64,Zm9v",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	var got ChatRequest
	require.NoError(t, json.Unmarshal(req.Body, &got))
	require.Len(t, got.Messages, 1)
	require.Equal(t, "OCR this image.", got.Messages[0].Content)
	require.Equal(t, []string{"Zm9v"}, got.Messages[0].Images)
}

func TestTransformRequestPreservesPlainImageURLParts(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL: "http://localhost:11434",
	})
	require.NoError(t, err)

	req, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model: "vision-model",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: "https://example.com/image.png",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	var got ChatRequest
	require.NoError(t, json.Unmarshal(req.Body, &got))
	require.Len(t, got.Messages, 1)
	require.Equal(t, []string{"https://example.com/image.png"}, got.Messages[0].Images)
}
