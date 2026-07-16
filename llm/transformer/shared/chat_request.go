package shared

import (
	"context"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

// BuildChatCompletionHTTPRequest builds a standard OpenAI-compatible
// chat-completions httpclient.Request: POST {baseURL}/chat/completions with
// JSON content-type, bearer auth, and the request's TransformerMetadata
// propagated for the round-trip.
//
// The OpenAI-compatible channels (openrouter, deepseek, doubao, moonshot,
// zai, gemini-openai) share this exact request-assembly tail. Centralizing it
// prevents copy-paste drift — the same drift that previously caused the copilot
// channel to silently drop PropagateRequestMetadata.
func BuildChatCompletionHTTPRequest(
	ctx context.Context,
	keyProvider auth.APIKeyProvider,
	baseURL string,
	body []byte,
	llmReq *llm.Request,
) *httpclient.Request {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     baseURL + "/chat/completions",
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   httpclient.AuthTypeBearer,
			APIKey: keyProvider.Get(ctx),
		},
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
	}
	RecordOpenAIChatUnsupportedNativeToolLossyDowngrades(llmReq)
	RecordResponsesLossyDowngradeDiagnosticsForTarget(llmReq, llm.APIFormatOpenAIChatCompletion)
	PropagateRequestMetadata(httpReq, llmReq)
	return httpReq
}


// RecordOpenAIChatUnsupportedNativeToolLossyDowngrades records non-Chat tool
// declarations that OpenAI-compatible RequestFromLLM intentionally omits from
// tools[]. Chat keeps function/custom tools only; image/google/web_search native
// tools are not faked into Chat function tools.
func RecordOpenAIChatUnsupportedNativeToolLossyDowngrades(llmReq *llm.Request) {
	if llmReq == nil || len(llmReq.Tools) == 0 {
		return
	}

	sourceProtocol := llmReq.APIFormat
	if sourceProtocol == "" {
		sourceProtocol = llm.APIFormatOpenAIChatCompletion
	}

	hasImageGeneration := false
	hasWebSearch := false
	hasGoogleSearch := false
	hasGoogleCodeExecution := false
	hasGoogleURLContext := false

	for _, tool := range llmReq.Tools {
		switch tool.Type {
		case llm.ToolTypeImageGeneration:
			hasImageGeneration = true
		case llm.ToolTypeWebSearch:
			hasWebSearch = true
		case llm.ToolTypeGoogleSearch:
			hasGoogleSearch = true
		case llm.ToolTypeGoogleCodeExecution:
			hasGoogleCodeExecution = true
		case llm.ToolTypeGoogleUrlContext:
			hasGoogleURLContext = true
		}
	}

	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=image_generation",
		llm.APIFormatOpenAIChatCompletion,
		hasImageGeneration,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=web_search",
		llm.APIFormatOpenAIChatCompletion,
		hasWebSearch,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=google_search",
		llm.APIFormatOpenAIChatCompletion,
		hasGoogleSearch,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=google_code_execution",
		llm.APIFormatOpenAIChatCompletion,
		hasGoogleCodeExecution,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=google_url_context",
		llm.APIFormatOpenAIChatCompletion,
		hasGoogleURLContext,
	)
}
