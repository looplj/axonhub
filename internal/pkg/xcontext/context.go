package xcontext

import (
	"context"
	"time"
)

func DetachWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx = context.WithoutCancel(ctx)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	return ctx, cancel
}
