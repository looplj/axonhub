package biz

import "github.com/looplj/axonhub/internal/ent"

// NewChannelServiceForTest creates a minimal ChannelService for testing purposes.
// It initializes the perfCh channel and starts a goroutine to drain it.
func NewChannelServiceForTest(client *ent.Client) *ChannelService {
	perfCh := make(chan *PerformanceRecord, 1024)
	// Start a goroutine to drain the performance channel
	go func() {
		for range perfCh {
			// Discard performance records in tests
		}
	}()

	return &ChannelService{
		AbstractService: &AbstractService{
			db: client,
		},
		channelPerfMetrics: make(map[int]*channelMetrics),
		perfCh:             perfCh,
		EnabledChannels:    []*Channel{},
	}
}
