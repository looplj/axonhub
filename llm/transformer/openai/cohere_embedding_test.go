// 职责: 验证 Cohere 原生图文嵌入、模型映射、响应数量和用量契约。
// 关联文件:
//
//	llm/transformer/openai/cohere_embedding.go
//	llm/embedding.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestCohereEmbeddingNativeRoundTrip(t *testing.T) {
	t.Parallel()
	const requestBody = `{"model":"image-alias","inputs":[{"content":[{"type":"text","text":"a cat"},{"type":"image_url","image_url":{"url":"data:image/png;base64,aW1hZ2U="}}]}],"input_type":"image","output_dimension":3,"embedding_types":["float"],"truncate":"END","provider_extension":{"retain":true}}`
	inbound := NewCohereEmbeddingInboundTransformer()
	request, err := inbound.TransformRequest(t.Context(), &httpclient.Request{Body: []byte(requestBody)})
	require.NoError(t, err)
	require.Equal(t, llm.APIFormatCohereEmbed, request.APIFormat)
	require.Equal(t, llm.RequestTypeEmbedding, request.RequestType)
	require.Equal(t, 1, request.Embedding.Cohere.InputCount)
	require.Empty(t, request.Messages)
	require.Empty(t, request.Embedding.Input.GetType())
	request.Model = "qwen3-vl-embedding-8b"
	outbound, err := NewCohereEmbeddingOutboundTransformerWithConfig(&Config{
		BaseURL: "https://example.com/", EndpointPath: "/cohere/v2/embed", APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)
	sent, err := outbound.TransformRequest(t.Context(), request)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/cohere/v2/embed", sent.URL)
	require.Equal(t, "qwen3-vl-embedding-8b", gjson.GetBytes(sent.Body, "model").String())
	require.Equal(t, gjson.Get(requestBody, "inputs").Raw, gjson.GetBytes(sent.Body, "inputs").Raw)
	require.True(t, gjson.GetBytes(sent.Body, "provider_extension.retain").Bool())
	require.Equal(t, int64(3), gjson.GetBytes(sent.Body, "output_dimension").Int())
	require.Equal(t, "image-alias", gjson.GetBytes(request.Embedding.Cohere.Body, "model").String())
	require.False(t, outbound.AllowPassThroughBody(t.Context(), request, sent))
	require.False(t, outbound.AllowPassThroughResponse(t.Context(), request, sent))
	const responseBody = `{"id":"3181","model":"qwen3-vl-embedding-8b","embeddings":{"float":[[0.03056407,-0.024977088,0.035329435]],"int8":[[1,-2,3]],"uint8":[[1,2,3]],"binary":[[1]],"ubinary":[[1]],"base64":["AQID"]},"images":[{"width":1280,"height":720}],"meta":{"billed_units":{"input_tokens":898,"images":1}},"usage":{"billed_units":{"input_tokens":898,"images":1}},"extension":{"preserve":true}}`
	response, err := outbound.TransformResponse(t.Context(), &httpclient.Response{StatusCode: http.StatusOK, Request: sent, Body: []byte(responseBody)})
	require.NoError(t, err)
	require.Equal(t, int64(898), response.Usage.PromptTokens)
	require.Equal(t, int64(898), response.Usage.TotalTokens)
	require.Equal(t, 1, response.Embedding.Cohere.VectorCount)
	returned, err := inbound.TransformResponse(t.Context(), response)
	require.NoError(t, err)
	require.Equal(t, responseBody, string(returned.Body))
}

func TestCohereEmbeddingInputForms(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		input string
		count int
	}{
		{name: "texts", input: `"texts":["one","two"]`, count: 2},
		{name: "images", input: `"images":["data:image/png;base64,YQ==","data:image/jpeg;base64,Yg=="]`, count: 2},
		{name: "mixed batch", input: `"inputs":[{"content":[{"type":"text","text":"one"},{"type":"image_url","image_url":{"url":"https://example.com/image.png"}}]},{"content":[{"type":"text","text":"two"}]}]`, count: 2},
		{name: "text blocks", input: `"inputs":[{"content":[{"type":"text","text":"one"},{"type":"text","text":"two"}]}]`, count: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request, err := NewCohereEmbeddingInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{"Content-Type": {"application/json; charset=utf-8"}},
				Body:    []byte(`{"model":"qwen","input_type":"search_document",` + tc.input + `}`),
			})
			require.NoError(t, err)
			require.Equal(t, tc.count, request.Embedding.Cohere.InputCount)
		})
	}
}

func TestCohereEmbeddingPreservesOptionalInputType(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, field, expectedRaw string }{
		{name: "omitted"},
		{name: "null", field: `,"input_type":null`, expectedRaw: `null`},
		{name: "empty", field: `,"input_type":""`, expectedRaw: `""`},
		{name: "image task", field: `,"input_type":"image"`, expectedRaw: `"image"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := []byte(`{"model":"qwen-alias","inputs":[{"content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,aW1hZ2U="}}]}]` + tc.field + `}`)
			request, err := NewCohereEmbeddingInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: body})
			require.NoError(t, err)
			request.Model = "qwen3-vl-embedding-8b"
			outbound, err := NewCohereEmbeddingOutboundTransformerWithConfig(&Config{
				BaseURL: "http://localhost:8000", APIKeyProvider: auth.NewStaticKeyProvider("EMPTY"),
			})
			require.NoError(t, err)
			sent, err := outbound.TransformRequest(t.Context(), request)
			require.NoError(t, err)
			require.Equal(t, tc.expectedRaw, gjson.GetBytes(sent.Body, "input_type").Raw)
			require.Equal(t, "qwen3-vl-embedding-8b", gjson.GetBytes(sent.Body, "model").String())
			response, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
				StatusCode: http.StatusOK, Request: sent, Body: []byte(`{"embeddings":{"float":[[0.1,-0.2]]}}`),
			})
			require.NoError(t, err)
			require.Equal(t, 1, response.Embedding.Cohere.VectorCount)
		})
	}
}

func TestCohereEmbeddingRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body, contentType string }{
		{name: "missing model", body: `{"texts":["one"],"input_type":"search_query"}`},
		{name: "unknown input type", body: `{"model":"qwen","texts":["one"],"input_type":"custom"}`},
		{name: "numeric input type", body: `{"model":"qwen","texts":["one"],"input_type":42}`},
		{name: "missing inputs", body: `{"model":"qwen","input_type":"image"}`},
		{name: "null text", body: `{"model":"qwen","input_type":"search_query","texts":[null]}`},
		{name: "empty inputs", body: `{"model":"qwen","input_type":"image","inputs":[]}`},
		{name: "empty content", body: `{"model":"qwen","input_type":"image","inputs":[{"content":[]}]}`},
		{name: "missing image URL", body: `{"model":"qwen","input_type":"image","inputs":[{"content":[{"type":"image_url","image_url":{}}]}]}`},
		{name: "wrong text type", body: `{"model":"qwen","input_type":"image","inputs":[{"content":[{"type":"text","text":42}]}]}`},
		{name: "ambiguous input", body: `{"model":"qwen","input_type":"image","texts":["one"],"images":["data:image/png;base64,YQ=="]}`},
		{name: "streaming", body: `{"model":"qwen","input_type":"image","texts":["one"],"stream":true}`},
		{name: "trailing JSON", body: `{"model":"qwen","input_type":"image","texts":["one"]}{}`},
		{name: "multipart", body: `{}`, contentType: "multipart/form-data; boundary=test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inbound := NewCohereEmbeddingInboundTransformer()
			_, err := inbound.TransformRequest(t.Context(), &httpclient.Request{Headers: http.Header{"Content-Type": {tc.contentType}}, Body: []byte(tc.body)})
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
			require.Equal(t, http.StatusBadRequest, inbound.TransformError(t.Context(), err).StatusCode)
		})
	}
}

func TestCohereEmbeddingUsageAndResponseValidation(t *testing.T) {
	t.Parallel()
	outbound, err := NewCohereEmbeddingOutboundTransformerWithConfig(&Config{BaseURL: "https://example.com", APIKeyProvider: auth.NewStaticKeyProvider("test-key")})
	require.NoError(t, err)
	for _, tc := range []struct {
		name, suffix string
		tokens       *int64
	}{
		{name: "usage preferred", suffix: `,"usage":{"billed_units":{"input_tokens":20}},"meta":{"billed_units":{"input_tokens":30}}`, tokens: new(int64(20))},
		{name: "zero usage preferred", suffix: `,"usage":{"billed_units":{"input_tokens":0}},"meta":{"billed_units":{"input_tokens":30}}`, tokens: new(int64(0))},
		{name: "meta only", suffix: `,"meta":{"billed_units":{"input_tokens":30}}`, tokens: new(int64(30))},
		{name: "pure image tokens", suffix: `,"meta":{"billed_units":{"input_tokens":0,"image_tokens":44}}`, tokens: new(int64(44))},
		{name: "mixed input tokens", suffix: `,"meta":{"billed_units":{"input_tokens":26,"image_tokens":0}}`, tokens: new(int64(26))},
		{name: "both token components", suffix: `,"meta":{"billed_units":{"input_tokens":26,"image_tokens":44}}`, tokens: new(int64(70))},
		{name: "usage token pair preferred", suffix: `,"usage":{"billed_units":{"input_tokens":10,"image_tokens":44}},"meta":{"billed_units":{"input_tokens":20,"image_tokens":88}}`, tokens: new(int64(54))},
		{name: "usage image only stays one source", suffix: `,"usage":{"billed_units":{"image_tokens":44}},"meta":{"billed_units":{"input_tokens":20,"image_tokens":88}}`, tokens: new(int64(44))},
		{name: "usage input only stays one source", suffix: `,"usage":{"billed_units":{"input_tokens":0}},"meta":{"billed_units":{"image_tokens":44}}`, tokens: new(int64(0))},
		{name: "image count is not tokens", suffix: `,"meta":{"billed_units":{"input_tokens":26,"images":2}}`, tokens: new(int64(26))},
		{name: "image count without tokens is unknown", suffix: `,"meta":{"billed_units":{"images":2}}`},
		{name: "maximum token total", suffix: `,"meta":{"billed_units":{"input_tokens":9223372036854775806,"image_tokens":1}}`, tokens: new(int64(9223372036854775807))},
		{name: "usage token absent", suffix: `,"usage":{"billed_units":{"images":1}},"meta":{"billed_units":{"input_tokens":30}}`, tokens: new(int64(30))},
		{name: "unknown usage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := []byte(`{"embeddings":{"float":[[0.1,0.2]]}` + tc.suffix + `}`)
			response, err := outbound.TransformResponse(t.Context(), &httpclient.Response{StatusCode: 200, Body: body})
			require.NoError(t, err)
			require.Equal(t, json.RawMessage(body), response.Embedding.Cohere.Body)
			if tc.tokens == nil {
				require.Nil(t, response.Usage)
			} else {
				require.Equal(t, *tc.tokens, response.Usage.PromptTokens)
				require.Equal(t, *tc.tokens, response.Usage.TotalTokens)
			}
		})
	}
	for _, tc := range []struct{ name, body string }{
		{name: "missing vectors", body: `{}`},
		{name: "empty batch", body: `{"embeddings":{"float":[]}}`},
		{name: "empty vector", body: `{"embeddings":{"float":[[]]}}`},
		{name: "null number", body: `{"embeddings":{"float":[[null]]}}`},
		{name: "quoted number", body: `{"embeddings":{"float":[["1"]]}}`},
		{name: "infinite number", body: `{"embeddings":{"float":[[1e999]]}}`},
		{name: "inconsistent formats", body: `{"embeddings":{"float":[[1]],"int8":[[1],[2]]}}`},
		{name: "negative usage", body: `{"embeddings":{"float":[[1]]},"usage":{"billed_units":{"input_tokens":-1}}}`},
		{name: "null usage", body: `{"embeddings":{"float":[[1]]},"usage":{"billed_units":{"input_tokens":null}}}`},
		{name: "negative image tokens", body: `{"embeddings":{"float":[[1]]},"meta":{"billed_units":{"input_tokens":0,"image_tokens":-1}}}`},
		{name: "null image tokens", body: `{"embeddings":{"float":[[1]]},"meta":{"billed_units":{"image_tokens":null}}}`},
		{name: "string image tokens", body: `{"embeddings":{"float":[[1]]},"meta":{"billed_units":{"image_tokens":"44"}}}`},
		{name: "fractional image tokens", body: `{"embeddings":{"float":[[1]]},"meta":{"billed_units":{"image_tokens":0.5}}}`},
		{name: "image token overflow", body: `{"embeddings":{"float":[[1]]},"meta":{"billed_units":{"image_tokens":9223372036854775808}}}`},
		{name: "token sum overflow", body: `{"embeddings":{"float":[[1]]},"meta":{"billed_units":{"input_tokens":9223372036854775807,"image_tokens":1}}}`},
		{name: "invalid primary usage does not select meta", body: `{"embeddings":{"float":[[1]]},"usage":{"billed_units":{"image_tokens":null}},"meta":{"billed_units":{"input_tokens":44}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := outbound.TransformResponse(t.Context(), &httpclient.Response{StatusCode: 200, Body: []byte(tc.body)})
			require.ErrorIs(t, err, transformer.ErrInvalidResponse)
		})
	}
}

func TestCohereEmbeddingUsesActualRequestCount(t *testing.T) {
	t.Parallel()
	outbound, err := NewCohereEmbeddingOutboundTransformerWithConfig(&Config{BaseURL: "https://example.com", APIKeyProvider: auth.NewStaticKeyProvider("test-key")})
	require.NoError(t, err)
	request, err := NewCohereEmbeddingInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: []byte(`{"model":"qwen","input_type":"search_document","texts":["one"]}`)})
	require.NoError(t, err)
	sent, err := outbound.TransformRequest(t.Context(), request)
	require.NoError(t, err)
	// 参数覆盖改变批次数量后，以实际发送正文校验，不使用入站缓存的数量。
	sent.Body = []byte(`{"model":"qwen","input_type":"search_document","texts":["one","two"]}`)
	_, err = outbound.TransformResponse(t.Context(), &httpclient.Response{StatusCode: 200, Request: sent, Body: []byte(`{"embeddings":{"float":[[1]]}}`)})
	require.ErrorContains(t, err, "received 1 embeddings for 2 inputs")
	response, err := outbound.TransformResponse(t.Context(), &httpclient.Response{StatusCode: 200, Request: sent, Body: []byte(`{"embeddings":{"float":[[1],[2]]}}`)})
	require.NoError(t, err)
	require.Equal(t, 2, response.Embedding.Cohere.VectorCount)
	_, err = outbound.TransformRequest(t.Context(), &llm.Request{Model: "qwen", Embedding: &llm.EmbeddingRequest{Input: llm.EmbeddingInput{String: "one"}}})
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
}

func TestCohereEmbeddingBase64ResponseValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		vector string
		valid  bool
	}{
		{name: "encoded bytes", vector: `"AQID"`, valid: true},
		{name: "encoded float", vector: `"AACAPw=="`, valid: true},
		{name: "invalid alphabet", vector: `"%%%"`},
		{name: "missing padding", vector: `"AQI"`},
		{name: "empty string", vector: `""`},
		{name: "empty decoded data", vector: `"\n"`},
		{name: "non-string", vector: `12`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := []byte(`{"embeddings":{"base64":[` + tc.vector + `]}}`)
			response, err := (&CohereEmbeddingOutboundTransformer{}).TransformResponse(t.Context(), &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       body,
			})
			if !tc.valid {
				require.ErrorIs(t, err, transformer.ErrInvalidResponse)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, response.Embedding.Cohere.VectorCount)
			returned, err := NewCohereEmbeddingInboundTransformer().TransformResponse(t.Context(), response)
			require.NoError(t, err)
			require.Equal(t, body, returned.Body)
		})
	}
}

func TestCohereEmbeddingErrorBoundary(t *testing.T) {
	t.Parallel()
	inbound := NewCohereEmbeddingInboundTransformer()
	original := &httpclient.Error{
		StatusCode: http.StatusBadRequest,
		Headers:    http.Header{"Retry-After": {"5"}},
		Body:       []byte(`{"error":{"message":"provider detail","code":400,"param":null}}`),
	}
	var native error = (&CohereEmbeddingOutboundTransformer{}).TransformError(t.Context(), original)
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "native response", err: native},
		{name: "wrapped native response", err: fmt.Errorf("request failed: %w", native)},
		{name: "direct HTTP error", err: original},
		{name: "wrapped HTTP error", err: fmt.Errorf("request failed: %w", original)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			returned := inbound.TransformError(t.Context(), tc.err)
			require.Equal(t, original.StatusCode, returned.StatusCode)
			require.Equal(t, original.Headers, returned.Headers)
			require.Equal(t, original.Body, returned.Body)
		})
	}

	for _, tc := range []struct {
		name  string
		cause error
	}{
		{name: "outer response detail wins", cause: fmt.Errorf("request failed: %w", native)},
		{name: "typed nil HTTP cause", cause: (*httpclient.Error)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			canonical := &llm.ResponseError{
				StatusCode: http.StatusTooManyRequests,
				Detail: llm.ErrorDetail{
					Message: "safe message",
					Type:    "upstream_error",
				},
				Cause: tc.cause,
			}
			returned := inbound.TransformError(t.Context(), fmt.Errorf("handler failed: %w", error(canonical)))
			require.Equal(t, http.StatusTooManyRequests, returned.StatusCode)
			require.JSONEq(t, `{"error":{"message":"safe message","type":"upstream_error"}}`, string(returned.Body))
			require.Equal(t, tc.cause, canonical.Cause)
		})
	}

	t.Run("wrapped nil response", func(t *testing.T) {
		t.Parallel()
		var responseErr *llm.ResponseError
		returned := inbound.TransformError(t.Context(), fmt.Errorf("request failed: %w", error(responseErr)))
		require.Equal(t, http.StatusInternalServerError, returned.StatusCode)
		require.JSONEq(t, `{"error":{"message":"An unexpected error occurred","type":"unexpected_error"}}`, string(returned.Body))
	})
}

func TestCohereEmbeddingURLAndErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, base, path, expected string }{
		{name: "root", base: "https://example.com/", expected: "https://example.com/v2/embed"},
		{name: "version", base: "https://example.com/v2/", expected: "https://example.com/v2/embed"},
		{name: "custom prefix", base: "https://example.com/", path: "/cohere/v2/embed", expected: "https://example.com/cohere/v2/embed"},
		{name: "versioned custom", base: "https://example.com/cohere/v2/", path: "/embed", expected: "https://example.com/cohere/v2/embed"},
		{name: "raw", base: "https://example.com/custom##", expected: "https://example.com/custom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outbound, err := NewCohereEmbeddingOutboundTransformerWithConfig(&Config{BaseURL: tc.base, EndpointPath: tc.path, APIKeyProvider: auth.NewStaticKeyProvider("test-key")})
			require.NoError(t, err)
			sent, err := outbound.TransformRequest(t.Context(), &llm.Request{Model: "qwen", Embedding: &llm.EmbeddingRequest{Cohere: &llm.CohereEmbeddingRequest{Body: json.RawMessage(`{"model":"qwen","input_type":"search_query","texts":["one"]}`)}}})
			require.NoError(t, err)
			require.Equal(t, tc.expected, sent.URL)
			for _, code := range []int{http.StatusUnauthorized, http.StatusUnprocessableEntity} {
				original := &httpclient.Error{StatusCode: code, Body: []byte(`{"message":"provider rejected the request","detail":{"code":12}}`)}
				llmErr := outbound.TransformError(t.Context(), original)
				require.ErrorIs(t, llmErr, original)
				returned := NewCohereEmbeddingInboundTransformer().TransformError(t.Context(), llmErr)
				require.Equal(t, code, returned.StatusCode)
				require.Equal(t, original.Body, returned.Body)
			}
		})
	}
}
