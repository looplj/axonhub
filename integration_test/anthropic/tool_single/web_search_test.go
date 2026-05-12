package main

import (
	"os"
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

func validateAnthropicWebSearchResponse(t *testing.T, content []anthropic.ContentBlockUnion) {
	t.Helper()

	var (
		hasServerToolUse       bool
		hasWebSearchToolResult bool
		citationCount          int
	)

	for _, block := range content {
		switch block.Type {
		case "server_tool_use":
			toolUseBlock := block.AsServerToolUse()
			if toolUseBlock.Name != "web_search" {
				t.Fatalf("Expected server tool use name web_search, got: %#v", toolUseBlock)
			}
			if toolUseBlock.Input == nil {
				t.Fatalf("Expected non-empty server tool input, got: %#v", toolUseBlock)
			}
			hasServerToolUse = true
		case "text":
			for _, citation := range block.Citations {
				switch citation.Type {
				case "url_citation":
					if citation.URL == "" {
						t.Fatalf("Expected citation URL, got empty citation: %#v", citation)
					}
					if citation.Title == "" {
						t.Fatalf("Expected citation title, got empty citation: %#v", citation)
					}
					citationCount++
				case "web_search_result_location":
					if citation.URL == "" {
						t.Fatalf("Expected citation URL, got empty citation: %#v", citation)
					}
					if citation.Title == "" {
						t.Fatalf("Expected citation title, got empty citation: %#v", citation)
					}
					if citation.EncryptedIndex == "" {
						t.Fatalf("Expected citation encrypted_index, got empty citation: %#v", citation)
					}
					if citation.CitedText == "" {
						t.Fatalf("Expected citation cited_text, got empty citation: %#v", citation)
					}
					citationCount++
				case "search_result_location":
					if citation.Title == "" {
						t.Fatalf("Expected citation title, got empty citation: %#v", citation)
					}
					if citation.Source == "" {
						t.Fatalf("Expected citation source, got empty citation: %#v", citation)
					}
					if citation.EndBlockIndex < citation.StartBlockIndex {
						t.Fatalf("Expected ordered block indices, got start=%d end=%d", citation.StartBlockIndex, citation.EndBlockIndex)
					}
					citationCount++
				}
			}
		case "web_search_tool_result":
			resultBlock := block.AsWebSearchToolResult()
			if resultBlock.ToolUseID == "" {
				t.Fatalf("Expected web_search_tool_result.tool_use_id, got empty block: %#v", resultBlock)
			}
			results := resultBlock.Content.AsWebSearchResultBlockArray()
			if len(results) == 0 {
				t.Fatalf("Expected non-empty web search result content, got: %#v", resultBlock)
			}
			for _, result := range results {
				if result.URL == "" {
					t.Fatalf("Expected search result URL, got empty result: %#v", result)
				}
				if result.Title == "" {
					t.Fatalf("Expected search result title, got empty result: %#v", result)
				}
				if result.EncryptedContent == "" {
					t.Fatalf("Expected encrypted_content in search result, got empty result: %#v", result)
				}
			}
			hasWebSearchToolResult = true
		}
	}

	if citationCount == 0 {
		t.Fatalf("Expected at least one provider-native search citation, got response content: %#v", content)
	}

	if hasWebSearchToolResult && !hasServerToolUse {
		t.Fatalf("Expected server_tool_use block when web_search_tool_result exists, got response content: %#v", content)
	}
}

func TestValidateAnthropicWebSearchResponse_AllowsCitationOnlyFallback(t *testing.T) {
	content := []anthropic.ContentBlockUnion{
		{
			Type: "text",
			Text: "Fallback response",
		},
		{
			Type: "text",
			Text: "Recent announcement",
			Citations: []anthropic.TextCitationUnion{{
				Type:  "search_result_location",
				Title: "example.com",
				Source: "web",
			}},
		},
	}

	validateAnthropicWebSearchResponse(t, content)
}

func TestWebSearchToolReturnsProviderNativeCitations(t *testing.T) {
	helper := testutil.NewTestHelper(t, "web_search_citations")
	if searchModel := os.Getenv("TEST_ANTHROPIC_SEARCH_MODEL"); searchModel != "" {
		helper.SetModel(anthropic.Model(searchModel))
	}

	ctx := helper.CreateTestContext()
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Use web search to briefly summarize one recent AI announcement and cite the sources you used.")),
	}

	webSearchTool := anthropic.WebSearchTool20250305Param{
		Name: constant.WebSearch("web_search"),
		Type: constant.WebSearch20250305("web_search_20250305"),
	}

	stream := helper.CreateMessageStreamWithHeaders(ctx, anthropic.MessageNewParams{
		Model:     helper.GetModel(),
		Messages:  messages,
		Tools:     []anthropic.ToolUnionParam{{OfWebSearchTool20250305: &webSearchTool}},
		MaxTokens: 1024,
	})
	defer stream.Close()

	var response anthropic.Message
	for stream.Next() {
		event := stream.Current()
		err := response.Accumulate(event)
		helper.AssertNoError(t, err, "Failed to accumulate web search citation stream event")
	}

	helper.AssertNoError(t, stream.Err(), "Web search citation stream encountered an error")
	helper.ValidateMessageResponse(t, &response, "Web search provider-native citations test")

	validateAnthropicWebSearchResponse(t, response.Content)
}
