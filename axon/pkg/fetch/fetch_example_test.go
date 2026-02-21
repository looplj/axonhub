package fetch_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/axon/pkg/fetch"
)

func ExampleClient_Fetch() {
	client := fetch.NewClient()

	ctx := context.Background()

	result, err := client.Fetch(ctx, "https://developers.cloudflare.com/fundamentals/reference/markdown-for-agents/")
	if err != nil {
		return
	}

	fmt.Printf("StatusCode: %d\n", result.StatusCode)
	fmt.Printf("ContentType: %s\n", result.ContentType)
	fmt.Printf("Truncated: %v\n", result.Truncated)
	fmt.Printf("Title: %s\n", result.Title)
	fmt.Printf("Description: %s\n", result.Description)
	fmt.Printf("HasMarkdownContent: %v\n", strings.Contains(result.Content, "Markdown for Agents"))

	// - Output:
	// StatusCode: 200
	// ContentType: text/markdown; charset=utf-8
	// Truncated: false
	// Title: Markdown for Agents
	// Description: Markdown has quickly become the lingua franca for agents and AI systems as a whole.
	// HasMarkdownContent: true
}
