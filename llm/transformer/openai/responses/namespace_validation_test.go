package responses

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func namespaceValidationOutbounds(t *testing.T) map[string]transformer.Outbound {
	t.Helper()
	chat, err := openai.NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	claude, err := anthropic.NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	google, err := gemini.NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	claudeCode, err := claudecode.NewOutboundTransformer(claudecode.Params{
		BaseURL:       "https://example.com",
		IsOfficial:    true,
		TokenProvider: oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{AccessToken: "test"}),
	})
	require.NoError(t, err)
	return map[string]transformer.Outbound{"chat": chat, "anthropic": claude, "gemini": google, "claudecode": claudeCode}
}

func TestNamespaceValidation_UnsupportedChoices(t *testing.T) {
	for provider, outbound := range namespaceValidationOutbounds(t) {
		for _, choice := range []string{
			`{"tools":[{"type":"function","name":"search","namespace":"docs"},{"type":"function","name":"read","namespace":"docs"}]}`,
			`{"type":"namespace","name":"docs"}`,
			`{"type":"namespace","name":"missing"}`,
			`{"tools":[{"type":"namespace","name":"docs"}]}`,
			`{"tools":[{"type":"function","name":"missing","namespace":"docs"}]}`,
		} {
			t.Run(provider+"/"+choice, func(t *testing.T) {
				var input Request
				require.NoError(t, json.Unmarshal([]byte(namespaceReviewRequest), &input))
				input.Tools[0].Tools = append(input.Tools[0].Tools, Tool{Type: "function", Name: "read"})
				input.Tools = append(input.Tools, Tool{Type: "function", Name: "outside"})
				require.NoError(t, json.Unmarshal([]byte(choice), &input.ToolChoice))
				raw, err := json.Marshal(input)
				require.NoError(t, err)
				request, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: raw})
				require.NoError(t, err)
				before, err := json.Marshal(request)
				require.NoError(t, err)
				wire, err := outbound.TransformRequest(t.Context(), request)
				require.ErrorIs(t, err, transformer.ErrInvalidRequest)
				require.Nil(t, wire)
				after, err := json.Marshal(request)
				require.NoError(t, err)
				require.JSONEq(t, string(before), string(after))
			})
		}
	}
}

func TestNamespaceValidation_NameConflicts(t *testing.T) {
	for provider, outbound := range namespaceValidationOutbounds(t) {
		for _, scenario := range []struct {
			name      string
			functions []llm.Function
			history   []llm.ToolCall
		}{
			{name: "namespace before direct", functions: []llm.Function{{Name: "search", Namespace: "docs"}, {Name: "docs__search"}}},
			{name: "direct before namespace", functions: []llm.Function{{Name: "docs__search"}, {Name: "search", Namespace: "docs"}}},
			{name: "ambiguous underscores", functions: []llm.Function{{Name: "c", Namespace: "a__b"}, {Name: "b__c", Namespace: "a"}}},
			{name: "duplicate namespace function", functions: []llm.Function{{Name: "search", Namespace: "docs"}, {Name: "search", Namespace: "docs"}}},
			{name: "duplicate direct function", functions: []llm.Function{{Name: "search"}, {Name: "search"}}},
			{name: "history conflicts with declaration", functions: []llm.Function{{Name: "docs__search"}}, history: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: "search", Namespace: "docs"}}}},
			{name: "history identities conflict", history: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: "c", Namespace: "a__b"}}, {Function: llm.FunctionCall{Name: "b__c", Namespace: "a"}}}},
		} {
			t.Run(provider+"/"+scenario.name, func(t *testing.T) {
				request := &llm.Request{
					Model:    "test",
					Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}}},
				}
				for _, function := range scenario.functions {
					request.Tools = append(request.Tools, llm.Tool{Type: "function", Function: function})
				}
				if len(scenario.history) > 0 {
					request.Messages = append(request.Messages, llm.Message{Role: "assistant", ToolCalls: scenario.history})
				}
				wire, err := outbound.TransformRequest(t.Context(), request)
				require.ErrorIs(t, err, transformer.ErrInvalidRequest)
				require.Nil(t, wire)
			})
		}
	}
}

func TestNamespaceValidation_SingleFunctionChoices(t *testing.T) {
	for provider, outbound := range namespaceValidationOutbounds(t) {
		for _, choice := range []string{
			`{"type":"namespace","name":"docs"}`,
			`{"type":"function","name":"search","namespace":"docs"}`,
			`{"tools":[{"type":"namespace","name":"docs"}]}`,
			`{"tools":[{"type":"function","name":"search","namespace":"docs"}]}`,
		} {
			t.Run(provider+"/"+choice, func(t *testing.T) {
				var input Request
				require.NoError(t, json.Unmarshal([]byte(namespaceReviewRequest), &input))
				require.NoError(t, json.Unmarshal([]byte(choice), &input.ToolChoice))
				raw, err := json.Marshal(input)
				require.NoError(t, err)
				request, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: raw})
				require.NoError(t, err)
				before, err := json.Marshal(request)
				require.NoError(t, err)
				wire, err := outbound.TransformRequest(t.Context(), request)
				require.NoError(t, err)
				switch provider {
				case "chat":
					var body openai.Request
					require.NoError(t, json.Unmarshal(wire.Body, &body))
					require.Equal(t, "docs__search", body.Tools[0].Function.Name)
					require.Equal(t, body.Tools[0].Function.Name, body.ToolChoice.NamedToolChoice.Function.Name)
				case "gemini":
					var body gemini.GenerateContentRequest
					require.NoError(t, json.Unmarshal(wire.Body, &body))
					require.Equal(t, "docs__search", body.Tools[0].FunctionDeclarations[0].Name)
					require.Equal(t, "ANY", body.ToolConfig.FunctionCallingConfig.Mode)
					require.Equal(t, []string{body.Tools[0].FunctionDeclarations[0].Name}, body.ToolConfig.FunctionCallingConfig.AllowedFunctionNames)
				default:
					var body anthropic.MessageRequest
					require.NoError(t, json.Unmarshal(wire.Body, &body))
					name := "docs__search"
					if provider == "claudecode" {
						name = "proxy_" + name
					}
					require.Equal(t, name, body.Tools[0].Name)
					require.Equal(t, "tool", body.ToolChoice.Type)
					require.Equal(t, body.Tools[0].Name, *body.ToolChoice.Name)
				}
				after, err := json.Marshal(request)
				require.NoError(t, err)
				require.JSONEq(t, string(before), string(after))
			})
		}
	}
}
