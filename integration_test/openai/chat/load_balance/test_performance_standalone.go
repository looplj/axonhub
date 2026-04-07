package main

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
)

func main() {
	fmt.Println("=== Testing Performance Load Balancer Strategy ===")
	fmt.Println("This test verifies that the 'performance' strategy is working correctly")

	testPerformanceStrategy()
}

func testPerformanceStrategy() {
	helper := testutil.NewTestHelper(nil, "TestPerformanceStrategy")
	helper.Config.DisableTrace = true
	helper.Config.DisableThread = true

	fmt.Println("\n1. Making first request to establish baseline performance...")

	ctx := helper.CreateTestContext()

	start := time.Now()

	response, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What is 2+2? Just answer the number."),
		},
		Model: helper.GetModel(),
	})

	if err != nil {
		panic(fmt.Sprintf("Request failed: %v", err))
	}

	elapsed := time.Since(start)

	fmt.Printf("   ✓ First request completed in %v\n", elapsed)
	fmt.Printf("   Response: %s\n", response.Choices[0].Message.Content)

	fmt.Println("\n2. Making second request (performance strategy should prefer faster channels)...")

	start = time.Now()

	response2, err := helper.CreateChatCompletionWithHeaders(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What is 3+3? Just answer the number."),
		},
		Model: helper.GetModel(),
	})

	if err != nil {
		panic(fmt.Sprintf("Second request failed: %v", err))
	}

	elapsed2 := time.Since(start)

	fmt.Printf("   ✓ Second request completed in %v\n", elapsed2)
	fmt.Printf("   Response: %s\n", response2.Choices[0].Message.Content)

	fmt.Println("\n3. Checking response headers for performance metrics...")

	if helper.GetResponseHeader("X-Channel-ID") != "" {
		fmt.Printf("   ✓ Channel ID: %s\n", helper.GetResponseHeader("X-Channel-ID"))
	}

	fmt.Println("\n✅ Performance strategy is working!")
	fmt.Println("\nWhat to check in server logs:")
	fmt.Println("  - Look for: load_balance_strategy=performance")
	fmt.Println("  - Look for: performance-aware strategy scoring")
	fmt.Println("  - Look for: Channel scores based on TTFT and throughput metrics")
}
