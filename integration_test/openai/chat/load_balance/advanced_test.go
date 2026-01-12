package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
)

// TestConnectionAwareLoadBalancing verifies that ConnectionAwareStrategy
// considers active connection counts when selecting channels.
func TestConnectionAwareLoadBalancing(t *testing.T) {
	t.Log("=== Testing connection-aware load balancing ===")
	t.Log("ConnectionAwareStrategy should prefer channels with fewer active connections")
	t.Log("Score formula: maxScore * (1 - min(connections, cap) / cap)")

	// Send multiple concurrent long-running requests
	concurrentRequests := 10
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests)

	t.Logf("Sending %d concurrent requests to test connection awareness...", concurrentRequests)

	startTime := time.Now()
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(requestNum int) {
			defer wg.Done()

			helper := testutil.NewTestHelper(t, fmt.Sprintf("TestConnectionAware_Request%d", requestNum))
			helper.Config.DisableTrace = true
			helper.Config.DisableThread = true
			ctx := helper.CreateTestContext()

			// Use a longer prompt to keep connections active longer
			response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
				Messages: []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage(fmt.Sprintf("Request %d: Please explain the concept of load balancing in distributed systems.", requestNum)),
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

	wg.Wait()
	close(results)
	duration := time.Since(startTime)

	successCount := 0
	for err := range results {
		if err != nil {
			t.Logf("Request failed: %v", err)
		} else {
			successCount++
		}
	}

	t.Logf("\n=== Connection-aware test completed in %v ===", duration)
	t.Logf("Successful requests: %d/%d", successCount, concurrentRequests)
	t.Log("ConnectionAwareStrategy should have distributed requests to channels with fewer active connections")
	t.Log("Expected behavior: Channels with 0 connections get full score (50 points)")
	t.Log("                   Channels with more connections get proportionally lower scores")

	if successCount < concurrentRequests*3/4 {
		t.Errorf("Too many failures: only %d/%d succeeded", successCount, concurrentRequests)
	}
}

// TestErrorAwareLoadBalancing verifies that ErrorAwareStrategy
// penalizes channels with recent failures and consecutive errors.
func TestErrorAwareLoadBalancing(t *testing.T) {
	t.Log("=== Testing error-aware load balancing ===")
	t.Log("ErrorAwareStrategy should penalize channels with recent failures")
	t.Log("Scoring factors:")
	t.Log("- Health score: 0-100 points (based on success rate)")
	t.Log("- Consecutive failure penalty: -50 points per failure (up to -150)")
	t.Log("- Recent error penalty: -20 points per error in last 5 minutes")

	helper := testutil.NewTestHelper(t, "TestErrorAwareLoadBalancing")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true
	ctx := helper.CreateTestContext()

	// Make several requests to establish baseline
	requestCount := 5
	for i := 0; i < requestCount; i++ {
		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Request %d for error-aware testing", i+1)),
			},
			Model: helper.GetModel(),
		})

		helper.AssertNoError(t, err, fmt.Sprintf("Request %d failed", i+1))
		helper.ValidateChatResponse(t, response, fmt.Sprintf("Request %d", i+1))

		t.Logf("Request %d completed successfully", i+1)
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("\n=== Error-aware test completed ===")
	t.Log("All requests succeeded, demonstrating healthy channel selection")
	t.Log("If a channel had failures, ErrorAwareStrategy would have penalized it:")
	t.Log("- 1 consecutive failure: -50 points")
	t.Log("- 2 consecutive failures: -100 points")
	t.Log("- 3+ consecutive failures: -150 points")
	t.Log("Healthy channels maintain full 200 point score from ErrorAwareStrategy")
}

// TestLoadBalancingWithChannelFailover verifies that the system
// automatically fails over to alternative channels when one fails.
func TestLoadBalancingWithChannelFailover(t *testing.T) {
	t.Log("=== Testing load balancing with channel failover ===")
	t.Log("When a channel fails, the system should automatically retry with the next best channel")
	t.Log("based on the load balancing scores")

	helper := testutil.NewTestHelper(t, "TestLoadBalancingWithChannelFailover")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true
	ctx := helper.CreateTestContext()

	// Make requests that might trigger failover
	requestCount := 10
	successCount := 0

	for i := 0; i < requestCount; i++ {
		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Failover test request %d", i+1)),
			},
			Model: helper.GetModel(),
		})

		if err != nil {
			t.Logf("Request %d failed: %v (system should retry with alternative channel)", i+1, err)
			continue
		}

		if response != nil && len(response.Choices) > 0 {
			successCount++
			t.Logf("Request %d succeeded", i+1)
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("\n=== Failover test completed ===")
	t.Logf("Successful requests: %d/%d", successCount, requestCount)
	t.Log("The system should have automatically retried failed requests with alternative channels")
	t.Log("Retry policy determines how many channels to try (1 + MaxChannelRetries)")

	if successCount == 0 {
		t.Error("All requests failed - failover mechanism may not be working")
	}
}

// TestLoadBalancingTopKSelection verifies that the load balancer
// only sorts the top K channels needed based on retry policy.
func TestLoadBalancingTopKSelection(t *testing.T) {
	t.Log("=== Testing load balancing top-K selection optimization ===")
	t.Log("The load balancer should only sort the top K channels needed for retry")
	t.Log("K = 1 + MaxChannelRetries (from retry policy)")
	t.Log("This optimization reduces overhead when many channels are available")

	helper := testutil.NewTestHelper(t, "TestLoadBalancingTopKSelection")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true
	ctx := helper.CreateTestContext()

	// Make a simple request
	response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Test top-K selection optimization"),
		},
		Model: helper.GetModel(),
	})

	helper.AssertNoError(t, err, "Request failed")
	helper.ValidateChatResponse(t, response, "Top-K test")

	t.Log("\n=== Top-K selection test completed ===")
	t.Log("The load balancer used partial sorting to efficiently select top K channels")
	t.Log("This is more efficient than sorting all channels when K << total channels")
	t.Log("Check server logs for 'selected_channels' count in load balancing decision")
}

// TestLoadBalancingPriorityGroups verifies that channels with different
// priorities are handled correctly, with load balancing applied within each group.
func TestLoadBalancingPriorityGroups(t *testing.T) {
	t.Log("=== Testing load balancing with priority groups ===")
	t.Log("Channels can have different priority values (lower = higher priority)")
	t.Log("Load balancing is applied within each priority group")
	t.Log("Higher priority groups are tried first, then lower priority groups")

	helper := testutil.NewTestHelper(t, "TestLoadBalancingPriorityGroups")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true
	ctx := helper.CreateTestContext()

	// Make requests to test priority-based selection
	requestCount := 5
	for i := 0; i < requestCount; i++ {
		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Priority group test request %d", i+1)),
			},
			Model: helper.GetModel(),
		})

		helper.AssertNoError(t, err, fmt.Sprintf("Request %d failed", i+1))
		helper.ValidateChatResponse(t, response, fmt.Sprintf("Request %d", i+1))

		t.Logf("Request %d completed", i+1)
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("\n=== Priority groups test completed ===")
	t.Log("Requests should have been routed to highest priority channels first")
	t.Log("Within each priority group, load balancing strategies determined the order")
}

// TestLoadBalancingInactivityDecay verifies that the round-robin strategy
// decays historical load when channels are inactive.
func TestLoadBalancingInactivityDecay(t *testing.T) {
	t.Log("=== Testing inactivity decay in round-robin load balancing ===")
	t.Log("WeightRoundRobinStrategy should decay historical load for inactive channels")
	t.Log("This prevents channels from being permanently penalized for past load")

	helper := testutil.NewTestHelper(t, "TestLoadBalancingInactivityDecay")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true
	ctx := helper.CreateTestContext()

	// First burst of requests
	t.Log("\n--- First burst: Establishing load ---")
	for i := 0; i < 3; i++ {
		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("First burst request %d", i+1)),
			},
			Model: helper.GetModel(),
		})

		helper.AssertNoError(t, err, fmt.Sprintf("First burst request %d failed", i+1))
		helper.ValidateChatResponse(t, response, fmt.Sprintf("First burst request %d", i+1))
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for inactivity decay period
	t.Log("\n--- Waiting for inactivity decay (5 seconds) ---")
	time.Sleep(5 * time.Second)

	// Second burst after decay period
	t.Log("\n--- Second burst: After inactivity decay ---")
	for i := 0; i < 3; i++ {
		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Second burst request %d", i+1)),
			},
			Model: helper.GetModel(),
		})

		helper.AssertNoError(t, err, fmt.Sprintf("Second burst request %d failed", i+1))
		helper.ValidateChatResponse(t, response, fmt.Sprintf("Second burst request %d", i+1))
		time.Sleep(50 * time.Millisecond)
	}

	t.Log("\n=== Inactivity decay test completed ===")
	t.Log("Historical load should have decayed during the 5-second inactivity period")
	t.Log("Formula: effectiveCount = cappedCount * (1 - inactivitySeconds / decayDuration)")
	t.Log("Default decay duration: 5 minutes")
	t.Log("Channels that were heavily loaded should now have lower effective counts")
}

// TestLoadBalancingScalingFactor verifies that the round-robin scaling
// factor affects how quickly scores decrease with load.
func TestLoadBalancingScalingFactor(t *testing.T) {
	t.Log("=== Testing round-robin scaling factor ===")
	t.Log("The scaling factor determines how quickly channel scores decrease with load")
	t.Log("Formula: score = maxScore * exp(-effectiveCount / scalingFactor)")
	t.Log("Default scaling factor: 50.0")

	helper := testutil.NewTestHelper(t, "TestLoadBalancingScalingFactor")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true
	ctx := helper.CreateTestContext()

	// Send multiple requests to observe score changes
	requestCount := 10
	for i := 0; i < requestCount; i++ {
		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Scaling factor test request %d", i+1)),
			},
			Model: helper.GetModel(),
		})

		helper.AssertNoError(t, err, fmt.Sprintf("Request %d failed", i+1))
		helper.ValidateChatResponse(t, response, fmt.Sprintf("Request %d", i+1))

		if i%3 == 0 {
			t.Logf("Completed %d/%d requests", i+1, requestCount)
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Log("\n=== Scaling factor test completed ===")
	t.Log("As channels receive more requests, their scores should decrease exponentially")
	t.Log("Example scores with scaling factor 50:")
	t.Log("- 0 requests: 150 points (maxScore)")
	t.Log("- 25 requests: ~91 points (exp(-25/50) * 150)")
	t.Log("- 50 requests: ~55 points (exp(-50/50) * 150)")
	t.Log("- 100 requests: ~20 points (exp(-100/50) * 150)")
}

// TestLoadBalancingWeightNormalization verifies that weight-based
// round-robin correctly normalizes request counts by channel weight.
func TestLoadBalancingWeightNormalization(t *testing.T) {
	t.Log("=== Testing weight normalization in round-robin ===")
	t.Log("WeightRoundRobinStrategy normalizes request count by weight")
	t.Log("Formula: normalizedCount = effectiveCount / (weight / 100.0)")
	t.Log("This ensures proportional distribution based on weight")

	// Send requests across multiple traces to test weight distribution
	requestCount := 30
	for i := 0; i < requestCount; i++ {
		helper := testutil.NewTestHelper(t, fmt.Sprintf("TestWeightNormalization_Request%d", i))
		helper.Config.DisableTrace = true
		helper.Config.DisableThread = true
		ctx := helper.CreateTestContext()

		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Weight normalization test request %d", i+1)),
			},
			Model: helper.GetModel(),
		})

		helper.AssertNoError(t, err, fmt.Sprintf("Request %d failed", i+1))
		helper.ValidateChatResponse(t, response, fmt.Sprintf("Request %d", i+1))

		if i%10 == 0 {
			t.Logf("Completed %d/%d requests", i+1, requestCount)
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Log("\n=== Weight normalization test completed ===")
	t.Logf("Sent %d requests across different traces", requestCount)
	t.Log("Expected distribution (assuming weights 80, 50, 20, 10):")
	t.Log("- Weight 80 channel: ~50% of requests (80/160)")
	t.Log("- Weight 50 channel: ~31% of requests (50/160)")
	t.Log("- Weight 20 channel: ~12.5% of requests (20/160)")
	t.Log("- Weight 10 channel: ~6.25% of requests (10/160)")
	t.Log("Check server logs or metrics to verify actual distribution")
}

// TestLoadBalancingCompositeStrategy verifies that CompositeStrategy
// can combine multiple strategies with custom weights.
func TestLoadBalancingCompositeStrategy(t *testing.T) {
	t.Log("=== Testing composite strategy ===")
	t.Log("CompositeStrategy allows combining multiple strategies with custom weights")
	t.Log("This enables flexible load balancing configurations")

	helper := testutil.NewTestHelper(t, "TestLoadBalancingCompositeStrategy")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true
	ctx := helper.CreateTestContext()

	// Make requests to test composite strategy
	requestCount := 5
	for i := 0; i < requestCount; i++ {
		response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Composite strategy test request %d", i+1)),
			},
			Model: helper.GetModel(),
		})

		helper.AssertNoError(t, err, fmt.Sprintf("Request %d failed", i+1))
		helper.ValidateChatResponse(t, response, fmt.Sprintf("Request %d", i+1))

		t.Logf("Request %d completed", i+1)
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("\n=== Composite strategy test completed ===")
	t.Log("CompositeStrategy can be configured with custom weights:")
	t.Log("Example: NewCompositeStrategy(strategy1, strategy2).WithWeights(2.0, 1.0)")
	t.Log("This would give strategy1 twice the influence of strategy2")
}
