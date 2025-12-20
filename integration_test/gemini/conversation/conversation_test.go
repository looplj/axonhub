package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/looplj/axonhub/gemini_test/internal/testutil"
	"google.golang.org/genai"
)

func TestMain(m *testing.M) {
	// Set up any global test configuration here if needed
	code := m.Run()
	os.Exit(code)
}

func TestMultiTurnConversation(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "conversation")

	// Print headers for debugging
	helper.PrintHeaders(t)

	ctx := helper.CreateTestContext()

	// Start a conversation using chat
	modelName := helper.GetModel()

	t.Logf("Starting conversation...")

	// Create chat session
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)}
	chat, err := helper.CreateChatWithHeaders(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat")

	// First turn
	question1 := "Hello! I'm planning a trip to Japan. Can you help me?"
	t.Logf("Question 1: %s", question1)

	response1, err := chat.SendMessage(ctx, genai.Part{Text: question1})
	helper.AssertNoError(t, err, "Failed to start conversation")

	helper.ValidateChatResponse(t, response1, "First conversation turn")

	firstResponse := testutil.ExtractTextFromResponse(response1)
	t.Logf("Assistant (first): %s", firstResponse)

	// Verify context was preserved (should reference Japan trip)
	if !testutil.ContainsCaseInsensitive(firstResponse, "japan") && !testutil.ContainsCaseInsensitive(firstResponse, "trip") {
		t.Errorf("Expected first response to acknowledge Japan trip, got: %s", firstResponse)
	}

	// Second turn
	question2 := "What's the weather like in Tokyo this time of year?"
	t.Logf("Question 2: %s", question2)

	response2, err := chat.SendMessage(ctx, genai.Part{Text: question2})
	helper.AssertNoError(t, err, "Failed in second conversation turn")

	helper.ValidateChatResponse(t, response2, "Second conversation turn")

	secondResponse := testutil.ExtractTextFromResponse(response2)
	t.Logf("Assistant (second): %s", secondResponse)

	// Third turn with calculation
	question3 := "Actually, let me ask: what is 365 * 24?"
	t.Logf("Question 3: %s", question3)

	response3, err := chat.SendMessage(ctx, genai.Part{Text: question3})
	helper.AssertNoError(t, err, "Failed in third conversation turn")

	helper.ValidateChatResponse(t, response3, "Third conversation turn")

	thirdResponse := testutil.ExtractTextFromResponse(response3)
	t.Logf("Assistant (third): %s", thirdResponse)

	// Verify calculation
	if !testutil.ContainsAnyCaseInsensitive(thirdResponse, "8760", "8,760") && !testutil.ContainsCaseInsensitive(thirdResponse, "eight thousand") {
		t.Errorf("Expected calculation result 8760, got: %s", thirdResponse)
	}

	t.Logf("Conversation completed successfully with 3 turns")
}

func TestConversationWithTools(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "conversation")

	ctx := helper.CreateTestContext()

	modelName := helper.GetModel()

	t.Logf("Starting conversation with tools...")

	// Define tools for Gemini
	calculatorTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "calculate",
				Description: "Perform mathematical calculations",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"expression": {
							Type: genai.TypeString,
						},
					},
					Required: []string{"expression"},
				},
			},
		},
	}

	weatherTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "get_current_weather",
				Description: "Get the current weather for a specified location",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"location": {
							Type: genai.TypeString,
						},
					},
					Required: []string{"location"},
				},
			},
		},
	}

	tools := []*genai.Tool{calculatorTool, weatherTool}

	// Create chat with tools
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
		Tools:       tools,
	}
	chat, err := helper.CreateChatWithHeaders(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat with tools")

	// First turn - should trigger tool calls
	question1 := "I need help with some calculations and weather information for my trip planning. What's 100 / 4 and what's the weather in Tokyo?"
	t.Logf("Question 1: %s", question1)

	response1, err := chat.SendMessage(ctx, genai.Part{Text: question1})
	helper.AssertNoError(t, err, "Failed in conversation with tools")

	helper.ValidateChatResponse(t, response1, "Tool conversation first turn")

	// Process function calls in a loop until both tools are called
	var toolResults []string
	calledTools := make(map[string]bool)

	for len(calledTools) < 2 {
		// Check for function calls in the response
		functionCalls := response1.Candidates[0].Content.Parts
		hasNewToolCalls := false

		for _, part := range functionCalls {
			if part.FunctionCall != nil {
				hasNewToolCalls = true
				toolName := part.FunctionCall.Name

				// Skip if we've already processed this tool
				if calledTools[toolName] {
					continue
				}

				var result string
				switch toolName {
				case "calculate":
					args := part.FunctionCall.Args
					calcResult := simulateCalculatorFunction(args)
					result = fmt.Sprintf("%v", calcResult)
				case "get_current_weather":
					args := part.FunctionCall.Args
					result = simulateWeatherFunction(args)
				default:
					result = "Unknown function"
				}

				toolResults = append(toolResults, result)
				calledTools[toolName] = true
				t.Logf("Function %s result: %s", toolName, result)

				// Send function response back to chat
				functionResponse := genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						ID:   part.FunctionCall.ID,
						Name: toolName,
						Response: map[string]interface{}{
							"result": result,
						},
					},
				}
				response1, err = chat.SendMessage(ctx, functionResponse)
				helper.AssertNoError(t, err, "Failed to send function response")
			} else if part.Text != "" {
				// Regular text response
				t.Logf("Text response: %s", part.Text)
			}
		}

		// If no new tool calls in this response and we haven't called both tools yet,
		// prompt the model to use the remaining tools
		if !hasNewToolCalls && len(calledTools) < 2 {
			var remainingTools []string
			if !calledTools["calculate"] {
				remainingTools = append(remainingTools, "calculate")
			}
			if !calledTools["get_current_weather"] {
				remainingTools = append(remainingTools, "get_current_weather")
			}

			prompt := fmt.Sprintf("Please use the %s tool(s) to help with my request about calculations and weather information.",
				strings.Join(remainingTools, " and "))
			t.Logf("Prompting for remaining tools: %s", prompt)

			response1, err = chat.SendMessage(ctx, genai.Part{Text: prompt})
			helper.AssertNoError(t, err, "Failed to prompt for remaining tools")
		}
	}

	t.Logf("All tools called successfully. Called tools: %v", calledTools)

	// Second turn with tool results
	question2 := "Based on that information, should I pack an umbrella?"
	t.Logf("Question 2: %s", question2)

	response2, err := chat.SendMessage(ctx, genai.Part{Text: question2})
	helper.AssertNoError(t, err, "Failed in tool conversation second turn")

	helper.ValidateChatResponse(t, response2, "Tool conversation second turn")

	finalResponse := testutil.ExtractTextFromResponse(response2)
	t.Logf("Final response: %s", finalResponse)

	// Verify response incorporates tool results
	if len(finalResponse) == 0 {
		t.Error("Expected non-empty final response")
	}
}

func TestConversationContextPreservation(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "conversation")

	ctx := helper.CreateTestContext()

	modelName := helper.GetModel()

	// Test context preservation across multiple turns
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: "You are a helpful assistant knowledgeable about space and astronomy."}},
		},
	}
	chat, err := helper.CreateChatWithHeaders(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat for context preservation")

	// Turn 1: Greeting and topic introduction
	question1 := "Hi, I'm working on a science project about space."
	t.Logf("Question 1: %s", question1)

	response1, err := chat.SendMessage(ctx, genai.Part{Text: question1})
	helper.AssertNoError(t, err, "Failed in context preservation turn 1")

	helper.ValidateChatResponse(t, response1, "Context preservation turn 1")

	response1Text := testutil.ExtractTextFromResponse(response1)
	t.Logf("Turn 1: %s", response1Text)

	// Verify topic understanding
	if !testutil.ContainsCaseInsensitive(response1Text, "space") && !testutil.ContainsCaseInsensitive(response1Text, "science") {
		t.Errorf("Expected response to acknowledge space/science topic, got: %s", response1Text)
	}

	// Turn 2: Follow-up question (context should be preserved)
	question2 := "What about black holes? Are they really holes?"
	t.Logf("Question 2: %s", question2)

	response2, err := chat.SendMessage(ctx, genai.Part{Text: question2})
	helper.AssertNoError(t, err, "Failed in context preservation turn 2")

	helper.ValidateChatResponse(t, response2, "Context preservation turn 2")

	response2Text := testutil.ExtractTextFromResponse(response2)
	t.Logf("Turn 2: %s", response2Text)

	// Turn 3: Another follow-up (should maintain context)
	question3 := "How do they form?"
	t.Logf("Question 3: %s", question3)

	response3, err := chat.SendMessage(ctx, genai.Part{Text: question3})
	helper.AssertNoError(t, err, "Failed in context preservation turn 3")

	helper.ValidateChatResponse(t, response3, "Context preservation turn 3")

	response3Text := testutil.ExtractTextFromResponse(response3)
	t.Logf("Turn 3: %s", response3Text)

	// Verify all responses are related to space/astronomy
	topics := []string{"space", "black hole", "form", "star", "gravity"}
	contextScore := 0
	responses := []string{response1Text, response2Text, response3Text}
	for _, response := range responses {
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

	t.Logf("Context preservation test completed with conversation of 3 messages")
}

func TestConversationSystemPrompt(t *testing.T) {
	// Skip test if no API key is configured
	helper := testutil.NewTestHelper(t, "conversation")

	ctx := helper.CreateTestContext()

	modelName := helper.GetModel()

	// Test system prompt influence on conversation
	systemPrompt := "You are a helpful cooking assistant. Provide recipes and cooking tips in a friendly, encouraging way."

	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
	}
	chat, err := helper.CreateChatWithHeaders(ctx, modelName, config, nil)
	helper.AssertNoError(t, err, "Failed to create chat with system prompt")

	// First question
	question1 := "I want to make pasta tonight. Any suggestions?"
	t.Logf("Question 1: %s", question1)

	response1, err := chat.SendMessage(ctx, genai.Part{Text: question1})
	helper.AssertNoError(t, err, "Failed with system prompt")

	helper.ValidateChatResponse(t, response1, "System prompt test")

	response1Text := testutil.ExtractTextFromResponse(response1)
	t.Logf("Response with cooking system prompt: %s", response1Text)

	// Verify cooking context
	cookingTerms := []string{"pasta", "recipe", "cook", "ingredient", "boil"}
	foundTerms := 0
	for _, term := range cookingTerms {
		if testutil.ContainsCaseInsensitive(response1Text, term) {
			foundTerms++
		}
	}

	if foundTerms < 2 {
		t.Errorf("Expected response to contain cooking terms, found %d/%d", foundTerms, len(cookingTerms))
	}

	// Continue conversation with different topic
	question2 := "Actually, what about making pizza instead?"
	t.Logf("Question 2: %s", question2)

	response2, err := chat.SendMessage(ctx, genai.Part{Text: question2})
	helper.AssertNoError(t, err, "Failed in cooking conversation continuation")

	helper.ValidateChatResponse(t, response2, "Cooking conversation continuation")

	response2Text := testutil.ExtractTextFromResponse(response2)
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
