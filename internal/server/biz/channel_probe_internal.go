package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/log"
)

func (svc *ChannelProbeService) runProbePeriodically(ctx context.Context) {
	ctx, err := authz.WithSystemBypass(ctx, "channel_probe")
	if err != nil {
		log.Error(context.Background(), "failed to create bypass context", log.Cause(err))
		return
	}
	svc.runProbe(ctx)
}
