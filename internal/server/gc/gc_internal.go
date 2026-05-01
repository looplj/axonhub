package gc

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
)

func (w *Worker) runCleanupWithSystemContext(ctx context.Context) {
	w.wg.Add(1)
	defer w.wg.Done()

	ctx = authz.WithSystemBypass(ctx, "gc-cleanup")
	stats := &CleanupStats{}
	w.runCleanup(ctx, false, stats)
}
