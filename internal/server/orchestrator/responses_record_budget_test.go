package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesAdapterBudgetsHTTPRecord16MiB(t *testing.T) {
	for _, size := range []int{(1 << 20) - 1, (1 << 20) + 1, (16 << 20) - 1, (16 << 20) + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			// Given
			ctx := shared.WithSessionScope(context.Background(), "api_key:1:project:1")
			store := newResponsesSessionStore()
			item := `{"role":"user","content":"` + strings.Repeat("x", size-len(`{"role":"user","content":""}`)) + `"}`
			require.Len(t, item, size)
			// When
			store.record(ctx, []byte(`{"input":[`+item+`]}`), []byte(`{"id":"provider_id","status":"completed","output":[]}`))
			// Then
			record := store.lookup(ctx, "provider_id")
			require.NotNil(t, record)
			require.Equal(t, size <= 16<<20, record.complete)
		})
	}
}

func TestResponsesAdapterBudgetsHTTPPressurePreservesOrigin(t *testing.T) {
	// Given
	ctx := shared.WithSessionScope(t.Context(), "api_key:1:project:1")
	store := newResponsesSessionStore()
	input := []byte(`{"input":[{"role":"user","content":"` + strings.Repeat("x", (16<<20)-100) + `"}]}`)
	for i := range 4 {
		store.record(ctx, input, fmt.Appendf(nil, `{"id":"response_%d","status":"completed","output":[]}`, i))
	}
	// When
	store.record(ctx, input, []byte(`{"id":"response_4","status":"completed","output":[]}`))
	// Then
	record := store.lookup(ctx, "response_0")
	require.NotNil(t, record)
	require.False(t, record.complete)
	require.LessOrEqual(t, store.totalBytes, 64<<20)
}
