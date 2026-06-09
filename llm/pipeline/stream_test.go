package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

func TestNextLlmStreamEventReturnsTimeoutWhenTimerAlreadyWon(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	guard := &firstEventTimeoutGuard{
		timer:  time.NewTimer(time.Hour),
		cancel: cancel,
	}
	t.Cleanup(func() {
		guard.stop()
		cancel()
	})

	guard.state.Store(firstEventTimedOut)
	cancel()

	llmStream := streams.SliceStream([]*llm.Response{{Object: "chat.completion.chunk"}})

	hasNext, err := nextLlmStreamEvent(ctx, llmStream, true, guard)

	require.False(t, hasNext)
	require.ErrorIs(t, err, ErrStreamFirstEventTimeout)
}
