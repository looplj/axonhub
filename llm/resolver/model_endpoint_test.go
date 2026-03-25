package resolver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveEndpoint_FixtureFamilies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		models   []string
		expected EndpointType
	}{
		{
			name:     "responses fixtures",
			models:   ResponsesModelFixtures,
			expected: EndpointResponses,
		},
		{
			name:     "messages fixtures",
			models:   MessagesModelFixtures,
			expected: EndpointMessages,
		},
		{
			name:     "chat completions fixtures",
			models:   ChatCompletionsModelFixtures,
			expected: EndpointChatCompletions,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, model := range tt.models {
				model := model
				t.Run(model, func(t *testing.T) {
					t.Parallel()
					endpoint, err := ResolveEndpoint(model)
					require.NoError(t, err)
					require.Equal(t, tt.expected, endpoint)
				})
			}
		})
	}
}

func TestResolveEndpoint_UsesContainsWithSuffixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		model    string
		expected EndpointType
	}{
		{
			name:     "gpt with date suffix maps to chat completions",
			model:    "gpt-4-20250514",
			expected: EndpointChatCompletions,
		},
		{
			name:     "claude with date suffix maps to messages",
			model:    "claude-3-opus-20240229",
			expected: EndpointMessages,
		},
		{
			name:     "gpt 5.4 with suffix maps to responses",
			model:    "gpt-5.4-mini-20250514",
			expected: EndpointResponses,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			endpoint, err := ResolveEndpoint(tt.model)
			require.NoError(t, err)
			require.Equal(t, tt.expected, endpoint)
		})
	}
}

func TestResolveEndpoint_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model    string
		expected EndpointType
	}{
		{model: "CoDeX-MINI-LATEST", expected: EndpointResponses},
		{model: "GPT-5.4", expected: EndpointResponses},
		{model: "ClAuDe-3-OpUs-20240229", expected: EndpointMessages},
		{model: "GeMiNi-2.5-Pro", expected: EndpointChatCompletions},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			endpoint, err := ResolveEndpoint(tt.model)
			require.NoError(t, err)
			require.Equal(t, tt.expected, endpoint)
		})
	}
}

func TestUnknownModel(t *testing.T) {
	t.Parallel()

	unknownModels := []string{
		"",
		"   ",
		"not-a-real-model",
		"my-enterprise-alias",
	}

	for _, model := range unknownModels {
		model := model
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			endpoint, err := ResolveEndpoint(model)
			require.Error(t, err)
			require.Empty(t, endpoint)
			require.ErrorContains(t, err, "unknown model")
		})
	}
}
