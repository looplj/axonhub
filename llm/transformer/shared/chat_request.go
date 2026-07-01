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
	PropagateRequestMetadata(httpReq, llmReq)
	return httpReq
}
