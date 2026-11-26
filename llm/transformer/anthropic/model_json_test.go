package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestMessageContentBlockMarshalJSON_PreservesEmptyThinkingSignature(t *testing.T) {
	data, err := json.Marshal(MessageContentBlock{
		Type:      "thinking",
		Thinking:  lo.ToPtr(""),
		Signature: lo.ToPtr(""),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"thinking","thinking":"","signature":""}`, string(data))
}

func TestMessageContentBlockMarshalJSON_PreservesNilThinkingSignature(t *testing.T) {
	data, err := json.Marshal(MessageContentBlock{
		Type: "thinking",
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"thinking","thinking":"","signature":""}`, string(data))
}

func TestMessageContentBlockMarshalJSON_WithCitations(t *testing.T) {
	data, err := json.Marshal(MessageContentBlock{
		Type: "text",
		Text: lo.ToPtr("hello"),
		Citations: []TextCitation{
			{
				Type:           "url_citation",
				URL:            "https://example.com/a",
				Title:          "Example A",
				EncryptedIndex: lo.ToPtr("enc-1"),
				CitedText:      lo.ToPtr("quote"),
			},
		},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type":"text",
		"text":"hello",
		"citations":[{
			"type":"url_citation",
			"url":"https://example.com/a",
			"title":"Example A",
			"encrypted_index":"enc-1",
			"cited_text":"quote"
		}]
	}`, string(data))
}

func TestMessageContentBlockUnmarshalJSON_WithCitations(t *testing.T) {
	var block MessageContentBlock
	err := json.Unmarshal([]byte(`{
		"type":"text",
		"text":"hello",
		"citations":[{
			"type":"url_citation",
			"url":"https://example.com/a",
			"title":"Example A",
			"encrypted_index":"enc-1",
			"cited_text":"quote"
		}]
	}`), &block)
	require.NoError(t, err)
	require.Equal(t, "text", block.Type)
	require.Equal(t, "hello", lo.FromPtr(block.Text))
	require.Equal(t, []TextCitation{
		{
			Type:           "url_citation",
			URL:            "https://example.com/a",
			Title:          "Example A",
			EncryptedIndex: lo.ToPtr("enc-1"),
			CitedText:      lo.ToPtr("quote"),
		},
	}, block.Citations)
}

func TestStreamDeltaMarshalJSON_OmitsSignatureForThinkingDelta(t *testing.T) {
	data, err := json.Marshal(StreamDelta{
		Type:      lo.ToPtr("thinking_delta"),
		Thinking:  lo.ToPtr("Thinking..."),
		Signature: lo.ToPtr(""),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"thinking_delta","thinking":"Thinking..."}`, string(data))
}

func TestMessageContentBlockMarshalJSON_DefaultsEmptyToolUseInput(t *testing.T) {
	tests := []struct {
		name  string
		block MessageContentBlock
		want  string
	}{
		{
			name: "tool_use nil input",
			block: MessageContentBlock{
				Type: "tool_use",
				ID:   "toolu_1",
				Name: lo.ToPtr("ping"),
			},
			want: `{"type":"tool_use","id":"toolu_1","name":"ping","input":{}}`,
		},
		{
			name: "tool_use empty input",
			block: MessageContentBlock{
				Type:  "tool_use",
				ID:    "toolu_2",
				Name:  lo.ToPtr("ping"),
				Input: json.RawMessage(""),
			},
			want: `{"type":"tool_use","id":"toolu_2","name":"ping","input":{}}`,
		},
		{
			name: "tool_use whitespace input",
			block: MessageContentBlock{
				Type:  "tool_use",
				ID:    "toolu_3",
				Name:  lo.ToPtr("ping"),
				Input: json.RawMessage("   \t\n"),
			},
			want: `{"type":"tool_use","id":"toolu_3","name":"ping","input":{}}`,
		},
		{
			name: "server_tool_use nil input",
			block: MessageContentBlock{
				Type: "server_tool_use",
				ID:   "srvtoolu_1",
				Name: lo.ToPtr("web_search"),
			},
			want: `{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}`,
		},
		{
			name: "tool_use non-empty input preserved",
			block: MessageContentBlock{
				Type:  "tool_use",
				ID:    "toolu_4",
				Name:  lo.ToPtr("search"),
				Input: json.RawMessage(`{"q":"hello"}`),
			},
			want: `{"type":"tool_use","id":"toolu_4","name":"search","input":{"q":"hello"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.block)
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(data))
		})
	}
}
