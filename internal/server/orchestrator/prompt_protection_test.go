package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestProtectPromptsMaskContent(t *testing.T) {
	state := &PersistenceState{
		PromptProtecter: &stubPromptProtecter{
			result: nil,
		},
	}
	inbound := &PersistentInboundTransformer{state: state}
	middleware := protectPrompts(inbound)

	content := "token is secret-123"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}
	protectedContent := "token is [MASKED]"
	state.PromptProtecter.(*stubPromptProtecter).result = &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &protectedContent}},
		},
	}

	result, err := middleware.OnInboundLlmRequest(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Messages[0].Content.Content)
	assert.Equal(t, "token is [MASKED]", *result.Messages[0].Content.Content)
	assert.Equal(t, "token is secret-123", *request.Messages[0].Content.Content)
}

func TestProtectPromptsRejectContent(t *testing.T) {
	state := &PersistenceState{
		PromptProtecter: &stubPromptProtecter{
			err: biz.ErrPromptProtectionRejected,
		},
	}
	inbound := &PersistentInboundTransformer{state: state}
	middleware := protectPrompts(inbound)

	content := "contains secret"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	result, err := middleware.OnInboundLlmRequest(context.Background(), request)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, transformer.ErrInvalidRequest)
	assert.ErrorContains(t, err, promptProtectionRejectedMessage)
	assert.NotContains(t, err.Error(), "reject-secret")
}

func TestProtectPromptsScopeFiltering(t *testing.T) {
	state := &PersistenceState{
		PromptProtecter: &stubPromptProtecter{result: nil},
	}
	inbound := &PersistentInboundTransformer{state: state}
	middleware := protectPrompts(inbound)

	assistantContent := "contains secret"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "assistant", Content: llm.MessageContent{Content: &assistantContent}},
		},
	}

	result, err := middleware.OnInboundLlmRequest(context.Background(), request)
	require.NoError(t, err)
	assert.Same(t, request, result)
	assert.Equal(t, "contains secret", *result.Messages[0].Content.Content)
}

func TestProtectPromptsMaskMultipleContent(t *testing.T) {
	state := &PersistenceState{
		PromptProtecter: &stubPromptProtecter{
			result: nil,
		},
	}
	inbound := &PersistentInboundTransformer{state: state}
	middleware := protectPrompts(inbound)

	partText := "secret text"
	request := &llm.Request{
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: &partText},
					},
				},
			},
		},
	}
	maskedText := "[MASKED] text"
	state.PromptProtecter.(*stubPromptProtecter).result = &llm.Request{
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: &maskedText},
					},
				},
			},
		},
	}

	result, err := middleware.OnInboundLlmRequest(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, result.Messages[0].Content.MultipleContent, 1)
	require.NotNil(t, result.Messages[0].Content.MultipleContent[0].Text)
	assert.Equal(t, "[MASKED] text", *result.Messages[0].Content.MultipleContent[0].Text)
	assert.Equal(t, "secret text", *request.Messages[0].Content.MultipleContent[0].Text)
}

type stubPromptProtecter struct {
	result *llm.Request
	err    error
}

func (s *stubPromptProtecter) Protect(context.Context, *llm.Request) (*llm.Request, error) {
	return s.result, s.err
}
