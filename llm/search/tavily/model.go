package tavily

type SearchRequest struct {
	Query          string   `json:"query"`
	MaxResults     int      `json:"max_results,omitempty"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
}

type SearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score,omitempty"`
}

type SearchUsage struct {
	Credits int `json:"credits"`
}

type SearchResponse struct {
	Query        string         `json:"query"`
	Results      []SearchResult `json:"results"`
	ResponseTime float64        `json:"response_time"`
	Usage        *SearchUsage   `json:"usage,omitempty"`
	Answer       *string        `json:"answer,omitempty"`
}
