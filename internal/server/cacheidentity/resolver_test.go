package cacheidentity

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResolver_XSessionAffinity(t *testing.T) {
	resolver := NewResolver(Config{Enabled: true}, "")

	ctx := context.Background()
	req := &llm.Request{
		Model: "gpt-4o",
		RawRequest: &httpclient.Request{
			Headers: http.Header{
				"X-Session-Affinity": []string{"opencode-session-abc123"},
			},
		},
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
		},
	}

	result := resolver.Resolve(ctx, req)

	require.Equal(t, "opencode-session-abc123", result.SessionID)
	require.Equal(t, "opencode-session-abc123", result.PromptCacheKey)
	require.Equal(t, SourceSessionHeader, result.Source)
}

func TestResolver_CodexSessionTakesPrecedenceOverSessionAffinity(t *testing.T) {
	resolver := NewResolver(Config{Enabled: true}, "")

	ctx := context.Background()
	req := &llm.Request{
		Model: "gpt-4o",
		RawRequest: &httpclient.Request{
			Headers: http.Header{
				"Session_id":         []string{"codex-session-xyz"},
				"X-Session-Affinity": []string{"opencode-session-abc"},
			},
		},
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
		},
	}

	result := resolver.Resolve(ctx, req)

	require.Equal(t, "codex-session-xyz", result.SessionID)
	require.Equal(t, SourceSessionHeader, result.Source)
}

func TestResolver_ClientProvidedTakesPrecedenceOverAll(t *testing.T) {
	resolver := NewResolver(Config{Enabled: true}, "")

	ctx := context.Background()
	req := &llm.Request{
		Model:          "gpt-4o",
		PromptCacheKey: lo.ToPtr("client-key-explicit"),
		RawRequest: &httpclient.Request{
			Headers: http.Header{
				"X-Session-Affinity": []string{"opencode-session-abc"},
			},
		},
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
		},
	}

	result := resolver.Resolve(ctx, req)

	require.Equal(t, "client-key-explicit", result.PromptCacheKey)
	require.Empty(t, result.SessionID)
	require.Equal(t, SourceClientProvided, result.Source)
}

func TestResolver_DisabledReturnsNone(t *testing.T) {
	resolver := NewResolver(Config{Enabled: false}, "")

	ctx := context.Background()
	req := &llm.Request{
		Model: "gpt-4o",
		RawRequest: &httpclient.Request{
			Headers: http.Header{
				"X-Session-Affinity": []string{"opencode-session-abc"},
			},
		},
	}

	result := resolver.Resolve(ctx, req)

	require.Equal(t, SourceNone, result.Source)
	require.Empty(t, result.SessionID)
	require.Empty(t, result.PromptCacheKey)
}

func TestResolver_TrustedHosts(t *testing.T) {
	resolver := NewResolver(Config{
		Enabled:                    true,
		TrustedPromptCacheKeyHosts: []string{"ai-hub.example.com", "proxy.internal"},
	}, "")

	hosts := resolver.TrustedHosts()
	require.Equal(t, []string{"ai-hub.example.com", "proxy.internal"}, hosts)
}

func TestResolver_TrustedHostsEmptyDefault(t *testing.T) {
	resolver := NewResolver(Config{Enabled: true}, "")

	hosts := resolver.TrustedHosts()
	require.Empty(t, hosts)
}
