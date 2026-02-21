package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const duckDuckGoBaseURL = "https://html.duckduckgo.com/html/"

type DuckDuckGoProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewDuckDuckGoProvider() *DuckDuckGoProvider {
	return &DuckDuckGoProvider{
		baseURL: duckDuckGoBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *DuckDuckGoProvider) Search(ctx context.Context, req Request) (Response, error) {
	maxResults := 5
	if req.MaxResults != nil {
		maxResults = *req.MaxResults
	}

	if maxResults <= 0 {
		maxResults = 5
	}

	if maxResults > 10 {
		maxResults = 10
	}

	formData := url.Values{}
	formData.Set("q", req.Query)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return Response{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.0")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("failed to read response: %w", err)
	}

	return Response{
		Query:   req.Query,
		Results: parseResults(string(body), maxResults),
	}, nil
}

// parseResults extracts search results from DuckDuckGo HTML response.
func parseResults(html string, maxResults int) []Result {
	var results []Result

	resultBlocks := splitByResultBlock(html)

	for _, block := range resultBlocks {
		if len(results) >= maxResults {
			break
		}

		result := extractResult(block)
		if result.Title != "" && result.URL != "" {
			results = append(results, result)
		}
	}

	return results
}

// splitByResultBlock splits HTML into individual result blocks.
func splitByResultBlock(html string) []string {
	var blocks []string

	startMarker := `class="result results_links`

	remaining := html
	for {
		startIdx := strings.Index(remaining, startMarker)
		if startIdx == -1 {
			break
		}

		remaining = remaining[startIdx:]

		nextIdx := strings.Index(remaining[50:], startMarker)
		if nextIdx != -1 {
			block := remaining[:nextIdx+50]
			blocks = append(blocks, block)
			remaining = remaining[nextIdx+50:]
		} else {
			blocks = append(blocks, remaining)
			break
		}
	}

	return blocks
}

// extractResult extracts a single search result from a result block.
func extractResult(block string) Result {
	var result Result

	// Extract URL from result__a href
	result.URL = extractAttribute(block, `href="`, `"`)
	if result.URL != "" {
		// Clean up the URL - DuckDuckGo sometimes uses redirects
		if strings.HasPrefix(result.URL, "//") {
			result.URL = "https:" + result.URL
		}
	}

	// Extract title from result__a
	titleStart := strings.Index(block, `class="result__a"`)
	if titleStart != -1 {
		titleSection := block[titleStart:]

		_, after, ok := strings.Cut(titleSection, ">")
		if ok {
			titleContent := after

			before, _, ok := strings.Cut(titleContent, "</a>")
			if ok {
				result.Title = cleanHTML(before)
			}
		}
	}

	// Extract snippet from result__snippet
	snippetStart := strings.Index(block, `class="result__snippet"`)
	if snippetStart != -1 {
		snippetSection := block[snippetStart:]

		_, after, ok := strings.Cut(snippetSection, ">")
		if ok {
			snippetContent := after

			before, _, ok := strings.Cut(snippetContent, "</a>")
			if ok {
				result.Content = cleanHTML(before)
			}
		}
	}

	return result
}

// extractAttribute extracts an attribute value from HTML.
func extractAttribute(html, attr, endChar string) string {
	idx := strings.Index(html, attr)
	if idx == -1 {
		return ""
	}

	start := idx + len(attr)

	end := strings.Index(html[start:], endChar)
	if end == -1 {
		return ""
	}

	return html[start : start+end]
}

// cleanHTML removes HTML tags and decodes common entities.
func cleanHTML(s string) string {
	// Remove HTML tags
	for {
		start := strings.Index(s, "<")
		if start == -1 {
			break
		}

		end := strings.Index(s[start:], ">")
		if end == -1 {
			break
		}

		s = s[:start] + s[start+end+1:]
	}

	// Decode common HTML entities
	replacements := map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": `"`,
		"&#39;":  "'",
		"&nbsp;": " ",
	}

	for entity, char := range replacements {
		s = strings.ReplaceAll(s, entity, char)
	}

	return strings.TrimSpace(s)
}
