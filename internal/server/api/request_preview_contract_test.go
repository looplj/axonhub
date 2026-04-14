package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Step 1 request-detail SSE contract (red tests):
// - single-instance only
// - request-level only
// - replay current buffered chunks first, then incremental updates
// - keep final batch persistence semantics unchanged
// - fallback to normal static fetch when SSE cannot connect

func TestRequestDetailSSEContract_SingleInstanceOnly(t *testing.T) {
	contract := RequestDetailSSEContract()

	assert.True(t, contract.SingleInstanceOnly)
	assert.False(t, contract.SupportsDistributedReplay)
	assert.False(t, contract.AllowsDatabaseSchemaChanges)
	assert.False(t, contract.ExecutionLevelPreview)
}

func TestRequestDetailSSEContract_ReplayBeforeIncremental(t *testing.T) {
	contract := RequestDetailSSEContract()

	assert.Equal(t, []string{"replay", "incremental"}, contract.EventOrder)
	assert.Equal(t, "request", contract.Scope)
	assert.True(t, contract.ReuseInMemoryChunkBuffer)
	assert.True(t, contract.FinalBatchPersistenceUnchanged)
	assert.Equal(t, "static-fetch", contract.FallbackMode)
}

func TestRequestDetailSSEContract_EndpointSpec(t *testing.T) {
	contract := RequestDetailSSEContract()

	assert.Equal(t, "/admin/requests/:request_id/preview", contract.EndpointPath)
	assert.Equal(t, "text/event-stream", contract.ContentType)
	assert.Equal(t, []string{"preview.replay", "preview.chunk", "preview.completed"}, contract.EventTypes)
	assert.True(t, contract.ReplayOmitsTerminalDoneEvent)
	assert.True(t, contract.IncrementalOmitsTerminalDoneEvent)
	assert.True(t, contract.ConnectAfterCompletionFallsBackToStaticFetch)
}

func TestRequestDetailSSEContract_FallbackWhenSSEUnavailable(t *testing.T) {
	contract := RequestDetailSSEContract()

	assert.Equal(t, "static-fetch", contract.FallbackMode)
	assert.Equal(t, "load persisted request detail once when SSE cannot connect", contract.FallbackBehavior)
	assert.False(t, contract.FallbackUsesExecutionPreview)
	assert.False(t, contract.FallbackStartsSecondaryLivePollingLoop)
}
