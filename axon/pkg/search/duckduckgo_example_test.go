package search_test

import (
	"context"
	"fmt"
	"log"

	"github.com/looplj/axonhub/axon/pkg/search"
)

func ExampleDuckDuckGoProvider_Search() {
	provider := search.NewDuckDuckGoProvider()

	ctx := context.Background()
	maxResults := 3

	resp, err := provider.Search(ctx, search.Request{
		Query:      "golang",
		MaxResults: &maxResults,
	})
	if err != nil {
		log.Printf("Search failed: %v", err)
		return
	}

	for _, result := range resp.Results {
		fmt.Printf("Title: %s\n", result.Title)
		fmt.Printf("URL: %s\n", result.URL)
		fmt.Printf("Snippet: %s\n\n", result.Content)
	}

	// - Output:
	// Title: Golang (programming language) - Wikipedia
	// URL: https://en.wikipedia.org/wiki/Golang
	// Snippet: Go (also known as Golang) is a statically typed, compiled programming language designed at Google.
	//
	// Title: The Go Programming Language - golang.org
	// URL: https://golang.org/
	// Snippet: Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.
	//
	// Title: Go (programming language) - Wikiwand
	// URL: https://www.wikiwand.com/en/Go_(programming_language)
	// Snippet: Go (also known as Golang) is a statically typed, compiled programming language designed at Google.
}
