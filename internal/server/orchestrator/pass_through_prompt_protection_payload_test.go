package orchestrator

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/ollama"
	"github.com/looplj/axonhub/llm/transformer/openai"
	decisions "github.com/looplj/axonhub/llm/transformer/openai/decisions"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// TestPromptProtectedPassThroughPayload exercises real inbound and outbound
// mappings so a passing native patch cannot conceal unprotected emitted content.
func TestPromptProtectedPassThroughPayload(t *testing.T) {
	tests := []struct {
		name      string
		format    llm.APIFormat
		body      string
		pattern   string
		scopes    []objects.PromptProtectionScope
		masked    []string
		unchanged map[string]string
	}{
		{
			name: "OpenAI chat strings and multimodal parts", format: llm.APIFormatOpenAIChatCompletion,
			body:      `{"model":"alias","messages":[{"role":"system","content":"secret-system"},{"role":"user","content":"secret-user"},{"role":"user","content":[{"type":"text","text":"secret-part"},{"type":"image_url","image_url":{"url":"https://example.com/image.png"}}]}],"provider_option":{"keep":true}}`,
			scopes:    []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			masked:    []string{"messages.1.content", "messages.2.content.0.text"},
			unchanged: map[string]string{"messages.0.content": "secret-system", "messages.2.content.1.image_url.url": "https://example.com/image.png"},
		},
		{
			name: "OpenAI Responses string input and instructions", format: llm.APIFormatOpenAIResponse,
			body:   `{"model":"alias","instructions":"secret-system","input":"secret-user","provider_option":{"keep":true}}`,
			masked: []string{"instructions", "input"},
		},
		{
			name: "OpenAI Responses message text fallbacks", format: llm.APIFormatOpenAIResponse,
			body:   `{"model":"alias","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"secret-content"}]},{"type":"message","role":"user","text":"secret-message"},{"role":"user","text":"secret-default"},{"type":"input_text","role":"user","text":"secret-flat"},{"type":"message","role":"user","content":null,"text":"secret-null"}],"provider_option":{"keep":true}}`,
			masked: []string{"input.0.content.0.text", "input.1.text", "input.2.text", "input.3.text", "input.4.text"},
		},
		{
			name: "OpenAI Responses tool outputs", format: llm.APIFormatOpenAIResponse,
			body:      `{"model":"alias","input":[{"role":"user","content":"secret-user"},{"type":"function_call_output","call_id":"call_1","output":[{"type":"input_text","text":"secret-tool"}]},{"type":"custom_tool_call_output","call_id":"call_2","output":"secret-custom"}],"provider_option":{"keep":true}}`,
			scopes:    []objects.PromptProtectionScope{objects.PromptProtectionScopeTool},
			masked:    []string{"input.1.output.0.text", "input.2.output"},
			unchanged: map[string]string{"input.0.content": "secret-user"},
		},
		{
			name: "Responses Compact protects input and instructions", format: llm.APIFormatOpenAIResponseCompact,
			body:   `{"model":"alias","instructions":"secret-system","input":[{"role":"user","content":[{"type":"input_text","text":"secret-user"}]},{"role":"assistant","content":[{"type":"output_text","text":"secret-assistant"}]}],"provider_option":{"keep":true}}`,
			masked: []string{"instructions", "input.0.content.0.text", "input.1.content.0.text"},
		},
		{
			name: "Responses Compact role isolation", format: llm.APIFormatOpenAIResponseCompact,
			body:      `{"model":"alias","instructions":"secret-system","input":[{"role":"user","content":"secret-user"},{"role":"assistant","content":"secret-assistant"}],"provider_option":{"keep":true}}`,
			scopes:    []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			masked:    []string{"input.0.content"},
			unchanged: map[string]string{"instructions": "secret-system", "input.1.content": "secret-assistant"},
		},
		{
			name: "Anthropic system and tool result arrays", format: llm.APIFormatAnthropicMessage,
			body:   `{"model":"alias","max_tokens":64,"system":[{"type":"text","text":"secret-system"}],"messages":[{"role":"user","content":[{"type":"text","text":"secret-user"},{"type":"tool_result","tool_use_id":"call_1","content":[{"type":"text","text":"secret-tool"}]}]}],"provider_option":{"keep":true}}`,
			masked: []string{"system.0.text", "messages.0.content.0.text", "messages.0.content.1.content.0.text"},
		},
		{
			name: "Anthropic tool scope leaves user text unchanged", format: llm.APIFormatAnthropicMessage,
			body:      `{"model":"alias","max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"secret-user"},{"type":"tool_result","tool_use_id":"call_1","content":"secret-tool"}]}],"provider_option":{"keep":true}}`,
			scopes:    []objects.PromptProtectionScope{objects.PromptProtectionScopeTool},
			masked:    []string{"messages.0.content.1.content"},
			unchanged: map[string]string{"messages.0.content.0.text": "secret-user"},
		},
		{
			name: "Gemini tool strings preserve structure and numbers", format: llm.APIFormatGeminiContents,
			pattern:   `secret-(user|tool|nested)`,
			body:      `{"contents":[{"role":"user","parts":[{"text":"secret-user"},{"functionResponse":{"name":"lookup","response":{"token":"secret-tool","secret-property":"keep","nested":["secret-nested",true,null],"sequence":9007199254740993}}}]}],"provider_option":{"keep":true}}`,
			masked:    []string{"contents.0.parts.0.text", "contents.0.parts.1.functionResponse.response.token", "contents.0.parts.1.functionResponse.response.nested.0"},
			unchanged: map[string]string{"contents.0.parts.1.functionResponse.response.secret-property": "keep"},
		},
		{
			name: "Gemini user scope excludes tool and model", format: llm.APIFormatGeminiContents,
			body:      `{"systemInstruction":{"parts":[{"text":"secret-system"}]},"contents":[{"parts":[{"text":"secret-user"},{"functionResponse":{"name":"lookup","response":{"token":"secret-tool"}}}]},{"role":"model","parts":[{"text":"secret-assistant"}]}],"provider_option":{"keep":true}}`,
			scopes:    []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			masked:    []string{"contents.0.parts.0.text"},
			unchanged: map[string]string{"systemInstruction.parts.0.text": "secret-system", "contents.0.parts.1.functionResponse.response.token": "secret-tool", "contents.1.parts.0.text": "secret-assistant"},
		},
		{
			name: "Decisions string input", format: llm.APIFormatOpenAIDecisions,
			body:      `{"model":"alias","input":"secret-user","questions":[{"type":"choice","name":"kind"}],"x_beta":{"keep":true},"provider_option":{"keep":true}}`,
			masked:    []string{"input"},
			unchanged: map[string]string{},
		},
		{
			name: "Decisions input text and image", format: llm.APIFormatOpenAIDecisions,
			body:      `{"model":"alias","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"secret-user"},{"type":"input_image","image_url":"data:image/png;base64,secret"}]}],"questions":[{}],"provider_option":{"keep":true}}`,
			masked:    []string{"input.0.content.0.text"},
			unchanged: map[string]string{"input.0.content.1.image_url": "data:image/png;base64,secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound, outbound := promptProtectionPayloadTransformers(t, tt.format)
			pattern := tt.pattern
			if pattern == "" {
				pattern = "secret-[a-z]+"
			}
			rules := []*ent.PromptProtectionRule{{
				Pattern:  pattern,
				Settings: &objects.PromptProtectionSettings{Action: objects.PromptProtectionActionMask, Replacement: "redacted", Scopes: tt.scopes},
			}, {
				Pattern:  "redacted",
				Settings: &objects.PromptProtectionSettings{Action: objects.PromptProtectionActionMask, Replacement: "[MASKED]", Scopes: tt.scopes},
			}}
			state, emitted, _ := promptProtectionPayload(t, inbound, outbound, tt.body, &payloadPromptProtecter{rules: rules})
			require.True(t, state.PassThroughApplied)
			for _, path := range tt.masked {
				assert.Equal(t, "[MASKED]", gjson.GetBytes(emitted.Body, path).String(), path)
			}
			for path, expected := range tt.unchanged {
				assert.Equal(t, expected, gjson.GetBytes(emitted.Body, path).String(), path)
			}
			assert.True(t, gjson.GetBytes(emitted.Body, "provider_option.keep").Bool())
			if tt.format != llm.APIFormatGeminiContents {
				assert.Equal(t, "mapped-model", gjson.GetBytes(emitted.Body, "model").String())
			}
			if tt.name == "Gemini tool strings preserve structure and numbers" {
				assert.Equal(t, "9007199254740993", gjson.GetBytes(emitted.Body, "contents.0.parts.1.functionResponse.response.sequence").Raw)
				assert.Equal(t, "true", gjson.GetBytes(emitted.Body, "contents.0.parts.1.functionResponse.response.nested.1").Raw)
				assert.Equal(t, "null", gjson.GetBytes(emitted.Body, "contents.0.parts.1.functionResponse.response.nested.2").Raw)
			}
			assert.Equal(t, "application/json; charset=utf-8", emitted.Headers.Get("Content-Type"))
			assert.Equal(t, emitted.Headers.Get("Content-Type"), emitted.ContentType)
			assert.Equal(t, tt.body, string(state.RawRequest.Body))
		})
	}
}

// TestPromptProtectedPassThroughFallback verifies that partial native matches,
// structured regex semantics, and legacy protectors cannot restore original text.
func TestPromptProtectedPassThroughFallback(t *testing.T) {
	tests := []struct {
		name    string
		format  llm.APIFormat
		body    string
		pattern string
		legacy  bool
	}{
		{
			name: "Gemini regex spans JSON values despite user patch", format: llm.APIFormatGeminiContents,
			body:    `{"contents":[{"role":"user","parts":[{"text":"secret-user"},{"functionResponse":{"name":"lookup","response":{"first":"secret-tool","second":"tail"}}}]}]}`,
			pattern: `secret-user|secret-tool.*tail`,
		},
		{
			name: "Gemini normalized JSON escape differs from leaf text", format: llm.APIFormatGeminiContents,
			body:    `{"contents":[{"role":"user","parts":[{"text":"secret-user"},{"functionResponse":{"name":"lookup","response":{"token":"secret-\"quoted"}}}]}]}`,
			pattern: `secret-user|secret-\\`,
		},
		{
			name: "Gemini system regex spans parts despite user patch", format: llm.APIFormatGeminiContents,
			body:    `{"systemInstruction":{"parts":[{"text":"secret-"},{"text":"system"}]},"contents":[{"parts":[{"text":"secret-user"}]}]}`,
			pattern: `secret-user|secret-\nsystem`,
		},
		{
			name: "Gemini rule matches property name as well as user text", format: llm.APIFormatGeminiContents,
			body:    `{"contents":[{"role":"user","parts":[{"text":"secret-user"},{"functionResponse":{"name":"lookup","response":{"secret-property":"keep"}}}]}]}`,
			pattern: `secret-[a-z]+`,
		},
		{
			name: "Legacy Protect changes content without reporting rules", format: llm.APIFormatOpenAIChatCompletion,
			body:    `{"model":"alias","messages":[{"role":"user","content":"secret-user"}]}`,
			pattern: "secret-user", legacy: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound, outbound := promptProtectionPayloadTransformers(t, tt.format)
			rules := []*ent.PromptProtectionRule{{Pattern: tt.pattern, Settings: &objects.PromptProtectionSettings{Action: objects.PromptProtectionActionMask, Replacement: "[MASKED]"}}}
			var protector PromptProtecter = &payloadPromptProtecter{rules: rules}
			if tt.legacy {
				protector = &legacyPayloadPromptProtecter{rules: rules}
			}
			state, emitted, generated := promptProtectionPayload(t, inbound, outbound, tt.body, protector)
			require.False(t, state.PassThroughApplied)
			assert.Equal(t, generated, emitted.Body)
			assert.NotContains(t, string(emitted.Body), "secret-user")
			assert.NotContains(t, string(emitted.Body), "secret-")
			assert.Contains(t, string(emitted.Body), "[MASKED]")
		})
	}
}

// TestPromptProtectedCompactReject blocks protected Compact input before a
// provider payload is constructed, including instructions under the system scope.
func TestPromptProtectedCompactReject(t *testing.T) {
	for _, scope := range []objects.PromptProtectionScope{objects.PromptProtectionScopeSystem, objects.PromptProtectionScopeUser} {
		t.Run(string(scope), func(t *testing.T) {
			state := &PersistenceState{PromptProtecter: &payloadPromptProtecter{rules: []*ent.PromptProtectionRule{{
				Pattern: "secret", Settings: &objects.PromptProtectionSettings{Action: objects.PromptProtectionActionReject, Scopes: []objects.PromptProtectionScope{scope}},
			}}}}
			inbound := &PersistentInboundTransformer{wrapped: responses.NewCompactInboundTransformer(), state: state}
			request, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{"model":"alias","instructions":"secret-system","input":[{"role":"user","content":"secret-user"}]}`)})
			require.NoError(t, err)
			protected, err := protectPrompts(inbound).OnInboundLlmRequest(context.Background(), request)
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
			assert.Nil(t, protected)
		})
	}
}

// TestPromptProtectedOllamaPayload covers the real OpenAI-to-Ollama route:
// different API formats retain the masked generated body instead of raw replay.
func TestPromptProtectedOllamaPayload(t *testing.T) {
	outbound, err := ollama.NewOutboundTransformerWithConfig(&ollama.Config{BaseURL: "https://provider.example"})
	require.NoError(t, err)
	rules := []*ent.PromptProtectionRule{{Pattern: "secret", Settings: &objects.PromptProtectionSettings{Action: objects.PromptProtectionActionMask, Replacement: "[MASKED]"}}}
	state, emitted, generated := promptProtectionPayload(t, openai.NewInboundTransformer(), outbound,
		`{"model":"alias","messages":[{"role":"user","content":"secret"}]}`, &payloadPromptProtecter{rules: rules})
	require.False(t, state.PassThroughApplied)
	assert.Equal(t, generated, emitted.Body)
	assert.Equal(t, "[MASKED]", gjson.GetBytes(emitted.Body, "messages.0.content").String())
	assert.Equal(t, "mapped-model", gjson.GetBytes(emitted.Body, "model").String())

	// Ollama has no native inbound route today, but its raw-layout patcher must
	// still honor scopes if such a same-format request reaches the middleware.
	patched, err := patchPassThroughPromptProtection([]byte(`{"messages":[{"role":"user","content":"secret"},{"role":"tool","content":"secret"}],"options":{"num_ctx":8192}}`), llm.APIFormatOllamaChat, rules)
	require.NoError(t, err)
	assert.Equal(t, "[MASKED]", gjson.GetBytes(patched, "messages.0.content").String())
	assert.Equal(t, "[MASKED]", gjson.GetBytes(patched, "messages.1.content").String())
	assert.Equal(t, int64(8192), gjson.GetBytes(patched, "options.num_ctx").Int())
}

// promptProtectionPayload runs the security-relevant request stages with real
// provider codecs and returns both the emitted payload and the safe fallback.
func promptProtectionPayload(t *testing.T, codec transformer.Inbound, provider transformer.Outbound, body string, protector PromptProtecter) (*PersistenceState, *httpclient.Request, []byte) {
	t.Helper()
	ctx := context.Background()
	state := &PersistenceState{
		PromptProtecter: protector,
		CurrentCandidate: &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{
			ID: 1, Name: "protected-payload", Settings: &objects.ChannelSettings{PassThroughBody: lo.ToPtr(true)},
		}}},
	}
	inbound := &PersistentInboundTransformer{wrapped: codec, state: state}
	raw := &httpclient.Request{Body: []byte(body), Headers: http.Header{"Content-Type": {"application/json; charset=utf-8"}}, Path: "/v1beta/models/alias:generateContent"}
	request, err := inbound.TransformRequest(ctx, raw)
	require.NoError(t, err)
	request, err = protectPrompts(inbound).OnInboundLlmRequest(ctx, request)
	require.NoError(t, err)
	request.Model = "mapped-model"
	generated, err := provider.TransformRequest(ctx, request)
	require.NoError(t, err)
	fallback := append([]byte(nil), generated.Body...)
	outbound := &PersistentOutboundTransformer{wrapped: provider, state: state}
	emitted, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(ctx, generated)
	require.NoError(t, err)

	return state, emitted, fallback
}

// promptProtectionPayloadTransformers selects actual codecs without network I/O.
func promptProtectionPayloadTransformers(t *testing.T, format llm.APIFormat) (transformer.Inbound, transformer.Outbound) {
	t.Helper()
	var inbound transformer.Inbound
	var outbound transformer.Outbound
	var err error
	switch format {
	case llm.APIFormatOpenAIChatCompletion:
		inbound = openai.NewInboundTransformer()
		outbound, err = openai.NewOutboundTransformer("https://provider.example", "test-key")
	case llm.APIFormatOpenAIResponse:
		inbound = responses.NewInboundTransformer()
		outbound, err = responses.NewOutboundTransformer("https://provider.example", "test-key")
	case llm.APIFormatOpenAIResponseCompact:
		inbound = responses.NewCompactInboundTransformer()
		outbound, err = responses.NewOutboundTransformer("https://provider.example", "test-key")
	case llm.APIFormatOpenAIDecisions:
		inbound = decisions.NewInboundTransformer()
		outbound, err = decisions.NewOutboundTransformer("https://provider.example", "test-key")
	case llm.APIFormatAnthropicMessage:
		inbound = anthropic.NewInboundTransformer()
		outbound, err = anthropic.NewOutboundTransformer("https://provider.example", "test-key")
	case llm.APIFormatGeminiContents:
		inbound = gemini.NewInboundTransformer()
		outbound, err = gemini.NewOutboundTransformer("https://provider.example", "test-key")
	default:
		t.Fatalf("unsupported payload test format: %s", format)
	}
	require.NoError(t, err)

	return inbound, outbound
}

type payloadPromptProtecter struct {
	rules []*ent.PromptProtectionRule
}

// Protect delegates to the same policy evaluator used by the rule service.
func (p *payloadPromptProtecter) Protect(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	result, err := p.ProtectWithResult(ctx, request)
	return result.Request, err
}

// ProtectWithResult exposes real policy matches without database setup.
func (p *payloadPromptProtecter) ProtectWithResult(_ context.Context, request *llm.Request) (biz.PromptProtectionResult, error) {
	result := biz.ApplyPromptProtectionRules(request, p.rules)
	if result.Rejected {
		return result, biz.ErrPromptProtectionRejected
	}
	return result, nil
}

type legacyPayloadPromptProtecter struct {
	rules []*ent.PromptProtectionRule
}

// Protect returns a replacement request without raw metadata or matched rules,
// reproducing legacy implementations that previously allowed plaintext replay.
func (p *legacyPayloadPromptProtecter) Protect(_ context.Context, request *llm.Request) (*llm.Request, error) {
	cloned := *request
	cloned.RawRequest = nil
	cloned.Messages = append([]llm.Message(nil), request.Messages...)
	result := biz.ApplyPromptProtectionRules(&cloned, p.rules)
	if result.Rejected {
		return nil, biz.ErrPromptProtectionRejected
	}
	return result.Request, nil
}
