package agent

import "context"

// Summarizer generates summaries from conversation messages.
type Summarizer interface {
	Summarize(ctx context.Context, messages []Message) (string, error)
}
