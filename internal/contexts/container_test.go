package contexts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitializePersistsChannelAPIKeyExclusions(t *testing.T) {
	ctx := Initialize(context.Background())

	ExcludeChannelAPIKey(ctx, 42, `failed-key`)

	require.True(t, IsChannelAPIKeyExcluded(ctx, 42, `failed-key`))
}
