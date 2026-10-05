package transformer_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/jina"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestProtocolUsageCost(t *testing.T) {
	protocols := []struct {
		name       string
		newUsage   func() any
		tokenField string
	}{
		{"anthropic", func() any { return new(anthropic.Usage) }, "input_tokens"},
		{"gemini", func() any { return new(gemini.UsageMetadata) }, "promptTokenCount"},
		{"openai_chat", func() any { return new(openai.Usage) }, "prompt_tokens"},
		{"openai_responses", func() any { return new(responses.Usage) }, "input_tokens"},
		{"openai_completions", func() any { return new(openai.CompletionUsage) }, "prompt_tokens"},
		{"openai_embeddings", func() any { return new(openai.EmbeddingUsage) }, "prompt_tokens"},
		{"openai_images", func() any { return new(openai.ImagesResponseUsage) }, "input_tokens"},
		{"openai_video", func() any { return new(openai.OpenAIVideoUsage) }, "total_tokens"},
		{"jina_embeddings", func() any { return new(jina.EmbeddingUsage) }, "prompt_tokens"},
		{"jina_rerank", func() any { return new(jina.RerankUsage) }, "prompt_tokens"},
	}
	values := []struct {
		name string
		cost string
		want *float64
	}{
		{"number", "0.0001514", lo.ToPtr(0.0001514)},
		{"string", `"0.0001514"`, lo.ToPtr(0.0001514)},
		{"zero", "0", lo.ToPtr(float64(0))},
		{"string_zero", `"0"`, lo.ToPtr(float64(0))},
		{"exponent", `"1.514e-4"`, lo.ToPtr(0.0001514)},
		{"null", "null", nil},
		{"invalid_string", `"unknown"`, nil},
		{"empty_string", `""`, nil},
		{"object", `{"usd":0.0001514}`, nil},
		{"array", "[]", nil},
		{"boolean", "true", nil},
		{"overflow", "1e400", nil},
		{"nan", `"NaN"`, nil},
		{"infinity", `"Infinity"`, nil},
	}
	for _, protocol := range protocols {
		t.Run(protocol.name, func(t *testing.T) {
			for _, value := range values {
				t.Run(value.name, func(t *testing.T) {
					usage := protocol.newUsage()
					body := fmt.Sprintf(`{"%s":10,"cost":%s}`, protocol.tokenField, value.cost)
					require.NoError(t, json.Unmarshal([]byte(body), usage))
					encoded, err := json.Marshal(usage)
					require.NoError(t, err)
					var result map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(encoded, &result))
					require.JSONEq(t, "10", string(result[protocol.tokenField]))
					if value.want == nil {
						require.NotContains(t, result, "cost")
					} else {
						var cost float64
						require.NoError(t, json.Unmarshal(result["cost"], &cost))
						require.Equal(t, *value.want, cost)
					}
				})
			}
			t.Run("invalid_tokens_still_fail", func(t *testing.T) {
				body := fmt.Sprintf(`{"%s":"invalid","cost":"0.1"}`, protocol.tokenField)
				require.Error(t, json.Unmarshal([]byte(body), protocol.newUsage()))
			})
			t.Run("missing_cost", func(t *testing.T) {
				usage := protocol.newUsage()
				require.NoError(t, json.Unmarshal([]byte(`{"cost":"0.0001514"}`), usage))
				body := fmt.Sprintf(`{"%s":10}`, protocol.tokenField)
				require.NoError(t, json.Unmarshal([]byte(body), usage))
				encoded, err := json.Marshal(usage)
				require.NoError(t, err)
				require.NotContains(t, string(encoded), `"cost"`)
			})
			t.Run("malformed_json_still_fails", func(t *testing.T) {
				require.Error(t, json.Unmarshal([]byte(`{"cost":`), protocol.newUsage()))
			})
		})
	}
}
