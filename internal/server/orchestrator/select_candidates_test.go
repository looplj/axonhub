package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestSelectCandidates_PrioritizesCachePrimaryChannel(t *testing.T) {
	primaryID := 2
	inbound := &PersistentInboundTransformer{state: &PersistenceState{
		APIKey: &ent.APIKey{Profiles: &objects.APIKeyProfiles{
			ActiveProfile: "default",
			Profiles: []objects.APIKeyProfile{{
				Name:                  "default",
				CachePrimaryChannelID: &primaryID,
			}},
		}},
		CandidateSelector: &staticChannelSelector{candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "fallback-1"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "primary"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "fallback-2"}}},
		}},
	}}

	middleware := selectCandidates(inbound)
	_, err := middleware.OnInboundLlmRequest(context.Background(), &llm.Request{Model: "gpt-5.4"})
	require.NoError(t, err)
	require.Len(t, inbound.state.ChannelModelsCandidates, 3)
	require.Equal(t, 2, inbound.state.ChannelModelsCandidates[0].Channel.ID)
	require.Equal(t, 1, inbound.state.ChannelModelsCandidates[1].Channel.ID)
	require.Equal(t, 3, inbound.state.ChannelModelsCandidates[2].Channel.ID)
}
