package objects

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageChannelAgentInstanceBinding_Equals(t *testing.T) {
	t.Run("equal when allow without mention matches", func(t *testing.T) {
		binding := &MessageChannelAgentInstanceBinding{
			ChatType:            MessageChatTypeGroup,
			ChatID:              "oc_group",
			AllowWithoutMention: true,
			AllowFrom:           []string{"user_1"},
			ExcludeKeywords:     []string{"skip"},
		}

		other := &MessageChannelAgentInstanceBinding{
			ChatType:            MessageChatTypeGroup,
			ChatID:              "oc_group",
			AllowWithoutMention: true,
			AllowFrom:           []string{"user_1"},
			ExcludeKeywords:     []string{"skip"},
		}

		require.True(t, binding.Equals(other))
	})

	t.Run("not equal when allow without mention differs", func(t *testing.T) {
		binding := &MessageChannelAgentInstanceBinding{
			ChatType:            MessageChatTypeGroup,
			ChatID:              "oc_group",
			AllowWithoutMention: true,
		}

		other := &MessageChannelAgentInstanceBinding{
			ChatType:            MessageChatTypeGroup,
			ChatID:              "oc_group",
			AllowWithoutMention: false,
		}

		require.False(t, binding.Equals(other))
	})

	t.Run("not equal when one has allow without mention set", func(t *testing.T) {
		binding := &MessageChannelAgentInstanceBinding{
			ChatType: MessageChatTypeGroup,
			ChatID:   "oc_group",
		}

		other := &MessageChannelAgentInstanceBinding{
			ChatType:            MessageChatTypeGroup,
			ChatID:              "oc_group",
			AllowWithoutMention: true,
		}

		require.False(t, binding.Equals(other))
	})
}
