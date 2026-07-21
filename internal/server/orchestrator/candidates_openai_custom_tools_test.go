package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestOpenAICustomToolsSelector_Select_OnlyKeepsCompatibleCandidates(t *testing.T) {
	selector := WithOpenAICustomToolsSelector(&mockSelector{candidates: []*ChannelModelsCandidate{
		{
			APIFormat: llm.APIFormatOpenAIResponse.String(),
			Channel:   &biz.Channel{Channel: &ent.Channel{Name: "responses"}},
		},
		{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			Channel: &biz.Channel{Channel: &ent.Channel{
				Name: "chat-unverified",
			}},
		},
		{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			Channel: &biz.Channel{Channel: &ent.Channel{
				Name: "chat-verified",
				Endpoints: []objects.ChannelEndpoint{{
					APIFormat:                     llm.APIFormatOpenAIChatCompletion.String(),
					SupportsOpenAIChatCustomTools: true,
				}},
			}},
		},
		{
			APIFormat: llm.APIFormatAnthropicMessage.String(),
			Channel:   &biz.Channel{Channel: &ent.Channel{Name: "claude"}},
		},
	}})

	result, err := selector.Select(context.Background(), &llm.Request{
		APIFormat: llm.APIFormatOpenAIResponse,
		Tools: []llm.Tool{{
			Type: llm.ToolTypeResponsesCustomTool,
			ResponseCustomTool: &llm.ResponseCustomTool{
				Name: "exec",
			},
		}},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"responses", "chat-unverified", "chat-verified", "claude"}, candidateNames(result))
}

func TestOpenAICustomToolsSelector_Select_RejectsWhenNoCandidateCanCarryOrBridgeCustomTools(t *testing.T) {
	selector := WithOpenAICustomToolsSelector(&mockSelector{candidates: []*ChannelModelsCandidate{
		{
			APIFormat: llm.APIFormatGeminiContents.String(),
			Channel:   &biz.Channel{Channel: &ent.Channel{Name: "gemini"}},
		},
		{
			APIFormat: llm.APIFormatOllamaChat.String(),
			Channel:   &biz.Channel{Channel: &ent.Channel{Name: "ollama"}},
		},
	}})

	result, err := selector.Select(context.Background(), &llm.Request{
		APIFormat: llm.APIFormatOpenAIResponse,
		Tools: []llm.Tool{{
			Type: llm.ToolTypeResponsesCustomTool,
			ResponseCustomTool: &llm.ResponseCustomTool{
				Name: "exec",
			},
		}},
	})

	require.Nil(t, result)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.ErrorContains(t, err, "no candidate supports or can bridge OpenAI custom tools")
}

func TestOpenAICustomToolsSelector_Select_OnlyKeepsVerifiedChatCandidatesForNativeChatCustomTools(t *testing.T) {
	selector := WithOpenAICustomToolsSelector(&mockSelector{candidates: []*ChannelModelsCandidate{
		{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			Channel:   &biz.Channel{Channel: &ent.Channel{Name: "chat-unverified"}},
		},
		{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			Channel: &biz.Channel{Channel: &ent.Channel{
				Name: "chat-verified",
				Endpoints: []objects.ChannelEndpoint{{
					APIFormat:                     llm.APIFormatOpenAIChatCompletion.String(),
					SupportsOpenAIChatCustomTools: true,
				}},
			}},
		},
	}})

	result, err := selector.Select(context.Background(), &llm.Request{
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Tools: []llm.Tool{{
			Type: "custom",
			OpenAIChatCustomTool: &llm.OpenAIChatCustomTool{
				Name: "exec",
			},
		}},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"chat-verified"}, candidateNames(result))
}

func TestOpenAICustomToolsSelector_Select_RejectsIncompatibleCandidatesForNativeChatCustomToolHistory(t *testing.T) {
	selector := WithOpenAICustomToolsSelector(&mockSelector{candidates: []*ChannelModelsCandidate{
		{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			Channel:   &biz.Channel{Channel: &ent.Channel{Name: "chat-unverified"}},
		},
		{
			APIFormat: llm.APIFormatAnthropicMessage.String(),
			Channel:   &biz.Channel{Channel: &ent.Channel{Name: "claude"}},
		},
	}})

	result, err := selector.Select(context.Background(), &llm.Request{
		APIFormat: llm.APIFormatOpenAIChatCompletion,
		Messages: []llm.Message{{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{{
				Type: "custom",
				OpenAIChatCustomToolCall: &llm.OpenAIChatCustomToolCall{
					Name:  "exec",
					Input: "await tools.exec_command(...) ",
				},
			}},
		}},
	})

	require.Nil(t, result)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.ErrorContains(t, err, "no candidate supports or can bridge OpenAI custom tools")
}

func TestOpenAICustomToolsSelector_Select_KeepsBridgeCandidatesForResponsesCustomToolHistory(t *testing.T) {
	selector := WithOpenAICustomToolsSelector(&mockSelector{candidates: []*ChannelModelsCandidate{
		{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			Channel:   &biz.Channel{Channel: &ent.Channel{Name: "chat-unverified"}},
		},
		{
			APIFormat: llm.APIFormatAnthropicMessage.String(),
			Channel:   &biz.Channel{Channel: &ent.Channel{Name: "claude"}},
		},
	}})

	result, err := selector.Select(context.Background(), &llm.Request{
		APIFormat: llm.APIFormatOpenAIResponse,
		Messages: []llm.Message{{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{{
				Type: llm.ToolTypeResponsesCustomTool,
				ResponseCustomToolCall: &llm.ResponseCustomToolCall{
					CallID: "call_exec",
					Name:   "exec",
				},
			}},
		}},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"chat-unverified", "claude"}, candidateNames(result))
}

func candidateNames(candidates []*ChannelModelsCandidate) []string {
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.Channel.Name)
	}

	return result
}
