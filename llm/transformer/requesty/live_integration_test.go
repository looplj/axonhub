package requesty_test

import (
	"context"
	"os"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/requesty"
)

// TestLiveRequesty exercises the full outbound path against the real Requesty
// router: it transforms an llm.Request into an HTTP request, sends it through
// the shared httpclient, and transforms the response back into an llm.Response.
//
// It only runs when REQUESTY_API_KEY is set, so it is skipped in CI.
func TestLiveRequesty(t *testing.T) {
	apiKey := os.Getenv("REQUESTY_API_KEY")
	if apiKey == "" {
		t.Skip("REQUESTY_API_KEY not set; skipping live integration test")
	}

	tr, err := requesty.NewOutboundTransformer("https://router.requesty.ai/v1", apiKey)
	if err != nil {
		t.Fatalf("NewOutboundTransformer: %v", err)
	}

	ctx := context.Background()
	content := "Reply with exactly: ok"
	llmReq := &llm.Request{
		Model: "openai/gpt-4o-mini",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	httpReq, err := tr.TransformRequest(ctx, llmReq)
	if err != nil {
		t.Fatalf("TransformRequest: %v", err)
	}

	client := httpclient.NewHttpClient()
	httpResp, err := client.Do(ctx, httpReq)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	if httpResp.StatusCode >= 400 {
		t.Fatalf("live request failed: HTTP %d: %s", httpResp.StatusCode, string(httpResp.Body))
	}

	resp, err := tr.TransformResponse(ctx, httpResp)
	if err != nil {
		t.Fatalf("TransformResponse: %v", err)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
		t.Fatalf("no choices in response: %+v", resp)
	}

	t.Logf("live requesty response: %+v", resp.Choices[0].Message.Content)
}
