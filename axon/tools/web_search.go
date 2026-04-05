package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/pkg/search"
)

//go:embed web_search.md
var webSearchDescription string

type WebSearchTool struct {
	provider search.Provider
}

func NewWebSearchTool(provider search.Provider) *WebSearchTool {
	return &WebSearchTool{
		provider: provider,
	}
}

type webSearchInput struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	MaxResults     *int     `json:"max_results,omitempty"`
}

var (
	webSearchMinQueryLen   = 2
	webSearchExclusiveMin0 = 0.0
	webSearchMax10         = 10.0
	webSearchParameters    = jsonschema.Schema{
		Schema: "http://json-schema.org/draft-07/schema#",
		Type:   "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				MinLength:   &webSearchMinQueryLen,
				Pattern:     ".*\\S.*",
				Description: "The search query string.",
			},
			"allowed_domains": {
				Type:        "array",
				Description: "Only search within these domains.",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"blocked_domains": {
				Type:        "array",
				Description: "Exclude these domains from results.",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"max_results": {
				Type:             "integer",
				Description:      "Maximum number of results to return.",
				ExclusiveMinimum: &webSearchExclusiveMin0,
				Maximum:          &webSearchMax10,
			},
		},
		Required: []string{"query"},
		AdditionalProperties: &jsonschema.Schema{
			Not: &jsonschema.Schema{},
		},
	}
)

func (t *WebSearchTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "WebSearch",
		Description: webSearchDescription,
		Parameters:  webSearchParameters,
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, input webSearchInput) agent.ToolResult {
	if t.provider == nil {
		return ErrorResult(fmt.Errorf("web search provider is not configured"))
	}

	q := strings.TrimSpace(input.Query)

	out, err := t.provider.Search(ctx, search.Request{
		Query:          q,
		AllowedDomains: input.AllowedDomains,
		BlockedDomains: input.BlockedDomains,
		MaxResults:     input.MaxResults,
	})
	if err != nil {
		return ErrorResult(err)
	}

	if len(out.Results) == 0 {
		return TextResult("(no results)")
	}

	var sb strings.Builder

	for i, r := range out.Results {
		idx := i + 1
		fmt.Fprintf(&sb, "[%d] %s\n", idx, strings.TrimSpace(r.Title))

		if r.URL != "" {
			fmt.Fprintf(&sb, "URL: %s\n", r.URL)
		}

		if r.Content != "" {
			fmt.Fprintf(&sb, "Summary: %s\n", strings.TrimSpace(r.Content))
		}

		sb.WriteString("\n")
	}

	return TextResult(sb.String())
}
