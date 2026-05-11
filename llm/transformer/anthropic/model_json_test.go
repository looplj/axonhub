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
