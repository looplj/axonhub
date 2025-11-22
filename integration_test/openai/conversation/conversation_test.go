package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func TestMain(m *testing.M) {
	// Set up any global test configuration here if needed
	code := m.Run()
	os.Exit(code)
}

func TestMultiTurnConversation(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Start a conversation
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello! I'm planning a trip to Japan. Can you help me?"),
	}

	t.Logf("Starting conversation...")

	// First turn
	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    helper.GetModel(),
	}

	completion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to start conversation")

	helper.ValidateChatResponse(t, completion, "First conversation turn")

	firstResponse := completion.Choices[0].Message.Content
	t.Logf("Assistant (first): %s", firstResponse)

	// Continue conversation with context
	messages = append(messages, completion.Choices[0].Message.ToParam())
	messages = append(messages, openai.UserMessage("What's the weather like in Tokyo this time of year?"))

	// Second turn
	params.Messages = messages
	completion2, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in second conversation turn")

	helper.ValidateChatResponse(t, completion2, "Second conversation turn")

	secondResponse := completion2.Choices[0].Message.Content
	t.Logf("Assistant (second): %s", secondResponse)

	// Verify context was preserved (should reference Japan trip)
	if !testutil.ContainsCaseInsensitive(firstResponse, "japan") && !testutil.ContainsCaseInsensitive(firstResponse, "trip") {
		t.Errorf("Expected first response to acknowledge Japan trip, got: %s", firstResponse)
	}

	// Third turn with tool usage
	messages = append(messages, completion2.Choices[0].Message.ToParam())
	messages = append(messages, openai.UserMessage("Actually, let me ask: what is 365 * 24?"))

	params.Messages = messages
	completion3, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in third conversation turn")

	helper.ValidateChatResponse(t, completion3, "Third conversation turn")

	thirdResponse := completion3.Choices[0].Message.Content
	t.Logf("Assistant (third): %s", thirdResponse)

	// Verify calculation
	if !testutil.ContainsAnyCaseInsensitive(thirdResponse, "8760", "8,760") && !testutil.ContainsCaseInsensitive(thirdResponse, "eight thousand") {
		t.Errorf("Expected calculation result 8760, got: %s", thirdResponse)
	}

	t.Logf("Conversation completed successfully with %d turns", len(messages)/2+1)
}

func TestConversationWithTools(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Start conversation with tool requirements
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("I need help with some calculations and weather information for my trip planning."),
	}

	t.Logf("Starting conversation with tools...")

	// Define tools
	calculatorFunction := shared.FunctionDefinitionParam{
		Name:        "calculate",
		Description: openai.String("Perform mathematical calculations"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]string{
					"type": "string",
				},
			},
			"required": []string{"expression"},
		},
	}

	weatherFunction := shared.FunctionDefinitionParam{
		Name:        "get_current_weather",
		Description: openai.String("Get the current weather for a specified location"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]string{
					"type": "string",
				},
			},
			"required": []string{"location"},
		},
	}

	calculatorTool := openai.ChatCompletionFunctionTool(calculatorFunction)
	weatherTool := openai.ChatCompletionFunctionTool(weatherFunction)

	// First turn - should trigger tool calls
	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Tools:    []openai.ChatCompletionToolUnionParam{calculatorTool, weatherTool},
		Model:    helper.GetModel(),
	}

	completion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in conversation with tools")

	helper.ValidateChatResponse(t, completion, "Tool conversation first turn")

	// Check for tool calls
	if len(completion.Choices) == 0 || completion.Choices[0].Message.ToolCalls == nil {
		// If no tool calls, continue normally
		t.Logf("No tool calls in first turn, continuing conversation")
	} else {
		t.Logf("Tool calls detected: %d", len(completion.Choices[0].Message.ToolCalls))

		// Process tool calls
		var toolResults []string
		for _, toolCall := range completion.Choices[0].Message.ToolCalls {
			var args map[string]interface{}
			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			helper.AssertNoError(t, err, "Failed to parse tool arguments")

			var result string
			switch toolCall.Function.Name {
			case "calculate":
				calcResult := simulateCalculatorFunction(args)
				result = fmt.Sprintf("%v", calcResult)
			case "get_current_weather":
				result = simulateWeatherFunction(args)
			default:
				result = "Unknown function"
			}

			toolResults = append(toolResults, result)
			t.Logf("Tool %s result: %s", toolCall.Function.Name, result)
		}

		// Continue conversation with tool results
		messages = append(messages, completion.Choices[0].Message.ToParam())
		for i, toolCall := range completion.Choices[0].Message.ToolCalls {
			messages = append(messages, openai.ToolMessage(toolResults[i], toolCall.ID))
		}

		// Add follow-up question
		messages = append(messages, openai.UserMessage("Based on that information, should I pack an umbrella?"))

		// Second turn with tool results
		params.Messages = messages
		completion2, err := helper.Client.Chat.Completions.New(ctx, params)
		helper.AssertNoError(t, err, "Failed in tool conversation second turn")

		helper.ValidateChatResponse(t, completion2, "Tool conversation second turn")

		secondResponse := completion2.Choices[0].Message.Content
		t.Logf("Final response: %s", secondResponse)

		// Verify response incorporates tool results
		if len(secondResponse) == 0 {
			t.Error("Expected non-empty final response")
		}
	}
}

func TestConversationContextPreservation(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Test context preservation across multiple turns
	var conversation []openai.ChatCompletionMessageParamUnion

	// Turn 1: Greeting and topic introduction
	conversation = append(conversation, openai.UserMessage("Hi, I'm working on a science project about space."))
	conversation = append(conversation, openai.SystemMessage("You are a helpful assistant knowledgeable about space and astronomy."))

	params := openai.ChatCompletionNewParams{
		Messages: conversation,
		Model:    helper.GetModel(),
	}

	completion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in context preservation turn 1")

	helper.ValidateChatResponse(t, completion, "Context preservation turn 1")

	response1 := completion.Choices[0].Message.Content
	t.Logf("Turn 1: %s", response1)

	// Verify topic understanding
	if !testutil.ContainsCaseInsensitive(response1, "space") && !testutil.ContainsCaseInsensitive(response1, "science") {
		t.Errorf("Expected response to acknowledge space/science topic, got: %s", response1)
	}

	// Turn 2: Follow-up question (context should be preserved)
	conversation = append(conversation, completion.Choices[0].Message.ToParam())
	conversation = append(conversation, openai.UserMessage("What about black holes? Are they really holes?"))

	params.Messages = conversation
	completion2, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in context preservation turn 2")

	helper.ValidateChatResponse(t, completion2, "Context preservation turn 2")

	response2 := completion2.Choices[0].Message.Content
	t.Logf("Turn 2: %s", response2)

	// Turn 3: Another follow-up (should maintain context)
	conversation = append(conversation, completion2.Choices[0].Message.ToParam())
	conversation = append(conversation, openai.UserMessage("How do they form?"))

	params.Messages = conversation
	completion3, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in context preservation turn 3")

	helper.ValidateChatResponse(t, completion3, "Context preservation turn 3")

	response3 := completion3.Choices[0].Message.Content
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

	t.Logf("Context preservation test completed with conversation of %d messages", len(conversation))
}

func TestConversationSystemPrompt(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Test system prompt influence on conversation
	systemPrompt := "You are a helpful cooking assistant. Provide recipes and cooking tips in a friendly, encouraging way."

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage("I want to make pasta tonight. Any suggestions?"),
	}

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    helper.GetModel(),
	}

	completion, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed with system prompt")

	helper.ValidateChatResponse(t, completion, "System prompt test")

	response := completion.Choices[0].Message.Content
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
	messages = append(messages, completion.Choices[0].Message.ToParam())
	messages = append(messages, openai.UserMessage("Actually, what about making pizza instead?"))

	params.Messages = messages
	completion2, err := helper.Client.Chat.Completions.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in cooking conversation continuation")

	helper.ValidateChatResponse(t, completion2, "Cooking conversation continuation")

	response2 := completion2.Choices[0].Message.Content
	t.Logf("Pizza response: %s", response2)

	// Verify continued cooking context
	if !testutil.ContainsCaseInsensitive(response2, "pizza") && !testutil.ContainsCaseInsensitive(response2, "dough") {
		t.Errorf("Expected pizza response to contain pizza-related terms, got: %s", response2)
	}
}

// Helper functions (same as in other test files)

func simulateCalculatorFunction(args map[string]interface{}) float64 {
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

func simulateWeatherFunction(args map[string]interface{}) string {
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
