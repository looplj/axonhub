package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// mockChannelService is a mock implementation of ChannelService for testing
type mockChannelService struct{}

func (m *mockChannelService) AsyncRecordPerformance(ctx context.Context, perf *biz.PerformanceRecord) {
	// No-op for testing
}

// TestPerformanceRecording_OnInboundLlmRequest_SetsStreamFlag verifies that
// the Stream flag is correctly set based on the request's Stream field.
func TestPerformanceRecording_OnInboundLlmRequest_SetsStreamFlag(t *testing.T) {
	tests := []struct {
		name         string
		streamValue  *bool
		expectedFlag bool
	}{
		{
			name:         "streaming request - Stream is true",
			streamValue:  boolPtr(true),
			expectedFlag: true,
		},
		{
			name:         "non-streaming request - Stream is false",
			streamValue:  boolPtr(false),
			expectedFlag: false,
		},
		{
			name:         "nil stream value defaults to false",
			streamValue:  nil,
			expectedFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			state := &PersistenceState{
				Perf: nil,
			}
			outbound := &PersistentOutboundTransformer{
				state: state,
			}
			middleware := &performanceRecording{
				outbound: outbound,
			}

			request := &llm.Request{
				Model:  "gpt-4",
				Stream: tt.streamValue,
			}
			ctx := context.Background()

			// Execute
			result, err := middleware.OnInboundLlmRequest(ctx, request)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, request, result)
			assert.NotNil(t, state.Perf)
			assert.Equal(t, tt.expectedFlag, state.Perf.Stream)
		})
	}
}

// TestPerformanceRecording_OnOutboundRawRequest_PreservesStreamFlag verifies that
// the Stream flag set in OnInboundLlmRequest is preserved when OnOutboundRawRequest
// creates a new PerformanceRecord. This test would FAIL if the bug were reverted.
func TestPerformanceRecording_OnOutboundRawRequest_PreservesStreamFlag(t *testing.T) {
	tests := []struct {
		name              string
		initialStreamFlag bool
		expectedStream    bool
	}{
		{
			name:              "streaming request preserves Stream=true",
			initialStreamFlag: true,
			expectedStream:    true,
		},
		{
			name:              "non-streaming request preserves Stream=false",
			initialStreamFlag: false,
			expectedStream:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:   1,
					Name: "test-channel",
				},
				Outbound: &mockTransformer{},
			}

			// Simulate that OnInboundLlmRequest already set the Stream flag
			state := &PersistenceState{
				Perf: &biz.PerformanceRecord{
					Stream: tt.initialStreamFlag,
				},
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
				},
			}
			outbound := &PersistentOutboundTransformer{
				state: state,
			}
			middleware := &performanceRecording{
				outbound: outbound,
			}

			request := &httpclient.Request{
				Method: "POST",
				URL:    "https://api.example.com/v1/chat/completions",
			}
			ctx := context.Background()

			// Execute - this is where the bug would cause Stream flag to be lost
			result, err := middleware.OnOutboundRawRequest(ctx, request)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, request, result)
			assert.NotNil(t, state.Perf)

			// CRITICAL: This assertion verifies the fix. If the bug were reverted
			// (i.e., creating a new PerformanceRecord without preserving Stream),
			// this test would FAIL.
			assert.Equal(t, tt.expectedStream, state.Perf.Stream,
				"Stream flag should be preserved from OnInboundLlmRequest through OnOutboundRawRequest. "+
					"If this fails, the bug has been reverted!")
		})
	}
}

// TestPerformanceRecording_FullLifecycle_StreamFlagPreserved tests the complete
// middleware lifecycle to ensure Stream flag is preserved end-to-end.
func TestPerformanceRecording_FullLifecycle_StreamFlagPreserved(t *testing.T) {
	// Setup
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test-channel",
		},
		Outbound: &mockTransformer{},
	}

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
		},
	}
	outbound := &PersistentOutboundTransformer{
		state: state,
	}
	middleware := &performanceRecording{
		outbound: outbound,
	}

	ctx := context.Background()
	streamValue := true

	// Step 1: Inbound request processing
	llmRequest := &llm.Request{
		Model:  "gpt-4",
		Stream: &streamValue,
	}
	_, err := middleware.OnInboundLlmRequest(ctx, llmRequest)
	require.NoError(t, err)
	require.NotNil(t, state.Perf)
	assert.True(t, state.Perf.Stream, "Stream flag should be true after OnInboundLlmRequest")

	// Step 2: Outbound request processing (this is where the bug occurred)
	httpRequest := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
	}
	_, err = middleware.OnOutboundRawRequest(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, state.Perf)

	// CRITICAL: Stream flag must still be true after OnOutboundRawRequest
	assert.True(t, state.Perf.Stream,
		"Stream flag should be preserved through OnOutboundRawRequest. "+
			"This assertion would FAIL if the bug (8afd95c3) were reverted.")
}

// TestPerformanceRecording_OnOutboundRawRequest_NoExistingPerf verifies that
// OnOutboundRawRequest works correctly even if OnInboundLlmRequest wasn't called
// (edge case where Perf is nil).
func TestPerformanceRecording_OnOutboundRawRequest_NoExistingPerf(t *testing.T) {
	// Setup
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test-channel",
		},
		Outbound: &mockTransformer{},
	}

	state := &PersistenceState{
		Perf: nil, // No existing PerformanceRecord
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
		},
	}
	outbound := &PersistentOutboundTransformer{
		state: state,
	}
	middleware := &performanceRecording{
		outbound: outbound,
	}

	request := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
	}
	ctx := context.Background()

	// Execute
	result, err := middleware.OnOutboundRawRequest(ctx, request)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, request, result)
	assert.NotNil(t, state.Perf)
	assert.False(t, state.Perf.Stream, "Stream should default to false when no existing Perf")
}

// TestPerformanceRecording_OnOutboundRawRequest_NoChannel verifies that
// OnOutboundRawRequest returns early when there's no channel.
func TestPerformanceRecording_OnOutboundRawRequest_NoChannel(t *testing.T) {
	// Setup
	state := &PersistenceState{
		Perf:             &biz.PerformanceRecord{Stream: true},
		CurrentCandidate: nil, // No channel
	}
	outbound := &PersistentOutboundTransformer{
		state: state,
	}
	middleware := &performanceRecording{
		outbound: outbound,
	}

	request := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
	}
	ctx := context.Background()

	// Execute
	result, err := middleware.OnOutboundRawRequest(ctx, request)

	// Assert - should return early without modifying Perf
	require.NoError(t, err)
	assert.Equal(t, request, result)
	// Perf should remain unchanged since we returned early
	assert.True(t, state.Perf.Stream)
}

// TestPerformanceRecording_StreamFlagBugRegression is specifically designed to
// catch the regression if the fix is reverted. It documents the exact bug scenario.
func TestPerformanceRecording_StreamFlagBugRegression(t *testing.T) {
	// This test documents the bug introduced in commit 8afd95c3:
	// "feat: trace stikcy api key for multiple api keys channel"
	//
	// The bug: OnOutboundRawRequest() was creating a new PerformanceRecord without
	// preserving the Stream flag that was set in OnInboundLlmRequest().
	//
	// The fix: Preserve the Stream flag before creating the new PerformanceRecord.

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test-channel",
		},
		Outbound: &mockTransformer{},
	}

	// Simulate the state after OnInboundLlmRequest was called with stream=true
	state := &PersistenceState{
		Perf: &biz.PerformanceRecord{
			Stream: true, // Set by OnInboundLlmRequest
		},
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
		},
	}
	outbound := &PersistentOutboundTransformer{
		state: state,
	}
	middleware := &performanceRecording{
		outbound: outbound,
	}

	ctx := context.Background()
	request := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
	}

	// Execute OnOutboundRawRequest
	_, err := middleware.OnOutboundRawRequest(ctx, request)
	require.NoError(t, err)

	// The bug would cause this assertion to fail because:
	// 1. OnInboundLlmRequest set Perf.Stream = true
	// 2. OnOutboundRawRequest created a new PerformanceRecord{}
	// 3. The new PerformanceRecord had Stream = false (zero value)
	// 4. This overwrote the Perf pointer, losing the Stream flag
	//
	// The fix preserves Stream from the old Perf before creating the new one.
	assert.True(t, state.Perf.Stream,
		"BUG REGRESSION DETECTED: Stream flag was lost in OnOutboundRawRequest. "+
			"This indicates the fix from commit 8afd95c3 has been reverted.")
}

// TestRecordPerformanceStream_MarksFirstToken verifies that recordPerformanceStream
// correctly marks the first token time.

// Helper function for creating bool pointers
func boolPtr(b bool) *bool {
	return &b
}
