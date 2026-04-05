package claw

import (
	"context"
	"testing"

	axoncontext "github.com/looplj/axonhub/axon/context"
)

func TestIsolatedPromptUsesNewThreadAndTraceIDs(t *testing.T) {
	parentThreadID := "th-parent"
	parentTraceID := "trace-parent"

	ctx := context.Background()
	ctx = axoncontext.WithThreadID(ctx, parentThreadID)
	ctx = axoncontext.WithTraceID(ctx, parentTraceID)

	isolatedCtx := newIsolatedContext(ctx)
	isolatedThreadID := axoncontext.ThreadID(isolatedCtx)
	isolatedTraceID := axoncontext.TraceID(isolatedCtx)

	if got := axoncontext.ThreadID(isolatedCtx); got == parentThreadID {
		t.Fatalf("ThreadID() = %q, want new thread id", got)
	}

	if got := axoncontext.TraceID(isolatedCtx); got == parentTraceID {
		t.Fatalf("TraceID() = %q, want new trace id", got)
	}

	if isolatedThreadID == "" {
		t.Fatal("isolated thread id is empty")
	}

	if isolatedTraceID == "" {
		t.Fatal("isolated trace id is empty")
	}
}
