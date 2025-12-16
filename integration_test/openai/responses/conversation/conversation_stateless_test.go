package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

// Helper function
func createUserMessage(text string) responses.ResponseInputItemUnionParam {
	return responses.ResponseInputItemParamOfInputMessage(
		responses.ResponseInputMessageContentListParam{
			responses.ResponseInputContentParamOfInputText(text),
		},
		"user",
	)
}

// Helper function to create an assistant message input item from response output
func createAssistantMessage(text string) responses.ResponseInputItemUnionParam {
	return responses.ResponseInputItemParamOfOutputMessage(
		[]responses.ResponseOutputMessageContentUnionParam{
			{
				OfOutputText: &responses.ResponseOutputTextParam{
					Text:        text,
					Annotations: []responses.ResponseOutputTextAnnotationUnionParam{},
				},
			},
		},
		"", // id can be empty for stateless
		responses.ResponseOutputMessageStatusCompleted,
	)
}

func TestResponsesConversationStateless(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestResponsesConversationStateless")

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// First turn: Introduce information
	firstInput := "Hello! I'm planning a trip to Japan. My favorite color is blue."
	t.Logf("Turn 1: %s", firstInput)

	params1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
			},
		},
	}

	resp1, err := helper.CreateResponseWithHeaders(ctx, params1)
	helper.AssertNoError(t, err, "Failed on first turn")

	if resp1 == nil {
		t.Fatal("First response is nil")
	}

	output1 := resp1.OutputText()
	t.Logf("Assistant (turn 1): %s", output1)

	// Second turn: Ask a follow-up question using conversation history in input array
	secondInput := "What is my favorite color?"
	t.Logf("Turn 2 (stateless with history): %s", secondInput)

	params2 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
				createAssistantMessage(output1),
				createUserMessage(secondInput),
			},
		},
	}

	resp2, err := helper.CreateResponseWithHeaders(ctx, params2)
	helper.AssertNoError(t, err, "Failed on second turn")

	if resp2 == nil {
		t.Fatal("Second response is nil")
	}

	output2 := resp2.OutputText()
	t.Logf("Assistant (turn 2): %s", output2)

	// Verify context was preserved (should mention "blue")
	if !testutil.ContainsCaseInsensitive(output2, "blue") {
		t.Errorf("Expected second response to reference 'blue', got: %s", output2)
	}

	// Third turn: Ask about the trip using context from first turn
	thirdInput := "Where am I planning to travel?"
	t.Logf("Turn 3 (stateless with history): %s", thirdInput)

	params3 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
				createAssistantMessage(output1),
				createUserMessage(secondInput),
				createAssistantMessage(output2),
				createUserMessage(thirdInput),
			},
		},
	}

	resp3, err := helper.CreateResponseWithHeaders(ctx, params3)
	helper.AssertNoError(t, err, "Failed on third turn")

	if resp3 == nil {
		t.Fatal("Third response is nil")
	}

	output3 := resp3.OutputText()
	t.Logf("Assistant (turn 3): %s", output3)

	// Verify context chain is maintained (should mention "Japan")
	if !testutil.ContainsCaseInsensitive(output3, "japan") {
		t.Errorf("Expected third response to reference 'Japan', got: %s", output3)
	}

	t.Logf("Stateless conversation completed successfully with 3 turns")
}

func TestResponsesConversationStatelessWithInstructions(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestResponsesConversationStatelessWithInstructions")

	ctx := helper.CreateTestContext()

	// Use instructions to set assistant behavior
	instructions := "You are a helpful space science tutor. Keep explanations simple and encouraging."

	// Turn 1: Introduction
	input1 := "I'm learning about black holes."
	t.Logf("Turn 1 (with instructions): %s", input1)

	params1 := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(helper.GetModel()),
		Instructions: openai.String(instructions),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
			},
		},
	}

	resp1, err := helper.CreateResponseWithHeaders(ctx, params1)
	helper.AssertNoError(t, err, "Failed on first turn with instructions")

	if resp1 == nil {
		t.Fatal("First response is nil")
	}

	output1 := resp1.OutputText()
	t.Logf("Assistant (turn 1): %s", output1)

	// Turn 2: Follow-up question using stateless history
	input2 := "How do they form?"
	t.Logf("Turn 2 (stateless): %s", input2)

	params2 := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(helper.GetModel()),
		Instructions: openai.String(instructions),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(input1),
				createAssistantMessage(output1),
				createUserMessage(input2),
			},
		},
	}

	resp2, err := helper.CreateResponseWithHeaders(ctx, params2)
	helper.AssertNoError(t, err, "Failed on second turn")

	if resp2 == nil {
		t.Fatal("Second response is nil")
	}

	output2 := resp2.OutputText()
	t.Logf("Assistant (turn 2): %s", output2)

	// Verify response is related to black hole formation
	if !testutil.ContainsAnyCaseInsensitive(output2, "black hole", "star", "gravity", "collapse") {
		t.Errorf("Expected response about black hole formation, got: %s", output2)
	}
}

func TestResponsesConversationStatelessContextChain(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestResponsesConversationStatelessContextChain")

	ctx := helper.CreateTestContext()

	// Create a longer conversation chain using stateless approach
	type conversationTurn struct {
		input    string
		validate func(output string) bool
		desc     string
	}

	facts := []conversationTurn{
		{
			input:    "My name is Alice.",
			validate: func(o string) bool { return testutil.ContainsCaseInsensitive(o, "alice") },
			desc:     "name introduction",
		},
		{
			input:    "I live in Paris.",
			validate: func(o string) bool { return testutil.ContainsCaseInsensitive(o, "paris") },
			desc:     "location introduction",
		},
		{
			input:    "What is my name?",
			validate: func(o string) bool { return testutil.ContainsCaseInsensitive(o, "alice") },
			desc:     "recall name",
		},
		{
			input:    "Where do I live?",
			validate: func(o string) bool { return testutil.ContainsCaseInsensitive(o, "paris") },
			desc:     "recall location",
		},
	}

	// Build conversation history incrementally
	var history []responses.ResponseInputItemUnionParam

	for i, fact := range facts {
		t.Logf("Turn %d (%s): %s", i+1, fact.desc, fact.input)

		// Add current user message to history
		currentHistory := append(history, createUserMessage(fact.input))

		params := responses.ResponseNewParams{
			Model: shared.ResponsesModel(helper.GetModel()),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: responses.ResponseInputParam(currentHistory),
			},
		}

		resp, err := helper.CreateResponseWithHeaders(ctx, params)
		helper.AssertNoError(t, err, "Failed on turn", i+1)

		if resp == nil {
			t.Fatalf("Response %d is nil", i+1)
		}

		output := resp.OutputText()
		t.Logf("Assistant (turn %d): %s", i+1, output)

		// Validate response contains expected context
		if fact.validate != nil && !fact.validate(output) {
			t.Errorf("Turn %d (%s) validation failed. Got: %s", i+1, fact.desc, output)
		}

		// Update history with user message and assistant response for next turn
		history = append(history, createUserMessage(fact.input))
		history = append(history, createAssistantMessage(output))
	}

	t.Logf("Stateless context chain test completed with %d turns", len(facts))
}

func TestMultiTurnConversationStateless(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestMultiTurnConversationStateless")

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// First turn: Introduce information
	firstInput := "Hello! I'm planning a trip to Japan. Can you help me?"
	t.Logf("Starting conversation...")
	t.Logf("Turn 1: %s", firstInput)

	params1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
			},
		},
	}

	resp1, err := helper.CreateResponseWithHeaders(ctx, params1)
	helper.AssertNoError(t, err, "Failed to start conversation")

	if resp1 == nil {
		t.Fatal("First response is nil")
	}

	firstResponse := resp1.OutputText()
	t.Logf("Assistant (first): %s", firstResponse)

	// Second turn: Weather question with context
	secondInput := "What's the weather like in Tokyo this time of year?"
	t.Logf("Turn 2: %s", secondInput)

	params2 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
				createAssistantMessage(firstResponse),
				createUserMessage(secondInput),
			},
		},
	}

	resp2, err := helper.CreateResponseWithHeaders(ctx, params2)
	helper.AssertNoError(t, err, "Failed in second conversation turn")

	if resp2 == nil {
		t.Fatal("Second response is nil")
	}

	secondResponse := resp2.OutputText()
	t.Logf("Assistant (second): %s", secondResponse)

	// Verify context was preserved (should reference Japan trip)
	if !testutil.ContainsCaseInsensitive(firstResponse, "japan") && !testutil.ContainsCaseInsensitive(firstResponse, "trip") {
		t.Errorf("Expected first response to acknowledge Japan trip, got: %s", firstResponse)
	}

	// Third turn: Calculation question (should trigger tool usage)
	thirdInput := "Actually, let me ask: what is 365 * 24?"
	t.Logf("Turn 3: %s", thirdInput)

	params3 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
				createAssistantMessage(firstResponse),
				createUserMessage(secondInput),
				createAssistantMessage(secondResponse),
				createUserMessage(thirdInput),
			},
		},
	}

	resp3, err := helper.CreateResponseWithHeaders(ctx, params3)
	helper.AssertNoError(t, err, "Failed in third conversation turn")

	if resp3 == nil {
		t.Fatal("Third response is nil")
	}

	thirdResponse := resp3.OutputText()
	t.Logf("Assistant (third): %s", thirdResponse)

	// Verify calculation
	if !testutil.ContainsAnyCaseInsensitive(thirdResponse, "8760", "8,760") && !testutil.ContainsCaseInsensitive(thirdResponse, "eight thousand") {
		t.Errorf("Expected calculation result 8760, got: %s", thirdResponse)
	}

	t.Logf("Stateless conversation completed successfully with 3 turns")
}

func TestConversationWithToolsStateless(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestConversationWithToolsStateless")

	ctx := helper.CreateTestContext()

	// Start conversation with tool requirements
	firstInput := "What is the weather in Tokyo and calculate 365 * 24 for me?"
	t.Logf("Starting stateless conversation with tools...")
	t.Logf("Turn 1: %s", firstInput)

	// Define tools for Responses API
	calculatorTool := responses.ToolUnionParam{
		OfFunction: &responses.FunctionToolParam{
			Name:        "calculate",
			Description: openai.String("Perform mathematical calculations"),
			Strict:      openai.Bool(true),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"expression": map[string]string{
						"type":        "string",
						"description": "The mathematical expression to evaluate",
					},
				},
				"required":             []string{"expression"},
				"additionalProperties": false,
			},
		},
	}

	weatherTool := responses.ToolUnionParam{
		OfFunction: &responses.FunctionToolParam{
			Name:        "get_current_weather",
			Description: openai.String("Get the current weather for a specified location"),
			Strict:      openai.Bool(true),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]string{
						"type":        "string",
						"description": "The city name to get weather for",
					},
				},
				"required":             []string{"location"},
				"additionalProperties": false,
			},
		},
	}

	tools := []responses.ToolUnionParam{calculatorTool, weatherTool}

	// First turn - should trigger tool calls
	params1 := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
			},
		},
		Tools: tools,
	}

	resp1, err := helper.CreateResponseWithHeaders(ctx, params1)
	helper.AssertNoError(t, err, "Failed in conversation with tools")

	if resp1 == nil {
		t.Fatal("First response is nil")
	}

	// Check for function calls in output
	var functionCalls []responses.ResponseFunctionToolCall
	for _, item := range resp1.Output {
		if item.Type == "function_call" {
			functionCalls = append(functionCalls, item.AsFunctionCall())
		}
	}

	if len(functionCalls) == 0 {
		// No tool calls, just log and continue
		t.Logf("No tool calls in first turn, response: %s", resp1.OutputText())
	} else {
		t.Logf("Tool calls detected: %d", len(functionCalls))

		// Build stateless history with tool calls and results
		var history []responses.ResponseInputItemUnionParam
		history = append(history, createUserMessage(firstInput))

		// Add function calls and their results to history
		for _, fc := range functionCalls {
			var args map[string]interface{}
			err = json.Unmarshal([]byte(fc.Arguments), &args)
			helper.AssertNoError(t, err, "Failed to parse tool arguments")

			var result string
			switch fc.Name {
			case "calculate":
				calcResult := simulateCalculatorFunctionStateless(args)
				result = fmt.Sprintf("%v", calcResult)
			case "get_current_weather":
				result = simulateWeatherFunctionStateless(args)
			default:
				result = "Unknown function"
			}

			t.Logf("Tool %s (call_id=%s) result: %s", fc.Name, fc.CallID, result)

			// Add function call to history
			history = append(history, responses.ResponseInputItemParamOfFunctionCall(fc.Arguments, fc.CallID, fc.Name))
			// Add function call output to history
			history = append(history, responses.ResponseInputItemParamOfFunctionCallOutput(fc.CallID, result))
		}

		// Second turn with tool results using stateless history
		params2 := responses.ResponseNewParams{
			Model: shared.ResponsesModel(helper.GetModel()),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: responses.ResponseInputParam(history),
			},
			Tools: tools,
		}

		resp2, err := helper.CreateResponseWithHeaders(ctx, params2)
		helper.AssertNoError(t, err, "Failed in tool conversation second turn")

		if resp2 == nil {
			t.Fatal("Second response is nil")
		}

		output2 := resp2.OutputText()
		t.Logf("Response after tool results: %s", output2)

		// Verify response incorporates tool results
		if len(output2) == 0 {
			t.Error("Expected non-empty response after tool results")
		}

		// Third turn - ask follow-up question using stateless history
		thirdInput := "Based on that weather, should I pack an umbrella?"
		t.Logf("Turn 3: %s", thirdInput)

		history = append(history, createAssistantMessage(output2))
		history = append(history, createUserMessage(thirdInput))

		params3 := responses.ResponseNewParams{
			Model: shared.ResponsesModel(helper.GetModel()),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: responses.ResponseInputParam(history),
			},
			Tools: tools,
		}

		resp3, err := helper.CreateResponseWithHeaders(ctx, params3)
		helper.AssertNoError(t, err, "Failed in tool conversation third turn")

		if resp3 == nil {
			t.Fatal("Third response is nil")
		}

		output3 := resp3.OutputText()
		t.Logf("Umbrella response: %s", output3)
	}

	t.Logf("Stateless tool conversation completed successfully")
}

// Helper functions for tool simulation

func simulateCalculatorFunctionStateless(args map[string]interface{}) float64 {
	expression, _ := args["expression"].(string)

	switch expression {
	case "365 * 24":
		return 8760
	case "100 / 4":
		return 25
	case "50 * 30":
		return 1500
	case "15 * 7 + 23":
		return 128
	default:
		return 42
	}
}

func simulateWeatherFunctionStateless(args map[string]interface{}) string {
	location, _ := args["location"].(string)

	// Mock weather data
	weatherData := map[string]map[string]string{
		"tokyo":    {"temp": "25", "condition": "Sunny", "humidity": "60%"},
		"london":   {"temp": "18", "condition": "Rainy", "humidity": "80%"},
		"new york": {"temp": "22", "condition": "Partly cloudy", "humidity": "65%"},
	}

	defaultWeather := map[string]string{"temp": "20", "condition": "Sunny", "humidity": "50%"}

	weather := defaultWeather
	if cityWeather, exists := weatherData[strings.ToLower(location)]; exists {
		weather = cityWeather
	}

	return fmt.Sprintf("Current weather in %s: %s°C, %s, humidity %s",
		location, weather["temp"], weather["condition"], weather["humidity"])
}

func TestConversationContextPreservationStateless(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestConversationContextPreservationStateless")

	ctx := helper.CreateTestContext()

	// Test context preservation across multiple turns
	systemPrompt := "You are a helpful assistant knowledgeable about space and astronomy."

	// Turn 1: Greeting and topic introduction
	firstInput := "Hi, I'm working on a science project about space."
	t.Logf("Turn 1: %s", firstInput)

	params1 := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(helper.GetModel()),
		Instructions: openai.String(systemPrompt),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
			},
		},
	}

	resp1, err := helper.CreateResponseWithHeaders(ctx, params1)
	helper.AssertNoError(t, err, "Failed in context preservation turn 1")

	if resp1 == nil {
		t.Fatal("First response is nil")
	}

	response1 := resp1.OutputText()
	t.Logf("Turn 1: %s", response1)

	// Verify topic understanding
	if !testutil.ContainsCaseInsensitive(response1, "space") && !testutil.ContainsCaseInsensitive(response1, "science") {
		t.Errorf("Expected response to acknowledge space/science topic, got: %s", response1)
	}

	// Turn 2: Follow-up question (context should be preserved)
	secondInput := "What about black holes? Are they really holes?"
	t.Logf("Turn 2: %s", secondInput)

	params2 := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(helper.GetModel()),
		Instructions: openai.String(systemPrompt),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
				createAssistantMessage(response1),
				createUserMessage(secondInput),
			},
		},
	}

	resp2, err := helper.CreateResponseWithHeaders(ctx, params2)
	helper.AssertNoError(t, err, "Failed in context preservation turn 2")

	if resp2 == nil {
		t.Fatal("Second response is nil")
	}

	response2 := resp2.OutputText()
	t.Logf("Turn 2: %s", response2)

	// Turn 3: Another follow-up (should maintain context)
	thirdInput := "How do they form?"
	t.Logf("Turn 3: %s", thirdInput)

	params3 := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(helper.GetModel()),
		Instructions: openai.String(systemPrompt),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
				createAssistantMessage(response1),
				createUserMessage(secondInput),
				createAssistantMessage(response2),
				createUserMessage(thirdInput),
			},
		},
	}

	resp3, err := helper.CreateResponseWithHeaders(ctx, params3)
	helper.AssertNoError(t, err, "Failed in context preservation turn 3")

	if resp3 == nil {
		t.Fatal("Third response is nil")
	}

	response3 := resp3.OutputText()
	t.Logf("Turn 3: %s", response3)

	// Verify all responses are related to space/astronomy
	topics := []string{"space", "black hole", "form", "star", "gravity"}
	contextScore := 0
	for _, response := range []string{response1, response2, response3} {
		for _, topic := range topics {
			if testutil.ContainsCaseInsensitive(response, topic) {
				contextScore++
				break
			}
		}
	}

	if contextScore < 2 {
		t.Errorf("Expected responses to maintain space/astronomy context, got context score: %d/3", contextScore)
	}

	t.Logf("Stateless context preservation test with tools completed with conversation of 3 turns")
}

func TestConversationSystemPromptStateless(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestConversationSystemPromptStateless")

	ctx := helper.CreateTestContext()

	// Test system prompt influence on conversation
	systemPrompt := "You are a helpful cooking assistant. Provide recipes and cooking tips in a friendly, encouraging way."

	// Turn 1: Pasta suggestion
	firstInput := "I want to make pasta tonight. Any suggestions?"
	t.Logf("Turn 1 (with cooking system prompt): %s", firstInput)

	params1 := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(helper.GetModel()),
		Instructions: openai.String(systemPrompt),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
			},
		},
	}

	resp1, err := helper.CreateResponseWithHeaders(ctx, params1)
	helper.AssertNoError(t, err, "Failed with system prompt")

	if resp1 == nil {
		t.Fatal("First response is nil")
	}

	response := resp1.OutputText()
	t.Logf("Response with cooking system prompt: %s", response)

	// Verify cooking context
	cookingTerms := []string{"pasta", "recipe", "cook", "ingredient", "boil"}
	foundTerms := 0
	for _, term := range cookingTerms {
		if testutil.ContainsCaseInsensitive(response, term) {
			foundTerms++
		}
	}

	if foundTerms < 2 {
		t.Errorf("Expected response to contain cooking terms, found %d/%d", foundTerms, len(cookingTerms))
	}

	// Continue conversation with different topic
	secondInput := "Actually, what about making pizza instead?"
	t.Logf("Turn 2: %s", secondInput)

	params2 := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(helper.GetModel()),
		Instructions: openai.String(systemPrompt),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				createUserMessage(firstInput),
				createAssistantMessage(response),
				createUserMessage(secondInput),
			},
		},
	}

	resp2, err := helper.CreateResponseWithHeaders(ctx, params2)
	helper.AssertNoError(t, err, "Failed in cooking conversation continuation")

	if resp2 == nil {
		t.Fatal("Second response is nil")
	}

	response2 := resp2.OutputText()
	t.Logf("Pizza response: %s", response2)

	// Verify continued cooking context
	if !testutil.ContainsCaseInsensitive(response2, "pizza") && !testutil.ContainsCaseInsensitive(response2, "dough") {
		t.Errorf("Expected pizza response to contain pizza-related terms, got: %s", response2)
	}

	t.Logf("Stateless system prompt test with tools completed successfully")
}
