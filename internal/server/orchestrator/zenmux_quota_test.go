package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestQuotaRoutingGate_filters_exhausted_zenmux_native_video_channel(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {ProviderType: "zenmux", Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {ProviderType: "zenmux", Status: providerquotastatus.StatusAvailable, Ready: true},
		},
	}
	gate := NewQuotaRoutingGate(provider, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted})
	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{
				Channel: &biz.Channel{Channel: &ent.Channel{
					ID:        1,
					Name:      "zenmux-native-video",
					Type:      channel.TypeZenmux,
					Endpoints: []objects.ChannelEndpoint{{APIFormat: llm.APIFormatZenmuxVideo.String()}},
				}},
				APIFormat: llm.APIFormatZenmuxVideo.String(),
			},
			{
				Channel: &biz.Channel{Channel: &ent.Channel{
					ID:   2,
					Name: "zenmux-openai-video",
					Type: channel.TypeZenmux,
				}},
				APIFormat: llm.APIFormatOpenAIVideo.String(),
			},
		},
	}

	result, err := WithQuotaRoutingSelector(inner, gate).Select(context.Background(), &llm.Request{
		Model:       "video-model",
		RequestType: llm.RequestTypeVideo,
		Video:       &llm.VideoRequest{Model: "video-model"},
	})

	require.NoError(t, err)
	require.Equal(t, []int{2}, quotaRoutingIDs(result))
}
