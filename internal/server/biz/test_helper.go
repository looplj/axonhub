package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

func (svc *ChannelService) PreloadModelPricesForTest(ctx context.Context, ch *Channel) {
	svc.preloadModelPrices(ctx, ch)
}

func NewChannelServiceForTest(client *ent.Client) *ChannelService {
	mockSysSvc := &SystemService{
		AbstractService: &AbstractService{
			db: client,
		},
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	svc := NewChannelService(ChannelServiceParams{
		CacheConfig:   xcache.Config{Mode: xcache.ModeMemory},
		Ent:           client,
		SystemService: mockSysSvc,
		HttpClient:    httpclient.NewHttpClient(),
	})

	svc.SetEnabledChannelsForTest([]*Channel{})

	return svc
}
