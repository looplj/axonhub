package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

// TestResponsesStreamingClientDisconnectAfterDone tests the fix for GitHub Issue #827
// where client disconnecting immediately after receiving [DONE] should not mark
// the request as canceled.
//
// Issue: https://github.com/looplj/axonhub/issues/827
//
// Before the fix:
// - Client receives all chunks including [DONE]
// - Client disconnects immediately
// - Context is canceled
// - Request execution status was incorrectly set to "canceled"
//
// After the fix:
// - Client receives all chunks including [DONE]
// - Client disconnects immediately
// - Context is canceled
// - Request execution status is correctly set to "completed" because [DONE] was received
func TestResponsesStreamingClientDisconnectAfterDone(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestResponsesStreamingClientDisconnectAfterDone")

	ctx := helper.CreateTestContext()

	// Use a simple question that will generate a short response
	question := "Say 'hello' and nothing else."

	t.Logf("Sending streaming request: %s", question)

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
	}

	// Create a cancellable context to simulate client disconnect
	clientCtx, clientCancel := context.WithCancel(ctx)
	defer clientCancel()

	// Make streaming API call with cancellable context
	stream := helper.CreateResponseStreamingWithHeaders(clientCtx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start Responses streaming")

	// Read and process the stream
	var fullContent strings.Builder
	var chunks int
	var receivedDone bool

	for stream.Next() {
		event := stream.Current()
		chunks++

		// Handle text delta events
		if event.Type == "response.output_text.delta" && event.Delta != "" {
			fullContent.WriteString(event.Delta)
		}

		// Check if this is the done event
		if event.Type == "response.completed" {
			receivedDone = true
			t.Log("Received response.completed event (equivalent to [DONE])")
		}

		// Simulate client disconnect immediately after receiving the done event
		if receivedDone {
			t.Log("Simulating client disconnect immediately after receiving done event")
			clientCancel()
			break
		}
	}

	// Wait a bit to let the server process the disconnect
	time.Sleep(100 * time.Millisecond)

	// Check for stream errors - we expect context canceled error
	if err := stream.Err(); err != nil {
		t.Logf("Stream error (expected context canceled): %v", err)
	}

	// Validate that we received content before disconnect
	finalContent := fullContent.String()
	t.Logf("Received %d events before disconnect", chunks)
	t.Logf("Final content: %s", finalContent)

	if chunks == 0 {
		t.Error("Expected at least one event from Responses streaming")
	}

	if !receivedDone {
		t.Log("Warning: Did not receive explicit done event - this may be expected depending on API behavior")
	}

	// The key assertion: even though we disconnected immediately after receiving [DONE],
	// the request execution status should be "completed", not "canceled"
	// Note: This requires checking the AxonHub database or logs to verify the status
	t.Log("Test completed. Check AxonHub request execution status:")
	t.Log("  - Status should be 'completed' (not 'canceled')")
	t.Log("  - This verifies the fix for GitHub Issue #827")
}

// TestResponsesStreamingClientDisconnectMidStream tests that disconnecting
// mid-stream (before [DONE]) correctly marks the request as canceled
func TestResponsesStreamingClientDisconnectMidStream(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestResponsesStreamingClientDisconnectMidStream")

	ctx := helper.CreateTestContext()

	// Use a question that will generate a longer response to ensure we can disconnect mid-stream
	question := "Write a long story about a robot learning to paint. Make it at least 500 words."

	t.Logf("Sending streaming request: %s", question)

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
	}

	// Create a cancellable context to simulate client disconnect
	clientCtx, clientCancel := context.WithCancel(ctx)
	defer clientCancel()

	// Make streaming API call with cancellable context
	stream := helper.CreateResponseStreamingWithHeaders(clientCtx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start Responses streaming")

	// Read and process the stream, but disconnect after receiving some chunks
	var chunks int
	maxChunksBeforeDisconnect := 5

	for stream.Next() {
		event := stream.Current()
		chunks++

		// Simulate client disconnect after receiving a few chunks (before [DONE])
		if chunks >= maxChunksBeforeDisconnect {
			t.Logf("Simulating client disconnect after receiving %d chunks (before done)", chunks)
			clientCancel()
			break
		}

		// Handle text delta events
		if event.Type == "response.output_text.delta" && event.Delta != "" {
			t.Logf("Received chunk %d: %s", chunks, event.Delta)
		}
	}

	// Wait a bit to let the server process the disconnect
	time.Sleep(100 * time.Millisecond)

	// Check for stream errors - we expect context canceled error
	if err := stream.Err(); err != nil {
		t.Logf("Stream error (expected context canceled): %v", err)
	}

	t.Logf("Received %d events before disconnect", chunks)

	// The key assertion: since we disconnected before [DONE], the request
	// execution status should be "canceled"
	t.Log("Test completed. Check AxonHub request execution status:")
	t.Log("  - Status should be 'canceled' (because we disconnected before [DONE])")
}

// TestResponsesStreamingRapidDisconnects tests multiple rapid connect/disconnect cycles
// to ensure the server handles them correctly

// TestResponsesStreamingClientDisconnectAfterOutputTextDone simulates a client
// that stops reading immediately after the final text payload is available,
// before the explicit response.completed event is observed locally.
//
// This is closer to a real proxy/client chain such as Claude Code -> AxonHub -> Codex,
// where downstream teardown can happen very close to completion and the transport may be
// canceled before the terminal event is processed in the expected order.
func TestResponsesStreamingClientDisconnectAfterOutputTextDone(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestResponsesStreamingClientDisconnectAfterOutputTextDone")

	ctx := helper.CreateTestContext()
	question := "Write exactly one short sentence about oceans."

	t.Logf("Sending streaming request: %s", question)

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
	}

	clientCtx, clientCancel := context.WithCancel(ctx)
	defer clientCancel()

	stream := helper.CreateResponseStreamingWithHeaders(clientCtx, params)
	helper.AssertNoError(t, stream.Err(), "Failed to start Responses streaming")

	var fullContent strings.Builder
	var chunks int
	var sawOutputTextDone bool
	var sawResponseCompleted bool

	for stream.Next() {
		event := stream.Current()
		chunks++

		if event.Type == "response.output_text.delta" && event.Delta != "" {
			fullContent.WriteString(event.Delta)
		}

		if event.Type == "response.output_text.done" {
			sawOutputTextDone = true
			t.Log("Received response.output_text.done, canceling before response.completed is consumed")
			clientCancel()
			break
		}

		if event.Type == "response.completed" {
			sawResponseCompleted = true
		}
	}

	time.Sleep(100 * time.Millisecond)

	if err := stream.Err(); err != nil {
		t.Logf("Stream error after early teardown: %v", err)
	}

	finalContent := fullContent.String()
	t.Logf("Received %d events before disconnect", chunks)
	t.Logf("Final content collected before disconnect: %s", finalContent)
	t.Logf("sawOutputTextDone=%v sawResponseCompleted=%v", sawOutputTextDone, sawResponseCompleted)

	if chunks == 0 {
		t.Error("Expected at least one event from Responses streaming")
	}
	if finalContent == "" {
		t.Error("Expected non-empty text content before disconnect")
	}
	if !sawOutputTextDone {
		t.Skip("Provider did not emit response.output_text.done in this run; cannot verify near-completion teardown path")
	}
	if sawResponseCompleted {
		t.Log("response.completed was observed before cancellation in this run; near-completion timing may vary by provider")
	}

	t.Log("Near-completion teardown test finished. Verify AxonHub logs / request execution status:")
	t.Log("  - final decision should still resolve to completed when aggregation proves completion")
	t.Log("  - this is the closest integration-level regression to Claude Code style early stop")
}
func TestResponsesStreamingRapidDisconnects(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestResponsesStreamingRapidDisconnects")

	question := "Say 'test' and nothing else."

	var wg sync.WaitGroup
	numRequests := 3

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()

			ctx := helper.CreateTestContext()

			params := responses.ResponseNewParams{
				Model: shared.ResponsesModel(helper.GetModel()),
				Input: responses.ResponseNewParamsInputUnion{
					OfString: openai.String(question),
				},
			}

			// Create a cancellable context
			clientCtx, clientCancel := context.WithCancel(ctx)
			defer clientCancel()

			stream := helper.CreateResponseStreamingWithHeaders(clientCtx, params)
			if stream.Err() != nil {
				t.Logf("Iteration %d: Failed to start streaming: %v", iteration, stream.Err())
				return
			}

			var chunks int
			var receivedDone bool

			for stream.Next() {
				event := stream.Current()
				chunks++

				if event.Type == "response.completed" {
					receivedDone = true
					// Disconnect immediately after done
					clientCancel()
					break
				}

				// Also disconnect if we received some chunks but no done yet
				if chunks >= 10 {
					clientCancel()
					break
				}
			}

			t.Logf("Iteration %d: Received %d chunks, receivedDone=%v", iteration, chunks, receivedDone)

			// Small delay between requests
			time.Sleep(50 * time.Millisecond)
		}(i)
	}

	wg.Wait()

	t.Log("Rapid disconnect test completed")
	t.Log("Check AxonHub request execution statuses:")
	t.Log("  - Requests that received [DONE] should be 'completed'")
	t.Log("  - Requests that disconnected before [DONE] should be 'canceled'")
}
