package gemini

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestInboundTransformer_GenerateContentFieldAliases(t *testing.T) {
	for _, fields := range []struct {
		name, system, inline, mime string
	}{
		{"camelCase", "systemInstruction", "inlineData", "mimeType"},
		{"snake_case", "system_instruction", "inline_data", "mime_type"},
		{"mixed", "systemInstruction", "inline_data", "mimeType"},
	} {
		t.Run(fields.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{
				fields.system: map[string]any{"parts": []any{map[string]any{"text": "Describe images briefly."}}},
				"contents": []any{map[string]any{
					"role": "user",
					"parts": []any{
						map[string]any{"text": "What is in this image?"},
						map[string]any{fields.inline: map[string]any{fields.mime: "image/png", "data": "aGVsbG8="}},
					},
				}},
			})
			require.NoError(t, err)
			result, err := NewInboundTransformer().TransformRequest(context.Background(), &httpclient.Request{
				Path: "/v1beta/models/gemini-test:generateContent", Body: body,
			})
			require.NoError(t, err)
			require.Equal(t, "gemini-test", result.Model)
			require.Len(t, result.Messages, 2)
			require.Equal(t, "system", result.Messages[0].Role)
			require.Equal(t, "Describe images briefly.", *result.Messages[0].Content.Content)
			require.Len(t, result.Messages[1].Content.MultipleContent, 2)
			require.Equal(t, "data:image/png;base64,aGVsbG8=", result.Messages[1].Content.MultipleContent[1].ImageURL.URL)
		})
	}
}

func TestGenerateContentFieldAliases_PreferCamelCaseAndMarshalCanonicalNames(t *testing.T) {
	var request GenerateContentRequest
	err := json.Unmarshal([]byte(`{
		"system_instruction":{"parts":[{"text":"alias"}]},
		"systemInstruction":{"parts":[{"text":"canonical"}]},
		"contents":[{"parts":[{
			"inline_data":{"mime_type":"image/jpeg","data":"alias"},
			"inlineData":{"mime_type":"image/jpeg","mimeType":"image/png","data":"canonical"}
		}]}]
	}`), &request)
	require.NoError(t, err)
	require.Equal(t, "canonical", request.SystemInstruction.Parts[0].Text)
	require.Equal(t, "canonical", request.Contents[0].Parts[0].InlineData.Data)
	require.Equal(t, "image/png", request.Contents[0].Parts[0].InlineData.MIMEType)

	body, err := json.Marshal(request)
	require.NoError(t, err)
	require.Contains(t, string(body), `"systemInstruction"`)
	require.Contains(t, string(body), `"inlineData"`)
	require.Contains(t, string(body), `"mimeType"`)
	for _, alias := range []string{"system_instruction", "inline_data", "mime_type"} {
		require.NotContains(t, string(body), alias)
	}
}

func TestGenerateContentFieldAliases_RejectInvalidValues(t *testing.T) {
	for _, body := range []string{
		`{"system_instruction":42}`,
		`{"contents":[{"parts":[{"inline_data":42}]}]}`,
		`{"contents":[{"parts":[{"inline_data":{"mime_type":42}}]}]}`,
	} {
		var request GenerateContentRequest
		require.Error(t, json.Unmarshal([]byte(body), &request), body)
	}
}
