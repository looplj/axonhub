package anthropic

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultMaxTokens = 8192

const (
	defaultThreadHeader = "AH-Thread-Id"
	defaultTraceHeader  = "AH-Trace-Id"
	defaultMaxRetries   = 3
)

type Provider struct {
	client          anthropic.Client
	threadHeader    string
	traceHeader     string
	reasoningEffort string
	maxRetries      int
}

type Option func(*Provider)

func WithThreadHeader(header string) Option {
	return func(p *Provider) {
		p.threadHeader = header
	}
}

func WithTraceHeader(header string) Option {
	return func(p *Provider) {
		p.traceHeader = header
	}
}

func WithReasoningEffort(effort string) Option {
	return func(p *Provider) {
		p.reasoningEffort = effort
	}
}

func WithMaxRetries(retries int) Option {
	return func(p *Provider) {
		if retries > 0 {
			p.maxRetries = retries
		}
	}
}

func New(baseURL, apiKey string, opts ...Option) *Provider {
	p := &Provider{
		client: anthropic.NewClient(
			option.WithBaseURL(baseURL),
			option.WithAPIKey(apiKey)),
		threadHeader: defaultThreadHeader,
		traceHeader:  defaultTraceHeader,
		maxRetries:   defaultMaxRetries,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func reasoningEffortToBudget(effort string) int64 {
	switch effort {
	case "off", "none":
		return 0
	case "low":
		return 5000
	case "high":
		return 30000
	case "medium":
		return 15000
	default:
		return 0
	}
}
