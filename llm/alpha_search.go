package llm

// AlphaSearchRequest carries the original Codex/CPA alpha search JSON body.
// The protocol is intentionally kept opaque so AxonHub can proxy new search
// fields without repeatedly changing the unified model.
type AlphaSearchRequest struct {
	Body []byte `json:"-"`
}

// AlphaSearchResponse carries the upstream alpha search JSON response verbatim.
type AlphaSearchResponse struct {
	Body []byte `json:"-"`
}
