package llm

// ReasoningEffortMapping is a single reasoning_effort value mapping entry.
// From is the inbound effort value (e.g. "xhigh"); To is the outbound value
// (e.g. "max") sent to the upstream provider.
//
// Defined at the llm root package (rather than under transformer/openai) so that
// both the OpenAI transformer implementation and internal/objects can reference
// the same strong-typed entry without internal/objects depending on a concrete
// transformer package. Mirrors the ModelMapping pattern in internal/objects.
type ReasoningEffortMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}
