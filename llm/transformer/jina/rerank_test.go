// 职责: 验证文本和图文重排请求的转换，以及多模态响应字段的保留。
// 关联文件:
//   llm/rerank.go
//   llm/transformer/jina/rerank_inbound.go
//   llm/transformer/jina/outbound.go

package jina

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestRerankRequestRoundTrip(t *testing.T) {
	t.Parallel()
	const image = `{"content":[{"type":"image_url","image_url":{"url":"https://example.com/square.png"}}]}`
	const mixed = `{"content":[{"type":"text","text":"a red square"},{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}]}`
	for _, tc := range []struct {
		name      string
		query     string
		documents string
		native    bool
	}{
		{"legacy text", `"red square"`, `["red","blue"]`, false},
		{"ARAG content array", `"red square"`, `[` + mixed + `,{"content":[{"type":"text","text":"blue"}]}]`, true},
		{"mixed candidate types", `"red square"`, `["plain text",` + image + `,` + mixed + `]`, true},
		{"single image document", `"red square"`, image, true},
		{"single text document", `"red square"`, `"red"`, true},
		{"image query", image, `["red","blue"]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbound := NewRerankInboundTransformer()
			body := `{"model":"alias","query":` + tc.query + `,"documents":` + tc.documents + `,"top_n":1,"return_documents":false}`
			if tc.native {
				body = body[:len(body)-1] + `,"instruction":"Find matching images"}`
			}
			req, err := inbound.TransformRequest(t.Context(), &httpclient.Request{Body: []byte(body)})
			require.NoError(t, err)
			require.Equal(t, tc.native, len(req.Rerank.NativeBody) > 0)
			if !tc.native {
				require.Equal(t, "red square", req.Rerank.Query)
				require.Equal(t, []string{"red", "blue"}, req.Rerank.Documents)
			}

			req.Model = "qwen3-vl-reranker-8b"
			outbound, err := NewOutboundTransformer("https://example.com", "key")
			require.NoError(t, err)
			sent, err := outbound.TransformRequest(t.Context(), req)
			require.NoError(t, err)
			require.Equal(t, "https://example.com/v1/rerank", sent.URL)
			require.Equal(t, req.Model, gjson.GetBytes(sent.Body, "model").String())
			require.JSONEq(t, tc.query, gjson.GetBytes(sent.Body, "query").Raw)
			require.JSONEq(t, tc.documents, gjson.GetBytes(sent.Body, "documents").Raw)
			require.Equal(t, `1`, gjson.GetBytes(sent.Body, "top_n").Raw)
			require.Equal(t, `false`, gjson.GetBytes(sent.Body, "return_documents").Raw)
			if tc.native {
				require.Equal(t, "Find matching images", gjson.GetBytes(sent.Body, "instruction").String())
			}
			require.Equal(t, "alias", gjson.GetBytes(req.RawRequest.Body, "model").String())
		})
	}
}

func TestRerankRejectsInvalidContent(t *testing.T) {
	t.Parallel()
	inbound := NewRerankInboundTransformer()
	for _, tc := range []struct{ name, fields string }{
		{"empty query content", `"query":{"content":[]},"documents":["text"]`},
		{"empty documents", `"query":"q","documents":[]`},
		{"null document", `"query":"q","documents":[null]`},
		{"missing content", `"query":"q","documents":[{"text":"doc"}]`},
		{"missing content type", `"query":"q","documents":[{"content":[{}]}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
				Body: []byte(`{"model":"alias",` + tc.fields + `}`),
			})
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
			require.Equal(t, http.StatusBadRequest, inbound.TransformError(t.Context(), err).StatusCode)
		})
	}
}

func TestRerankResponsePreservesMultimodalDocument(t *testing.T) {
	t.Parallel()
	const parts = `[{"type":"image_url","image_url":{"url":"https://example.com/square.png"}}]`
	outbound, err := NewOutboundTransformer("https://example.com", "key")
	require.NoError(t, err)
	resp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Request:    &httpclient.Request{APIFormat: "jina/rerank"},
		Body: []byte(`{"id":"rerank-1","model":"upstream","results":[{"index":1,"relevance_score":0.9,` +
			`"document":{"text":null,"multi_modal":` + parts + `}},{"index":0,"relevance_score":0.1,"document":{"text":"text"}}],` +
			`"usage":{"prompt_tokens":7,"total_tokens":7}}`),
	})
	require.NoError(t, err)
	clientResp, err := NewRerankInboundTransformer().TransformResponse(t.Context(), resp)
	require.NoError(t, err)
	result := gjson.ParseBytes(clientResp.Body)
	require.Equal(t, "rerank-1", result.Get("id").String())
	require.Equal(t, int64(1), result.Get("results.0.index").Int())
	require.Equal(t, 0.9, result.Get("results.0.relevance_score").Float())
	require.JSONEq(t, parts, result.Get("results.0.document.multi_modal").Raw)
	require.Equal(t, "text", result.Get("results.1.document.text").String())
	require.Equal(t, int64(7), result.Get("usage.total_tokens").Int())
}
