package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/log"
)

func (svc *ProviderQuotaService) runQuotaCheckScheduled(ctx context.Context) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	ctx, err := authz.WithSystemBypass(ctx, "provider_quota")
	if err != nil {
		log.Error(context.Background(), "failed to create bypass context", log.Cause(err))
		return
	}
	svc.runQuotaCheck(ctx, false)
}
