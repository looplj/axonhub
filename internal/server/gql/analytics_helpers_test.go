package gql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// An aggregate over an empty request_executions table still returns one row,
// so the counts must come back as zero rather than an error.
func TestQueryExecutionSuccessCounts_EmptyTable(t *testing.T) {
	resolver, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	success, failed, err := resolver.queryExecutionSuccessCounts(ctx, nil, nil, false, time.UTC)
	require.NoError(t, err)
	require.Zero(t, success)
	require.Zero(t, failed)
}
