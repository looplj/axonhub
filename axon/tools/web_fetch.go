package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/pkg/fetch"
)

//go:embed web_fetch.md
var webFetchDescription string

var webFetchParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"query": {
			Type:        "string",
			MinLength:   new(1),
			Pattern:     ".*\\S.*",
			Description: "The URL to fetch (http/https only).",
		},
	},
	Required: []string{"query"},
	AdditionalProperties: &jsonschema.Schema{
		Not: &jsonschema.Schema{},
	},
}

type WebFetchTool struct{}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{}
}

type webFetchInput struct {
	Query string `json:"query"`
}

func (t *WebFetchTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "WebFetch",
		Description: webFetchDescription,
		Parameters:  webFetchParameters,
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, input webFetchInput) agent.ToolResult {
	rawURL := strings.TrimSpace(input.Query)
	client := fetch.NewClient()

	r, err := client.Fetch(ctx, rawURL)
	if err != nil {
		return ErrorResult(fmt.Errorf("web fetch failed: %w", err))
	}

	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return ErrorResult(fmt.Errorf("web fetch failed (HTTP %d)", r.StatusCode))
	}

	return TextResult(r.Content)
}
