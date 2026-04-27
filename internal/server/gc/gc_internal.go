package gc

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/log"
)

func (w *Worker) runScheduledCleanup(ctx context.Context) {
	ctx, err := authz.WithSystemBypass(ctx, "gc-scheduled-cleanup")
	if err != nil {
		log.Error(context.Background(), "failed to create bypass context", log.Cause(err))
		return
	}
	w.runCleanup(ctx, false, nil)
}
