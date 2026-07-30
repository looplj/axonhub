package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/log"
)

// ChannelProviderQuotaInvalidator lets ChannelService discard quota state when
// a channel's provider identity changes. It lives in biz to avoid coupling the
// channel service to a concrete quota implementation.
type ChannelProviderQuotaInvalidator interface {
	InvalidateChannelQuota(ctx context.Context, channelID int) error
}

// SetChannelProviderQuotaInvalidator wires the provider quota invalidator once
// at startup. It is optional for tests that construct ChannelService directly.
func (svc *ChannelService) SetChannelProviderQuotaInvalidator(invalidator ChannelProviderQuotaInvalidator) {
	svc.providerQuotaInvalidator = invalidator
}

func (svc *ChannelService) invalidateProviderQuota(ctx context.Context, channelID int) {
	if svc.providerQuotaInvalidator == nil {
		return
	}

	if err := svc.providerQuotaInvalidator.InvalidateChannelQuota(ctx, channelID); err != nil {
		log.Warn(ctx, "failed to invalidate provider quota after channel provider change",
			log.Int("channel_id", channelID),
			log.Cause(err))
	}
}
