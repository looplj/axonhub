package biz

import (
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func NewChannelServiceForTest(client *ent.Client) *ChannelService {
	mockSysSvc := &SystemService{
		AbstractService: &AbstractService{
			db: client,
		},
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	svc := NewChannelService(ChannelServiceParams{
		CacheConfig:   xcache.Config{Mode: xcache.ModeMemory},
		Executor:      executors.NewPoolScheduleExecutor(),
		Ent:           client,
		SystemService: mockSysSvc,
	})

	svc.SetEnabledChannelsForTest([]*Channel{})

	return svc
}
