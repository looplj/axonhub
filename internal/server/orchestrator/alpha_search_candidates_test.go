package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestPopulateAPIFormatFiltersAlphaSearchUnsupportedChannels(t *testing.T) {
	unsupported := &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{Type: channel.TypeOpenai}}}
	supported := &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{Type: channel.TypeCodex}}}

	candidates := populateAPIFormat([]*ChannelModelsCandidate{unsupported, supported}, &llm.Request{
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
	})

	require.Equal(t, []*ChannelModelsCandidate{supported}, candidates)
	require.Equal(t, llm.APIFormatOpenAIAlphaSearch.String(), supported.APIFormat)
}
