package main

import (
	"fmt"
	"testing"

	"github.com/looplj/axonhub/gemini_test/internal/testutil"
	"google.golang.org/genai"
)

func TestSingleThreadMultipleTraces(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "thread_multiple_traces")

	// Print headers for debugging
	helper.PrintHeaders(t)

	// Get current thread ID for this test
	currentThreadID := helper.Config.ThreadID
	t.Logf("Using thread ID: %s", currentThreadID)

	t.Logf("Starting single thread with multiple traces...")

	// Trace 1: Project planning
	t.Logf("=== Starting Trace 1: Project Planning ===")
	ctx1 := helper.CreateTestContext()
	modelName := helper.GetModel()

	// Create chat for trace 1
	var config1 *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat1, err := helper.Client.Chats.Create(ctx1, modelName, config1, nil)
	helper.AssertNoError(t, err, "Failed to create chat for trace 1")

	question1 := "I need to plan a software development project. What are the key phases?"
	t.Logf("Trace 1 - Question 1: %s", question1)

	response1, err := chat1.SendMessage(ctx1, genai.Part{Text: question1})
	helper.AssertNoError(t, err, "Failed in trace 1, call 1")

	helper.ValidateChatResponse(t, response1, "Trace 1, call 1")

	response1Text := testutil.ExtractTextFromResponse(response1)
	t.Logf("Trace 1 - Assistant: %s", response1Text)

	// Continue trace 1 with more calls
	question2 := "What tools and technologies should I consider for each phase?"
	t.Logf("Trace 1 - Question 2: %s", question2)

	response2, err := chat1.SendMessage(ctx1, genai.Part{Text: question2})
	helper.AssertNoError(t, err, "Failed in trace 1, call 2")

	helper.ValidateChatResponse(t, response2, "Trace 1, call 2")

	response2Text := testutil.ExtractTextFromResponse(response2)
	t.Logf("Trace 1 - Assistant: %s", response2Text)

	// Trace 2: Different topic but same thread
	t.Logf("=== Starting Trace 2: Team Management ===")

	// Create new trace but same thread using helper function
	helper2 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)
	ctx2 := helper2.CreateTestContext()

	// Create chat for trace 2
	var config2 *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat2, err := helper2.Client.Chats.Create(ctx2, modelName, config2, nil)
	helper.AssertNoError(t, err, "Failed to create chat for trace 2")

	question3 := "For the development team, how should I structure the team roles?"
	t.Logf("Trace 2 - Question 1: %s", question3)

	response3, err := chat2.SendMessage(ctx2, genai.Part{Text: question3})
	helper.AssertNoError(t, err, "Failed in trace 2, call 1")

	helper.ValidateChatResponse(t, response3, "Trace 2, call 1")

	response3Text := testutil.ExtractTextFromResponse(response3)
	t.Logf("Trace 2 - Assistant: %s", response3Text)

	// Continue trace 2
	question4 := "What about the project timeline and milestones?"
	t.Logf("Trace 2 - Question 2: %s", question4)

	response4, err := chat2.SendMessage(ctx2, genai.Part{Text: question4})
	helper.AssertNoError(t, err, "Failed in trace 2, call 2")

	helper.ValidateChatResponse(t, response4, "Trace 2, call 2")

	response4Text := testutil.ExtractTextFromResponse(response4)
	t.Logf("Trace 2 - Assistant: %s", response4Text)

	// Trace 3: Tool usage in same thread
	t.Logf("=== Starting Trace 3: Resource Planning ===")

	helper3 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)
	ctx3 := helper3.CreateTestContext()

	// Define calculator tool for cost estimation
	costCalculatorTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "calculate_cost",
				Description: "Calculate project costs based on team size and duration",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"team_size": {
							Type: genai.TypeNumber,
						},
						"duration_months": {
							Type: genai.TypeNumber,
						},
						"hourly_rate": {
							Type: genai.TypeNumber,
						},
					},
					Required: []string{"team_size", "duration_months", "hourly_rate"},
				},
			},
		},
	}

	tools := []*genai.Tool{costCalculatorTool}

	// Create chat for trace 3 with tools
	var config3 *genai.GenerateContentConfig = &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
		Tools:       tools,
	}
	chat3, err := helper3.Client.Chats.Create(ctx3, modelName, config3, nil)
	helper.AssertNoError(t, err, "Failed to create chat for trace 3 with tools")

	question5 := "I need to estimate the project costs. What's a typical budget breakdown?"
	t.Logf("Trace 3 - Question 1: %s", question5)

	response5, err := chat3.SendMessage(ctx3, genai.Part{Text: question5})
	helper.AssertNoError(t, err, "Failed in trace 3, call 1 with tools")

	helper.ValidateChatResponse(t, response5, "Trace 3, call 1 with tools")

	// Check for function calls
	if len(response5.Candidates) > 0 {
		candidate := response5.Candidates[0]
		if candidate.Content != nil {
			var functionCall *genai.Part
			for _, part := range candidate.Content.Parts {
				if part != nil && part.FunctionCall != nil && part.FunctionCall.Name == "calculate_cost" {
					functionCall = part
					break
				}
			}

			if functionCall != nil {
				t.Logf("Function call detected in trace 3: %s", functionCall.FunctionCall.Name)

				// Simulate cost calculation: team_size * duration_months * hourly_rate * 160 (hours per month)
				teamSize := functionCall.FunctionCall.Args["team_size"].(float64)
				duration := functionCall.FunctionCall.Args["duration_months"].(float64)
				hourlyRate := functionCall.FunctionCall.Args["hourly_rate"].(float64)
				totalCost := teamSize * duration * hourlyRate * 160
				result := fmt.Sprintf("Total estimated cost: $%.2f", totalCost)

				t.Logf("Tool %s result: %s", functionCall.FunctionCall.Name, result)

				// Send function response
				functionResponse := genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						ID:   functionCall.FunctionCall.ID,
						Name: functionCall.FunctionCall.Name,
						Response: map[string]interface{}{
							"result": result,
						},
					},
				}

				response5b, err := chat3.SendMessage(ctx3, functionResponse)
				helper.AssertNoError(t, err, "Failed to send function response in trace 3")

				helper.ValidateChatResponse(t, response5b, "Trace 3 function response")

				response5bText := testutil.ExtractTextFromResponse(response5b)
				t.Logf("Trace 3 - Function Response: %s", response5bText)

				// Add follow-up question
				question6 := "Based on this estimate, how should I adjust the project scope?"
				t.Logf("Trace 3 - Question 2: %s", question6)

				response6, err := chat3.SendMessage(ctx3, genai.Part{Text: question6})
				helper.AssertNoError(t, err, "Failed in trace 3, call 2")

				helper.ValidateChatResponse(t, response6, "Trace 3, call 2")

				response6Text := testutil.ExtractTextFromResponse(response6)
				t.Logf("Trace 3 - Assistant: %s", response6Text)

				t.Logf("Single thread test completed successfully with 3 traces, 6 total AI calls, and tool usage")
			} else {
				// If no function calls, continue with text conversation
				response5Text := testutil.ExtractTextFromResponse(response5)
				t.Logf("Trace 3 - Assistant: %s", response5Text)

				// Add follow-up question
				question6 := "Based on this information, what are your recommendations?"
				t.Logf("Trace 3 - Question 2: %s", question6)

				response6, err := chat3.SendMessage(ctx3, genai.Part{Text: question6})
				helper.AssertNoError(t, err, "Failed in trace 3, call 2")

				helper.ValidateChatResponse(t, response6, "Trace 3, call 2")

				response6Text := testutil.ExtractTextFromResponse(response6)
				t.Logf("Trace 3 - Assistant: %s", response6Text)

				t.Logf("Single thread test completed successfully with 3 traces and 6 total AI calls (text only)")
			}
		}
	}

	// Verify all traces used the same thread ID
	if helper.Config.ThreadID != helper2.Config.ThreadID || helper2.Config.ThreadID != helper3.Config.ThreadID {
		t.Errorf("Expected all traces to use the same thread ID %s, but got: %s, %s, %s",
			currentThreadID, helper.Config.ThreadID, helper2.Config.ThreadID, helper3.Config.ThreadID)
	}

	// Verify all traces used different trace IDs
	if helper.Config.TraceID == helper2.Config.TraceID || helper2.Config.TraceID == helper3.Config.TraceID || helper.Config.TraceID == helper3.Config.TraceID {
		t.Errorf("Expected all traces to use different trace IDs, but got duplicates: %s, %s, %s",
			helper.Config.TraceID, helper2.Config.TraceID, helper3.Config.TraceID)
	}

	t.Logf("Thread ID consistency verified: %s", currentThreadID)
	t.Logf("Trace ID uniqueness verified: %s, %s, %s",
		helper.Config.TraceID, helper2.Config.TraceID, helper3.Config.TraceID)
}

func TestMultipleThreadsParallelTraces(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "thread_multiple_traces")

	modelName := helper.GetModel()

	t.Logf("Testing multiple threads with parallel traces")

	// Create multiple threads with different scenarios
	threads := []struct {
		name      string
		topic     string
		questions []string
	}{
		{
			name:  "Technical Discussion",
			topic: "Discuss the pros and cons of microservices architecture",
			questions: []string{
				"What are the main benefits of microservices?",
				"What are the potential challenges?",
			},
		},
		{
			name:  "Design Planning",
			topic: "Plan a user authentication system",
			questions: []string{
				"What security measures should I implement?",
				"Which authentication methods are most secure?",
			},
		},
		{
			name:  "Performance Optimization",
			topic: "Optimize database performance",
			questions: []string{
				"What are the key indexing strategies?",
				"How can I reduce query execution time?",
			},
		},
	}

	// Run threads in parallel (conceptually - in real test this would be goroutines)
	for i, thread := range threads {
		t.Run(fmt.Sprintf("Thread_%d_%s", i+1, thread.name), func(t *testing.T) {
			// Create new helper for each thread (different thread ID)
			threadHelper := testutil.CreateTestHelperWithNewTrace(t, helper.Config)
			threadCtx := threadHelper.CreateTestContext()

			t.Logf("Starting thread %d: %s", i+1, thread.name)

			// Create chat for this thread
			var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
			chat, err := threadHelper.Client.Chats.Create(threadCtx, modelName, config, nil)
			helper.AssertNoError(t, err, fmt.Sprintf("Failed to create chat for thread %d", i+1))

			// Send initial topic
			response, err := chat.SendMessage(threadCtx, genai.Part{Text: thread.topic})
			helper.AssertNoError(t, err, fmt.Sprintf("Failed in thread %d initial call", i+1))

			helper.ValidateChatResponse(t, response, fmt.Sprintf("Thread %d initial", i+1))

			initialResponse := testutil.ExtractTextFromResponse(response)
			t.Logf("Thread %d - Initial: %s", i+1, initialResponse[:min(100, len(initialResponse))])

			// Send follow-up questions
			for j, question := range thread.questions {
				response, err := chat.SendMessage(threadCtx, genai.Part{Text: question})
				helper.AssertNoError(t, err, fmt.Sprintf("Failed in thread %d question %d", i+1, j+1))

				helper.ValidateChatResponse(t, response, fmt.Sprintf("Thread %d question %d", i+1, j+1))

				questionResponse := testutil.ExtractTextFromResponse(response)
				t.Logf("Thread %d - Q%d: %s", i+1, j+1, questionResponse[:min(100, len(questionResponse))])
			}

			t.Logf("Thread %d completed successfully", i+1)
		})
	}

	t.Logf("Multiple threads parallel traces test completed")
}

func TestThreadContextIsolation(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "thread_multiple_traces")

	modelName := helper.GetModel()

	t.Logf("Testing thread context isolation")

	// Thread 1: Discuss cooking
	helper1 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)
	ctx1 := helper1.CreateTestContext()

	var config1 *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat1, err := helper1.Client.Chats.Create(ctx1, modelName, config1, nil)
	helper.AssertNoError(t, err, "Failed to create chat for thread 1")

	response1, err := chat1.SendMessage(ctx1, genai.Part{Text: "I'm learning to cook Italian food. What should I start with?"})
	helper.AssertNoError(t, err, "Failed in thread 1")

	response1Text := testutil.ExtractTextFromResponse(response1)
	t.Logf("Thread 1 (Cooking): %s", response1Text[:min(100, len(response1Text))])

	// Thread 2: Discuss programming (should not have cooking context)
	helper2 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)
	ctx2 := helper2.CreateTestContext()

	var config2 *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat2, err := helper2.Client.Chats.Create(ctx2, modelName, config2, nil)
	helper.AssertNoError(t, err, "Failed to create chat for thread 2")

	response2, err := chat2.SendMessage(ctx2, genai.Part{Text: "What's the best way to learn Python?"})
	helper.AssertNoError(t, err, "Failed in thread 2")

	response2Text := testutil.ExtractTextFromResponse(response2)
	t.Logf("Thread 2 (Programming): %s", response2Text[:min(100, len(response2Text))])

	// Verify context isolation - thread 2 should not mention cooking
	if testutil.ContainsCaseInsensitive(response2Text, "cooking") || testutil.ContainsCaseInsensitive(response2Text, "italian") || testutil.ContainsCaseInsensitive(response2Text, "food") {
		t.Errorf("Thread 2 should not have context from Thread 1, but got: %s", response2Text)
	}

	// Continue thread 1 - should still remember cooking context
	response1b, err := chat1.SendMessage(ctx1, genai.Part{Text: "What about pasta dishes?"})
	helper.AssertNoError(t, err, "Failed in thread 1 continuation")

	response1bText := testutil.ExtractTextFromResponse(response1b)
	t.Logf("Thread 1 continuation: %s", response1bText[:min(100, len(response1bText))])

	// Verify thread 1 maintained context
	if !testutil.ContainsCaseInsensitive(response1bText, "pasta") && !testutil.ContainsCaseInsensitive(response1bText, "italian") {
		t.Errorf("Thread 1 should maintain cooking context, but got: %s", response1bText)
	}

	// Verify thread IDs are different
	if helper1.Config.ThreadID == helper2.Config.ThreadID {
		t.Errorf("Expected different thread IDs, but both were: %s", helper1.Config.ThreadID)
	}

	t.Logf("Thread context isolation test completed successfully")
}

func TestThreadWithStreaming(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "thread_multiple_traces")

	modelName := helper.GetModel()

	t.Logf("Testing thread with streaming")

	// Create thread with streaming
	helper1 := testutil.CreateTestHelperWithNewTrace(t, helper.Config)
	ctx1 := helper1.CreateTestContext()

	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat, err := helper1.Client.Chats.Create(ctx1, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat for streaming thread")

	// Streaming call 1
	question1 := "Write a short poem about nature"
	t.Logf("Streaming call 1: %s", question1)

	stream1 := chat.SendMessageStream(ctx1, genai.Part{Text: question1})

	var response1Text string
	var chunkCount1 int

	for response, err := range stream1 {
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			helper.AssertNoError(t, err, "Stream 1 encountered error")
		}

		chunkCount1++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						response1Text += part.Text
					}
				}
			}
		}
	}

	t.Logf("Thread streaming 1: %d chunks", chunkCount1)

	// Streaming call 2 (context should be preserved)
	question2 := "Now write about the ocean"
	t.Logf("Streaming call 2: %s", question2)

	stream2 := chat.SendMessageStream(ctx1, genai.Part{Text: question2})

	var response2Text string
	var chunkCount2 int

	for response, err := range stream2 {
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			helper.AssertNoError(t, err, "Stream 2 encountered error")
		}

		chunkCount2++
		if response != nil && len(response.Candidates) > 0 {
			candidate := response.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part != nil && part.Text != "" {
						response2Text += part.Text
					}
				}
			}
		}
	}

	t.Logf("Thread streaming 2: %d chunks", chunkCount2)

	// Validate responses
	if len(response1Text) == 0 || len(response2Text) == 0 {
		t.Error("Expected non-empty responses from streaming thread")
	}

	// Check context preservation
	if testutil.ContainsCaseInsensitive(response2Text, "poem") || testutil.ContainsCaseInsensitive(response2Text, "nature") {
		t.Logf("Context preserved in streaming thread")
	}

	t.Logf("Thread streaming test completed successfully")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
