package api

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/decisions"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// TestChatCompletionRequestBodyTooLarge 覆盖实际 HTTP handler 的原始体与解压后超限。
// 只配置入站转换器：若错误意外进入上游处理，测试会失败而不是产生真实模型调用。
func TestChatCompletionRequestBodyTooLarge(t *testing.T) {
	for name, inbound := range map[string]transformer.Inbound{
		"chat":        openai.NewInboundTransformer(),
		"responses":   responses.NewInboundTransformer(),
		"compact":     responses.NewCompactInboundTransformer(),
		"completions": openai.NewCompletionInboundTransformer(),
		"anthropic":   anthropic.NewInboundTransformer(),
		"gemini":      gemini.NewInboundTransformer(),
		"decisions":   decisions.NewInboundTransformer(),
	} {
		for _, encoding := range []string{"", "gzip"} {
			t.Run(name+"/"+encoding, func(t *testing.T) {
				handler := NewChatCompletionHandlers(&orchestrator.ChatCompletionOrchestrator{Inbound: inbound})
				body := bytes.Repeat([]byte(" "), 4097)
				if encoding == "gzip" {
					var compressed bytes.Buffer
					writer := gzip.NewWriter(&compressed)
					_, err := writer.Write(body)
					require.NoError(t, err)
					require.NoError(t, writer.Close())
					body = compressed.Bytes()
				}
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
				req.Header.Set("Content-Encoding", encoding)
				req = req.WithContext(httpclient.WithRequestBodyLimit(req.Context(), 4096))
				// 模拟长度未知、由入口 MaxBytesReader 限流的请求。
				req.ContentLength = -1
				req.Body = http.MaxBytesReader(nil, req.Body, 4096)
				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				ctx.Request = req
				handler.ChatCompletion(ctx)
				require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
				require.Contains(t, recorder.Body.String(), "request body too large")
				require.NotContains(t, recorder.Body.String(), "internal_error")
			})
		}
	}
}

// TestPlaygroundRequestBodyTooLarge 保证管理端聊天也按 413 展示解压后超限。
func TestPlaygroundRequestBodyTooLarge(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bytes.Repeat([]byte(" "), 65)))
	req = req.WithContext(httpclient.WithRequestBodyLimit(req.Context(), 64))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	new(PlaygroundHandlers).ChatCompletion(ctx)
	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Contains(t, recorder.Body.String(), "request body too large")
}

func TestReadHTTPRequestPreservesOtherErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid gzip")))
	req.Header.Set("Content-Encoding", "gzip")
	_, err := readHTTPRequest(req)
	require.ErrorContains(t, err, "failed to create gzip reader")
}
