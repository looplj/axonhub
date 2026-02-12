package main

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/looplj/axonhub/anthropic_test/internal/testutil"
)

func TestWebSearchTool(t *testing.T) {
	helper := testutil.NewTestHelper(t, "web_search")

	ctx := helper.CreateTestContext()

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("What are the latest developments in AI in 2025?")),
	}

	webSearchTool := anthropic.WebSearchTool20250305Param{
		Name: constant.WebSearch("web_search"),
		Type: constant.WebSearch20250305("web_search_20250305"),
	}

	tools := []anthropic.ToolUnionParam{
		{OfWebSearchTool20250305: &webSearchTool},
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 1024,
	}

	response, err := helper.CreateMessageWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed in web search tool call")

	helper.ValidateMessageResponse(t, response, "Web search tool test")

	if response.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Web search tool call detected: %d", len(response.Content))

		for _, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				t.Logf("Tool call: %s", toolUseBlock.Name)
				t.Logf("Tool input: %s", toolUseBlock.Input)
			}
		}
	} else {
		t.Logf("No tool calls detected, checking direct response")

		responseText := ""
		for _, block := range response.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				responseText += textBlock.Text
			}
		}

		if len(responseText) == 0 {
			t.Error("Expected non-empty response")
		}

		t.Logf("Direct response: %s", responseText)
	}
}

func TestWebSearchToolWithUserLocation(t *testing.T) {
	helper := testutil.NewTestHelper(t, "web_search_location")

	ctx := helper.CreateTestContext()

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather like today?")),
	}

	webSearchTool := anthropic.WebSearchTool20250305Param{
		Name: constant.WebSearch("web_search"),
		Type: constant.WebSearch20250305("web_search_20250305"),
		UserLocation: anthropic.WebSearchTool20250305UserLocationParam{
			Type:     constant.Approximate("approximate"),
			City:     anthropic.String("San Francisco"),
			Country:  anthropic.String("US"),
			Region:   anthropic.String("California"),
			Timezone: anthropic.String("America/Los_Angeles"),
		},
	}

	tools := []anthropic.ToolUnionParam{
		{OfWebSearchTool20250305: &webSearchTool},
	}

	params := anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 1024,
	}

	response, err := helper.CreateMessageWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed in web search tool with location call")

	helper.ValidateMessageResponse(t, response, "Web search tool with location test")

	if response.StopReason == anthropic.StopReasonToolUse {
		t.Logf("Web search tool call with location detected")

		for _, block := range response.Content {
			if toolUseBlock := block.AsToolUse(); toolUseBlock.Name != "" {
				t.Logf("Tool call: %s", toolUseBlock.Name)
				t.Logf("Tool input: %s", toolUseBlock.Input)
			}
		}
	} else {
		responseText := ""
		for _, block := range response.Content {
			if textBlock := block.AsText(); textBlock.Text != "" {
				responseText += textBlock.Text
			}
		}

		t.Logf("Direct response: %s", responseText)
	}
}
