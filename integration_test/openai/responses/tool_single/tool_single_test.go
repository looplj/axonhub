package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/looplj/axonhub/openai_test/internal/testutil"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestResponsesWebSearchReturnsCitations(t *testing.T) {
	helper := testutil.NewTestHelper(t, "TestResponsesWebSearchReturnsCitations")
	if searchModel := os.Getenv("TEST_OPENAI_SEARCH_MODEL"); searchModel != "" {
		helper.SetModel(openai.ChatModel(searchModel))
	}

	ctx := helper.CreateTestContext()
	question := "Use web search to summarize one recent AI announcement in 2-3 sentences and cite the source you used."

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(helper.GetModel()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
		Tools: []responses.ToolUnionParam{{
			OfWebSearch: &responses.WebSearchToolParam{
				Type: responses.WebSearchToolTypeWebSearch,
			},
		}},
		Include: []responses.ResponseIncludable{
			responses.ResponseIncludableWebSearchCallActionSources,
			responses.ResponseIncludableWebSearchCallResults,
		},
		ToolChoice: responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsRequired),
		},
	}

	resp, err := helper.CreateResponseWithHeaders(ctx, params)
	helper.AssertNoError(t, err, "Failed to get web search response")

	if resp == nil {
		t.Fatal("Response is nil")
	}

	output := resp.OutputText()
	if output == "" {
		t.Fatal("Expected non-empty output text")
	}

	var webSearchCalls int
	var urlCitations int

	for _, item := range resp.Output {
		switch item.Type {
		case "web_search_call":
			webSearchCalls++
			call := item.AsWebSearchCall()
			var rawCall struct {
				Action struct {
					Queries []string `json:"queries"`
				} `json:"action"`
			}
			if err := json.Unmarshal([]byte(call.RawJSON()), &rawCall); err != nil {
				t.Fatalf("Expected web search call raw JSON to unmarshal, got error: %v", err)
			}
			if len(rawCall.Action.Queries) == 0 {
				t.Fatalf("Expected included web search queries, got none in %#v", call)
			}
			if call.Action.Type == "" {
				t.Fatalf("Expected web search action type, got empty action in %#v", call)
			}
			if len(call.Action.Sources) == 0 {
				t.Fatalf("Expected included web search action sources, got none in %#v", call.Action)
			}
		case "message":
			msg := item.AsMessage()
			for _, content := range msg.Content {
				if content.Type != "output_text" {
					continue
				}
				outputText := content.AsOutputText()
				for _, annotation := range outputText.Annotations {
					if annotation.Type != "url_citation" {
						continue
					}
					citation := annotation.AsURLCitation()
					if citation.URL == "" {
						t.Fatalf("Expected citation URL, got empty citation: %#v", citation)
					}
					if citation.Title == "" {
						t.Fatalf("Expected citation title, got empty citation: %#v", citation)
					}
					if citation.EndIndex <= citation.StartIndex {
						t.Fatalf("Expected citation indices to be ordered, got start=%d end=%d", citation.StartIndex, citation.EndIndex)
					}
					urlCitations++
				}
			}
		}
	}

	if webSearchCalls == 0 {
		t.Fatalf("Expected at least one web_search_call item, got output: %#v", resp.Output)
	}
	if urlCitations == 0 {
		t.Fatalf("Expected at least one url_citation annotation, got output: %#v", resp.Output)
	}
}
