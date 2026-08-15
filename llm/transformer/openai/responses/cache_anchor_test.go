package responses

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestConversationAnchorIncludesProviderFileID(t *testing.T) {
	requestMessages := func(fileID string) []llm.Message {
		return []llm.Message{{
			Role: "user",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{{
				Type:     "image_url",
				ImageURL: &llm.ImageURL{FileID: fileID},
			}}},
		}}
	}

	require.NotEqual(
		t,
		conversationAnchor(requestMessages("file_123")),
		conversationAnchor(requestMessages("file_456")),
	)
}

func TestConversationAnchorIgnoresBlankProviderFileID(t *testing.T) {
	requestMessages := func(fileID string) []llm.Message {
		return []llm.Message{{
			Role: "user",
			Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{{
				Type:     "image_url",
				ImageURL: &llm.ImageURL{URL: "https://example.com/image.png", FileID: fileID},
			}}},
		}}
	}

	require.Equal(
		t,
		conversationAnchor(requestMessages("")),
		conversationAnchor(requestMessages(" \t\n ")),
	)
}
