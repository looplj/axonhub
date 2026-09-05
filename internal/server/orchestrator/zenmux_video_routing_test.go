//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to candidate routing.
package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	zenmuxtransformer "github.com/looplj/axonhub/llm/transformer/zenmux"
)

type zenmuxVideoCandidateFixture struct {
	candidate      *ChannelModelsCandidate
	openAIOutbound transformer.Outbound
	nativeOutbound transformer.Outbound
}

func newZenmuxVideoCandidateFixture(t *testing.T, settings *objects.ChannelSettings) zenmuxVideoCandidateFixture {
	t.Helper()

	openAIOutbound, err := openai.NewOutboundTransformer("https://zenmux.ai/api/v1", "test-key")
	require.NoError(t, err)
	nativeOutbound, err := zenmuxtransformer.NewOutboundTransformer("https://zenmux.ai/api/v1", "test-key")
	require.NoError(t, err)

	channelWithOutbounds := &biz.Channel{
		Channel: &ent.Channel{
			ID:       1,
			Name:     "ZenMux video",
			Type:     channel.TypeZenmux,
			Settings: settings,
		},
		Outbound: openAIOutbound,
		Outbounds: map[string]transformer.Outbound{
			llm.APIFormatOpenAIVideo.String(): openAIOutbound,
			llm.APIFormatZenmuxVideo.String(): nativeOutbound,
		},
	}

	return zenmuxVideoCandidateFixture{
		candidate: &ChannelModelsCandidate{
			Channel: channelWithOutbounds,
			Models: []biz.ChannelModelEntry{{
				RequestModel: "video-model",
				ActualModel:  "video-model",
			}},
		},
		openAIOutbound: openAIOutbound,
		nativeOutbound: nativeOutbound,
	}
}

func TestPopulateAPIFormat_ZenMuxVideoOverrideSelectsNativeOutbound(t *testing.T) {
	fixture := newZenmuxVideoCandidateFixture(t, &objects.ChannelSettings{
		ModelProtocols: []objects.ModelProtocol{{
			Model:      "video-model",
			APIFormats: []string{llm.APIFormatZenmuxVideo.String()},
		}},
	})
	request := &llm.Request{
		Model:       "video-model",
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatOpenAIVideo,
	}

	result := populateAPIFormat(context.Background(), []*ChannelModelsCandidate{fixture.candidate}, request)
	selectedOutbound := selectOutboundForCandidate(fixture.candidate)

	require.Len(t, result, 1)
	require.Equal(t, llm.APIFormatZenmuxVideo.String(), fixture.candidate.APIFormat)
	require.Same(t, fixture.nativeOutbound, selectedOutbound)
	require.IsType(t, &zenmuxtransformer.OutboundTransformer{}, selectedOutbound)
}

func TestPopulateAPIFormat_ZenMuxVideoWithoutOverrideSelectsOpenAIOutbound(t *testing.T) {
	fixture := newZenmuxVideoCandidateFixture(t, nil)
	request := &llm.Request{
		Model:       "video-model",
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatOpenAIVideo,
	}

	result := populateAPIFormat(context.Background(), []*ChannelModelsCandidate{fixture.candidate}, request)
	selectedOutbound := selectOutboundForCandidate(fixture.candidate)

	require.Len(t, result, 1)
	require.Equal(t, llm.APIFormatOpenAIVideo.String(), fixture.candidate.APIFormat)
	require.Same(t, fixture.openAIOutbound, selectedOutbound)
	require.IsType(t, &openai.OutboundTransformer{}, selectedOutbound)
}
