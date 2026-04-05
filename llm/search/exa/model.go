package exa

// SearchRequest is the Exa search API request payload.
// See: https://docs.exa.ai/reference/search
type SearchRequest struct {
	Query          string           `json:"query"`
	NumResults     int              `json:"numResults,omitempty"`
	IncludeDomains []string         `json:"includeDomains,omitempty"`
	ExcludeDomains []string         `json:"excludeDomains,omitempty"`
	Contents       *RequestContents `json:"contents,omitempty"`
}

type RequestContents struct {
	Text bool `json:"text"`
}

type SearchResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Text          string   `json:"text"`
	PublishedDate string   `json:"publishedDate,omitempty"`
	Favicon       string   `json:"favicon,omitempty"`
	Score         *float64 `json:"score,omitempty"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
}
