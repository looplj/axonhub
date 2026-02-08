package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
)

func (svc *ChannelProbeService) runProbePeriodically(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "channel_probe")
	svc.runProbe(ctx)
}
