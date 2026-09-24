package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

type staticAnthropicCandidateSelector []*ChannelModelsCandidate

func (s staticAnthropicCandidateSelector) Select(context.Context, *llm.Request) ([]*ChannelModelsCandidate, error) {
	return s, nil
}

func TestAnthropicNativeToolsSelector_Select_PreservesDeepSeek(t *testing.T) {
	newCandidate := func(channelType channel.Type) *ChannelModelsCandidate {
		return &ChannelModelsCandidate{
			Channel: &biz.Channel{Channel: &ent.Channel{Type: channelType}},
		}
	}

	direct := newCandidate(channel.TypeAnthropic)
	deepseek := newCandidate(channel.TypeDeepseekAnthropic)
	unsupported := newCandidate(channel.TypeMoonshotAnthropic)
	selector := WithAnthropicNativeToolsSelector(staticAnthropicCandidateSelector{
		direct,
		deepseek,
		unsupported,
	})

	got, err := selector.Select(context.Background(), &llm.Request{
		APIFormat: llm.APIFormatAnthropicMessage,
		Tools:     []llm.Tool{{Type: llm.ToolTypeWebSearch}},
	})
	require.NoError(t, err)
	require.Equal(t, []*ChannelModelsCandidate{direct, deepseek}, got)
}
