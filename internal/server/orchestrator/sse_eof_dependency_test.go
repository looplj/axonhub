package orchestrator_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestProductionModuleSSEDecoderIsSafeAfterTrailingEventEOF(t *testing.T) {
	decoder := httpclient.NewDefaultSSEDecoder(
		context.Background(),
		io.NopCloser(strings.NewReader("data: trailing-event\n")),
	)
	defer decoder.Close()

	require.True(t, decoder.Next())
	require.Equal(t, []byte("trailing-event"), decoder.Current().Data)

	for range 3 {
		require.NotPanics(t, func() {
			require.False(t, decoder.Next())
		})
		require.NoError(t, decoder.Err())
	}
}
