package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/looplj/axonhub/anthropic_test/internal/testutil"
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
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hello! I'm planning a trip to Japan. Can you help me?")),
	}

	t.Logf("Starting conversation...")

	// First turn
	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 1024,
	}

	response, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed to start conversation")

	helper.ValidateMessageResponse(t, response, "First conversation turn")

	firstResponse := ""
	for _, block := range response.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			firstResponse += textBlock.Text
		}
	}
	t.Logf("Assistant (first): %s", firstResponse)

	// Continue conversation with context
	messages = append(messages, response.ToParam())
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather like in Tokyo this time of year?")))

	// Second turn
	params.Messages = messages
	response2, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in second conversation turn")

	helper.ValidateMessageResponse(t, response2, "Second conversation turn")

	secondResponse := ""
	for _, block := range response2.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			secondResponse += textBlock.Text
		}
	}
	t.Logf("Assistant (second): %s", secondResponse)

	// Verify context was preserved (should reference Japan trip)
	if !testutil.ContainsCaseInsensitive(firstResponse, "japan") && !testutil.ContainsCaseInsensitive(firstResponse, "trip") {
		t.Errorf("Expected first response to acknowledge Japan trip, got: %s", firstResponse)
	}

	// Third turn with calculation
	messages = append(messages, response2.ToParam())
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Actually, let me ask: what is 365 * 24?")))

	params.Messages = messages
	response3, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in third conversation turn")

	helper.ValidateMessageResponse(t, response3, "Third conversation turn")

	thirdResponse := ""
	for _, block := range response3.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			thirdResponse += textBlock.Text
		}
	}
	t.Logf("Assistant (third): %s", thirdResponse)

	// Verify calculation
	if !testutil.ContainsCaseInsensitive(thirdResponse, "8760") && !testutil.ContainsCaseInsensitive(thirdResponse, "eight thousand") && !testutil.ContainsCaseInsensitive(thirdResponse, "8,760") {
		t.Errorf("Expected calculation result 8760, got: %s", thirdResponse)
	}

	t.Logf("Conversation completed successfully with %d turns", len(messages)/2+1)
}

func TestConversationWithTools(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Start conversation with tool requirements
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("I need help with some calculations and weather information for my trip planning.")),
	}

	t.Logf("Starting conversation with tools...")

	// Define tools
	calculatorTool := anthropic.ToolParam{
		Name:        "calculate",
		Description: anthropic.String("Perform mathematical calculations"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"expression": map[string]string{
					"type": "string",
				},
			},
			Required: []string{"expression"},
		},
	}

	weatherTool := anthropic.ToolParam{
		Name:        "get_current_weather",
		Description: anthropic.String("Get the current weather for a specified location"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"location": map[string]string{
					"type": "string",
				},
			},
			Required: []string{"location"},
		},
	}

	tools := []anthropic.ToolUnionParam{
		{OfTool: &calculatorTool},
		{OfTool: &weatherTool},
	}

	// First turn - should trigger tool calls
	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 1024,
	}

	response, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in conversation with tools")

	helper.ValidateMessageResponse(t, response, "Tool conversation first turn")

	// Check for tool calls
	if response.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Tool calls detected: %d", len(response.Content))

		// Process tool calls
		var toolResults []string
		for _, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				var args map[string]interface{}
				err = json.Unmarshal([]byte(toolUseBlock.Input), &args)
				helper.AssertNoError(t, err, "Failed to parse tool arguments")

				var result string
				switch toolUseBlock.Name {
				case "calculate":
					calcResult := simulateCalculatorFunction(args)
					result = fmt.Sprintf("%v", calcResult)
				case "get_current_weather":
					result = simulateWeatherFunction(args)
				default:
					result = "Unknown function"
				}

				toolResults = append(toolResults, result)
				t.Logf("Tool %s result: %s", toolUseBlock.Name, result)
			}
		}

		// Continue conversation with tool results
		messages = append(messages, response.ToParam())
		for i, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" && i < len(toolResults) {
				toolResult := anthropic.NewToolResultBlock(toolUseBlock.ID, toolResults[i], false)
				messages = append(messages, anthropic.NewUserMessage(toolResult))
			}
		}

		// Add follow-up question
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Based on that information, should I pack an umbrella?")))

		// Second turn with tool results
		params.Messages = messages
		response2, err := helper.Client.Messages.New(ctx, params)
		helper.AssertNoError(t, err, "Failed in tool conversation second turn")

		helper.ValidateMessageResponse(t, response2, "Tool conversation second turn")

		secondResponse := ""
		for _, block := range response2.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				secondResponse += textBlock.Text
			}
		}
		t.Logf("Final response: %s", secondResponse)

		// Verify response incorporates tool results
		if len(secondResponse) == 0 {
			t.Error("Expected non-empty final response")
		}
	} else {
		t.Logf("No tool calls in first turn, continuing conversation normally")
	}
}

func TestConversationContextPreservation(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Test context preservation across multiple turns
	var messages []anthropic.MessageParam

	// Turn 1: Greeting and topic introduction
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Hi, I'm working on a science project about space.")))

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: "You are a helpful assistant knowledgeable about space and astronomy."}},
	}

	response, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in context preservation turn 1")

	helper.ValidateMessageResponse(t, response, "Context preservation turn 1")

	response1 := ""
	for _, block := range response.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response1 += textBlock.Text
		}
	}
	t.Logf("Turn 1: %s", response1)

	// Verify topic understanding
	if !testutil.ContainsCaseInsensitive(response1, "space") && !testutil.ContainsCaseInsensitive(response1, "science") {
		t.Errorf("Expected response to acknowledge space/science topic, got: %s", response1)
	}

	// Turn 2: Follow-up question (context should be preserved)
	messages = append(messages, response.ToParam())
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("What about black holes? Are they really holes?")))

	params.Messages = messages
	response2, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in context preservation turn 2")

	helper.ValidateMessageResponse(t, response2, "Context preservation turn 2")

	response2Text := ""
	for _, block := range response2.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response2Text += textBlock.Text
		}
	}
	t.Logf("Turn 2: %s", response2Text)

	// Turn 3: Another follow-up (should maintain context)
	messages = append(messages, response2.ToParam())
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("How do they form?")))

	params.Messages = messages
	response3, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in context preservation turn 3")

	helper.ValidateMessageResponse(t, response3, "Context preservation turn 3")

	response3Text := ""
	for _, block := range response3.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response3Text += textBlock.Text
		}
	}
	t.Logf("Turn 3: %s", response3Text)

	// Verify all responses are related to space/astronomy
	topics := []string{"space", "black hole", "form", "star", "gravity"}
	contextScore := 0
	for _, response := range []string{response1, response2Text, response3Text} {
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

	t.Logf("Context preservation test completed with conversation of %d messages", len(messages))
}

func TestConversationSystemPrompt(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t)

	ctx := helper.CreateTestContext()

	// Test system prompt influence on conversation
	systemPrompt := "You are a helpful cooking assistant. Provide recipes and cooking tips in a friendly, encouraging way."

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("I want to make pasta tonight. Any suggestions?")),
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: systemPrompt}},
	}

	response, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed with system prompt")

	helper.ValidateMessageResponse(t, response, "System prompt test")

	responseText := ""
	for _, block := range response.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			responseText += textBlock.Text
		}
	}
	t.Logf("Response with cooking system prompt: %s", responseText)

	// Verify cooking context
	cookingTerms := []string{"pasta", "recipe", "cook", "ingredient", "boil"}
	foundTerms := 0
	for _, term := range cookingTerms {
		if testutil.ContainsCaseInsensitive(responseText, term) {
			foundTerms++
		}
	}

	if foundTerms < 2 {
		t.Errorf("Expected response to contain cooking terms, found %d/%d", foundTerms, len(cookingTerms))
	}

	// Continue conversation with different topic
	messages = append(messages, response.ToParam())
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Actually, what about making pizza instead?")))

	params.Messages = messages
	response2, err := helper.Client.Messages.New(ctx, params)
	helper.AssertNoError(t, err, "Failed in cooking conversation continuation")

	helper.ValidateMessageResponse(t, response2, "Cooking conversation continuation")

	response2Text := ""
	for _, block := range response2.Content {
		if textBlock := block.AsText(); textBlock.Text != "" {
			response2Text += textBlock.Text
		}
	}
	t.Logf("Pizza response: %s", response2Text)

	// Verify continued cooking context
	if !testutil.ContainsCaseInsensitive(response2Text, "pizza") && !testutil.ContainsCaseInsensitive(response2Text, "dough") {
		t.Errorf("Expected pizza response to contain pizza-related terms, got: %s", response2Text)
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
