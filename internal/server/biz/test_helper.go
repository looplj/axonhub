package biz

import (
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func NewChannelServiceForTest(client *ent.Client) *ChannelService {
	perfCh := make(chan *PerformanceRecord, 1024)
	mockSysSvc := &SystemService{
		AbstractService: &AbstractService{
			db: client,
		},
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	go func() {
		for range perfCh {
		}
	}()

	return &ChannelService{
		AbstractService: &AbstractService{
			db: client,
		},
		SystemService:      mockSysSvc,
		channelPerfMetrics: make(map[int]*channelMetrics),
		perfCh:             perfCh,
		enabledChannels:    []*Channel{},
	}
}
