package search

import "context"

type Request struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	MaxResults     *int     `json:"max_results,omitempty"`
}

type Result struct {
	Title   string   `json:"title"`
	URL     string   `json:"url"`
	Content string   `json:"content"`
	Score   *float64 `json:"score,omitempty"`
}

type Response struct {
	Query   string   `json:"query"`
	Results []Result `json:"results"`
}

type Provider interface {
	Search(ctx context.Context, req Request) (Response, error)
}
