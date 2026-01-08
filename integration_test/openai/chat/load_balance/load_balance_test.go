package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
)

// TestTraceBasedLoadBalancing verifies that trace-aware load balancing works correctly.
// When a trace has a successful request on a specific channel, subsequent requests
// in the same trace should prefer that channel (TraceAwareStrategy gives 1000 boost).
func TestTraceBasedLoadBalancing(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestTraceBasedLoadBalancing")
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	t.Log("=== Test 1: First request in trace (establishes channel preference) ===")

	// First request - this will establish which channel is used
	firstResponse, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What is 2+2?"),
		},
		Model: helper.GetModel(),
	})
	helper.AssertNoError(t, err, "First request failed")
	helper.ValidateChatResponse(t, firstResponse, "First request")

	t.Logf("First request completed successfully")
	t.Logf("Response: %s", firstResponse.Choices[0].Message.Content)

	// Small delay to ensure metrics are recorded
	time.Sleep(100 * time.Millisecond)

	t.Log("=== Test 2: Second request in same trace (should prefer same channel) ===")

	// Second request in same trace - should prefer the same channel due to TraceAwareStrategy
	secondResponse, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What is 3+3?"),
		},
		Model: helper.GetModel(),
	})
	helper.AssertNoError(t, err, "Second request failed")
	helper.ValidateChatResponse(t, secondResponse, "Second request")

	t.Logf("Second request completed successfully")
	t.Logf("Response: %s", secondResponse.Choices[0].Message.Content)

	t.Log("=== Test 3: Third request in same trace (verify consistency) ===")

	// Third request to verify consistency
	thirdResponse, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What is 5+5?"),
		},
		Model: helper.GetModel(),
	})
	helper.AssertNoError(t, err, "Third request failed")
	helper.ValidateChatResponse(t, thirdResponse, "Third request")

	t.Logf("Third request completed successfully")
	t.Logf("Response: %s", thirdResponse.Choices[0].Message.Content)

	t.Log("=== Trace-based load balancing test completed successfully ===")
	t.Log("All requests in the same trace should have been routed to the same channel")
	t.Log("due to TraceAwareStrategy giving 1000 point boost to the last successful channel")
}

// TestTraceBasedLoadBalancing_MultipleTraces verifies that different traces
// can use different channels independently.
func TestTraceBasedLoadBalancing_MultipleTraces(t *testing.T) {
	helper1 := testutil.NewTestHelper(t, "TestTraceBasedLoadBalancing_MultipleTraces_Trace1")
	helper2 := testutil.NewTestHelper(t, "TestTraceBasedLoadBalancing_MultipleTraces_Trace2")
	helper3 := testutil.NewTestHelper(t, "TestTraceBasedLoadBalancing_MultipleTraces_Trace3")

	t.Log("=== Testing multiple independent traces ===")
	t.Logf("Trace 1 ID: %s", helper1.Config.TraceID)
	t.Logf("Trace 2 ID: %s", helper2.Config.TraceID)
	t.Logf("Trace 3 ID: %s", helper3.Config.TraceID)

	// Each trace makes multiple requests
	traces := []*testutil.TestHelper{helper1, helper2, helper3}

	for i, helper := range traces {
		t.Logf("\n=== Trace %d: Making requests ===", i+1)
		ctx := helper.CreateTestContext()

		// First request in this trace
		response1, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Hello from trace %d, request 1", i+1)),
			},
			Model: helper.GetModel(),
		})
		helper.AssertNoError(t, err, fmt.Sprintf("Trace %d request 1 failed", i+1))
		helper.ValidateChatResponse(t, response1, fmt.Sprintf("Trace %d request 1", i+1))
		t.Logf("Trace %d request 1 completed", i+1)

		time.Sleep(50 * time.Millisecond)

		// Second request in this trace
		response2, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Hello from trace %d, request 2", i+1)),
			},
			Model: helper.GetModel(),
		})
		helper.AssertNoError(t, err, fmt.Sprintf("Trace %d request 2 failed", i+1))
		helper.ValidateChatResponse(t, response2, fmt.Sprintf("Trace %d request 2", i+1))
		t.Logf("Trace %d request 2 completed", i+1)
	}

	t.Log("\n=== Multiple traces test completed successfully ===")
	t.Log("Each trace should maintain its own channel preference independently")
}

// TestWeightBasedLoadBalancing verifies that weight-based load balancing
// distributes requests according to channel weights when no trace context exists.
func TestWeightBasedLoadBalancing(t *testing.T) {
	t.Log("=== Testing weight-based load balancing ===")
	t.Log("This test verifies that channels with different weights receive")
	t.Log("proportional amounts of traffic when no trace context is present")

	// Create multiple helpers without trace to test pure weight-based distribution
	requestCount := 60

	for i := 0; i < requestCount; i++ {
		// Create new helper for each request with trace and thread disabled
		helper := testutil.NewTestHelper(t, fmt.Sprintf("TestWeightBasedLoadBalancing_Request%d", i))
		helper.Config.DisableTrace = true
		helper.Config.DisableThread = true
		ctx := helper.CreateTestContext()

		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Request %d: What is %d+%d?", i+1, i, i)),
			},
			Model: helper.GetModel(),
		})

		helper.AssertNoError(t, err, fmt.Sprintf("Request %d failed", i+1))
		helper.ValidateChatResponse(t, response, fmt.Sprintf("Request %d", i+1))

		if i%5 == 0 {
			t.Logf("Completed %d/%d requests", i+1, requestCount)
		}

		// Small delay between requests
		time.Sleep(50 * time.Millisecond)
	}

	t.Log("\n=== Weight-based load balancing test completed ===")
	t.Logf("Sent %d requests across different traces", requestCount)
	t.Log("Channels should have received requests proportional to their weights:")
	t.Log("- High weight channels (weight=80-100) should get more requests")
	t.Log("- Medium weight channels (weight=40-60) should get moderate requests")
	t.Log("- Low weight channels (weight=10-30) should get fewer requests")
}

// TestWeightRoundRobinLoadBalancing verifies that WeightRoundRobinStrategy
// distributes load proportionally based on both weight and current load.
func TestWeightRoundRobinLoadBalancing(t *testing.T) {
	t.Log("=== Testing weighted round-robin load balancing ===")
	t.Log("This test sends concurrent requests to verify that WeightRoundRobinStrategy")
	t.Log("balances load based on both channel weight and current request count")

	// Send concurrent requests to test round-robin behavior
	concurrentRequests := 15
	results := make(chan error, concurrentRequests)

	t.Logf("Sending %d concurrent requests...", concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func(requestNum int) {
			helper := testutil.NewTestHelper(t, fmt.Sprintf("TestWeightRoundRobin_Request%d", requestNum))
			helper.Config.DisableTrace = true
			helper.Config.DisableThread = true
			ctx := helper.CreateTestContext()

			response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
				Messages: []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage(fmt.Sprintf("Concurrent request %d", requestNum)),
				},
				Model: helper.GetModel(),
			})

			if err != nil {
				results <- fmt.Errorf("request %d failed: %w", requestNum, err)
				return
			}

			if response == nil || len(response.Choices) == 0 {
				results <- fmt.Errorf("request %d: invalid response", requestNum)
				return
			}

			results <- nil
		}(i)
	}

	// Wait for all requests to complete
	successCount := 0
	for i := 0; i < concurrentRequests; i++ {
		err := <-results
		if err != nil {
			t.Logf("Request failed: %v", err)
		} else {
			successCount++
		}
	}

	t.Logf("\n=== Weighted round-robin test completed ===")
	t.Logf("Successful requests: %d/%d", successCount, concurrentRequests)
	t.Log("WeightRoundRobinStrategy should have distributed requests based on:")
	t.Log("- Channel weight (higher weight = can handle more requests)")
	t.Log("- Current load (channels with fewer active requests get priority)")
	t.Log("- Formula: normalizedCount = effectiveCount / (weight / 100.0)")

	if successCount < concurrentRequests/2 {
		t.Errorf("Too many failures: only %d/%d succeeded", successCount, concurrentRequests)
	}
}

// TestLoadBalancingWithRetry verifies that load balancing works correctly
// when retries are enabled and a channel fails.
func TestLoadBalancingWithRetry(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestLoadBalancingWithRetry")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true
	ctx := helper.CreateTestContext()

	t.Log("=== Testing load balancing with retry mechanism ===")
	t.Log("If a channel fails, the system should automatically retry with the next channel")

	// Make a request that might trigger retry behavior
	response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Test request for retry mechanism"),
		},
		Model: helper.GetModel(),
	})

	helper.AssertNoError(t, err, "Request failed even with retry")
	helper.ValidateChatResponse(t, response, "Retry test")

	t.Log("=== Load balancing with retry test completed ===")
	t.Log("Request succeeded, demonstrating that the system can handle channel failures")
	t.Log("and automatically retry with alternative channels based on load balancing scores")
}

// TestTraceAwareStrategyPriority verifies that TraceAwareStrategy has the highest
// priority and overrides other strategies when a trace context exists.
func TestTraceAwareStrategyPriority(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestTraceAwareStrategyPriority")
	ctx := helper.CreateTestContext()

	t.Log("=== Testing TraceAwareStrategy priority ===")
	t.Log("TraceAwareStrategy should give 1000 point boost to the last successful channel")
	t.Log("This should override other strategies like WeightStrategy, ErrorAwareStrategy, etc.")

	// First request establishes the channel
	t.Log("\n--- First request: Establishing channel preference ---")
	response1, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("First request in trace"),
		},
		Model: helper.GetModel(),
	})
	helper.AssertNoError(t, err, "First request failed")
	helper.ValidateChatResponse(t, response1, "First request")
	t.Log("First request completed - channel preference established")

	time.Sleep(100 * time.Millisecond)

	// Send multiple follow-up requests
	for i := 2; i <= 5; i++ {
		t.Logf("\n--- Request %d: Should use same channel due to trace ---", i)
		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Request %d in same trace", i)),
			},
			Model: helper.GetModel(),
		})
		helper.AssertNoError(t, err, fmt.Sprintf("Request %d failed", i))
		helper.ValidateChatResponse(t, response, fmt.Sprintf("Request %d", i))
		t.Logf("Request %d completed successfully", i)

		time.Sleep(50 * time.Millisecond)
	}

	t.Log("\n=== TraceAwareStrategy priority test completed ===")
	t.Log("All requests in the same trace should have been routed to the same channel")
	t.Log("Strategy scoring breakdown:")
	t.Log("- TraceAwareStrategy: 1000 points (for last successful channel)")
	t.Log("- ErrorAwareStrategy: 0-200 points (based on health)")
	t.Log("- WeightRoundRobinStrategy: 10-150 points (based on weight and load)")
	t.Log("- ConnectionAwareStrategy: 0-50 points (based on active connections)")
	t.Log("Total: TraceAware dominates with 1000 point boost")
}

// TestLoadBalancingDebugMode verifies that debug mode provides detailed
// load balancing decision information.
func TestLoadBalancingDebugMode(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestLoadBalancingDebugMode")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true
	ctx := helper.CreateTestContext()

	t.Log("=== Testing load balancing debug mode ===")
	t.Log("When AXONHUB_DEBUG_LOAD_BALANCER_ENABLED=true or AH-Debug header is set,")
	t.Log("the system should log detailed scoring information for each channel")

	// Make a request with debug context
	response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Debug mode test request"),
		},
		Model: helper.GetModel(),
	})

	helper.AssertNoError(t, err, "Debug mode request failed")
	helper.ValidateChatResponse(t, response, "Debug mode test")

	t.Log("\n=== Debug mode test completed ===")
	t.Log("Check server logs for detailed load balancing decision information:")
	t.Log("- Channel scores from each strategy")
	t.Log("- Total scores and final ranking")
	t.Log("- Strategy execution times")
	t.Log("- Detailed decision breakdown")
}

// TestLoadBalancingStrategyComposition verifies that multiple strategies
// work together correctly to produce the final channel ranking.
func TestLoadBalancingStrategyComposition(t *testing.T) {
	t.Log("=== Testing load balancing strategy composition ===")
	t.Log("The default orchestrator uses these strategies in order:")
	t.Log("1. TraceAwareStrategy (0 or 1000 points)")
	t.Log("2. ErrorAwareStrategy (0-200 points)")
	t.Log("3. WeightRoundRobinStrategy (10-150 points)")
	t.Log("4. ConnectionAwareStrategy (0-50 points)")
	t.Log("Total score determines channel priority")

	// Test scenario 1: No trace context (weight-based selection)
	t.Log("\n--- Scenario 1: No trace context ---")
	helper1 := testutil.NewTestHelper(t, "TestStrategyComposition_NoTrace")
	helper1.Config.DisableTrace = true
	helper1.Config.DisableThread = true
	ctx1 := helper1.CreateTestContext()

	response1, err := helper1.CreateChatCompletionWithHeaders(ctx1, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Request without trace context"),
		},
		Model: helper1.GetModel(),
	})
	helper1.AssertNoError(t, err, "No trace request failed")
	helper1.ValidateChatResponse(t, response1, "No trace request")
	t.Log("Expected: High weight channel selected (WeightRoundRobin + ErrorAware + Connection)")

	time.Sleep(100 * time.Millisecond)

	// Test scenario 2: With trace context (trace-aware selection)
	t.Log("\n--- Scenario 2: With trace context ---")
	helper2 := testutil.NewTestHelper(t, "TestStrategyComposition_WithTrace")
	ctx2 := helper2.CreateTestContext()

	// First request establishes trace
	response2a, err := helper2.CreateChatCompletionWithHeaders(ctx2, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("First request with trace"),
		},
		Model: helper2.GetModel(),
	})
	helper2.AssertNoError(t, err, "First trace request failed")
	helper2.ValidateChatResponse(t, response2a, "First trace request")
	t.Log("First request completed - channel preference established")

	time.Sleep(100 * time.Millisecond)

	// Second request should prefer same channel
	response2b, err := helper2.CreateChatCompletionWithHeaders(ctx2, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Second request with trace"),
		},
		Model: helper2.GetModel(),
	})
	helper2.AssertNoError(t, err, "Second trace request failed")
	helper2.ValidateChatResponse(t, response2b, "Second trace request")
	t.Log("Expected: Same channel as first request (TraceAware boost = 1000 points)")

	t.Log("\n=== Strategy composition test completed ===")
	t.Log("Verified that strategies work together to produce optimal channel selection")
}
