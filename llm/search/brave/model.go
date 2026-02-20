package brave

type WebSearchResponse struct {
	Web struct {
		Results []WebResult `json:"results"`
	} `json:"web"`
}

type WebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Age         string `json:"age,omitempty"`
	Favicon     string `json:"favicon,omitempty"`
}
