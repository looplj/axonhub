package llm

import "encoding/json"

// SearchRequest is the unified web search request.
type SearchRequest struct {
	Query          string          `json:"query" binding:"required"`
	MaxResults     *int            `json:"max_results,omitempty"` // default 5, max 20
	AllowedDomains []string        `json:"allowed_domains,omitempty"`
	BlockedDomains []string        `json:"blocked_domains,omitempty"`
	ExtraBody      json.RawMessage `json:"extra_body,omitempty"`
}

// SearchResponse is the unified web search response.
type SearchResponse struct {
	Query        string         `json:"query"`
	Results      []SearchResult `json:"results"`
	ResponseTime float64        `json:"response_time"`
}

type SearchResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Content       string   `json:"content"`
	Score         *float64 `json:"score,omitempty"`
	PublishedDate string   `json:"published_date,omitempty"`
	Favicon       string   `json:"favicon,omitempty"`
}
