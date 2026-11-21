// 职责: 验证 AutoTrust 原生决策协议的路由信息、图像状态、结果和错误处理。
// 关联文件:
//
//	llm/transformer/typesafe/autotrust.go
//	llm/systemone.go
//	llm/transformer/interfaces.go
//
// 注意事项:
//
//	这些测试使用协议样例验证转换行为，不代替真实模型联调。
package typesafe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestAutoTrustNativeRequest(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		body   string
		header string
		model  string
	}{
		{name: "header model with default state", body: `{"kind":"noul","question":"Is this valid?"}`, header: "decision-alias", model: "decision-alias"},
		{name: "body model with object state", body: `{"model":"decision-alias","kind":"score","state":{"image":"textual-field"},"question":"How good?"}`, model: "decision-alias"},
		{name: "matching models and image", body: `{"model":"decision-alias","kind":"choice","state":["Pick",{"image":"data:image/png;base64,aW1hZ2U="}],"question":"Which?","options":["later","first"],"thinking":"on","custom":{"large_id":9007199254740993}}`, header: "decision-alias", model: "decision-alias"},
		{name: "image_url content", body: `{"model":"vision","kind":"noul","state":[{"type":"image_url","image_url":{"url":"https://example.com/picture.png"}}],"question":"A cat?"}`, model: "vision"},
		{name: "text mentioning image", body: `{"model":"text","kind":"noul","state":["image",{"type":"text","text":"image_url"}],"question":"A cat?"}`, model: "text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			headers := make(http.Header)
			headers.Set(AutoTrustModelHeader, tc.header)
			request := &httpclient.Request{Body: []byte(tc.body), Headers: headers, ContentType: "application/json; charset=utf-8"}
			got, err := NewAutoTrustInboundTransformer().TransformRequest(t.Context(), request)
			require.NoError(t, err)
			require.Equal(t, tc.model, got.Model)
			require.Equal(t, llm.APIFormatAutoTrustDecide, got.APIFormat)
			require.Equal(t, llm.RequestTypeSystemOne, got.RequestType)
			require.Empty(t, got.Messages)
			require.Empty(t, got.SystemOne.Questions)
			require.Equal(t, tc.body, string(got.SystemOne.AutoTrust.Body))
		})
	}
}

func TestAutoTrustRejectsInvalidRequest(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		body        string
		header      string
		contentType string
	}{
		{name: "empty body"},
		{name: "trailing data", body: `{"model":"m","kind":"noul","question":"q"} {}`},
		{name: "nonobject", body: `[]`},
		{name: "missing model", body: `{"kind":"noul","question":"q"}`},
		{name: "conflicting model", body: `{"model":"one","kind":"noul","question":"q"}`, header: "two"},
		{name: "invalid kind", body: `{"model":"m","kind":"box","question":"q"}`},
		{name: "missing question", body: `{"model":"m","kind":"noul"}`},
		{name: "null question", body: `{"model":"m","kind":"noul","question":null}`},
		{name: "numeric question", body: `{"model":"m","kind":"noul","question":1}`},
		{name: "null state", body: `{"model":"m","kind":"noul","question":"q","state":null}`},
		{name: "boolean state", body: `{"model":"m","kind":"noul","question":"q","state":true}`},
		{name: "missing options", body: `{"model":"m","kind":"choice","question":"q"}`},
		{name: "one option", body: `{"model":"m","kind":"choice","question":"q","options":["a"]}`},
		{name: "null option", body: `{"model":"m","kind":"choice","question":"q","options":["a",null]}`},
		{name: "numeric option", body: `{"model":"m","kind":"choice","question":"q","options":["a",2]}`},
		{name: "stream", body: `{"model":"m","kind":"noul","question":"q","stream":true}`},
		{name: "multipart", body: `{"model":"m","kind":"noul","question":"q"}`, contentType: "multipart/form-data; boundary=test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inbound := NewAutoTrustInboundTransformer()
			headers := make(http.Header)
			headers.Set(AutoTrustModelHeader, tc.header)
			_, err := inbound.TransformRequest(t.Context(), &httpclient.Request{Body: []byte(tc.body), Headers: headers, ContentType: tc.contentType})
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
			require.Equal(t, http.StatusBadRequest, inbound.TransformError(t.Context(), err).StatusCode)
		})
	}
}

func TestAutoTrustOutboundPreservesNativeFieldsAndCleansRouting(t *testing.T) {
	t.Parallel()
	outbound, err := NewAutoTrustOutboundTransformerWithConfig(&Config{
		BaseURL: "https://decisions.example/v1", APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)
	inbound := NewAutoTrustInboundTransformer()
	original := []byte(`{"model":"alias","kind":"choice","state":["Context",{"image":"data:image/png;base64,aW1hZ2U="}],"question":"Which?","options":["second","first"],"thinking":"on","chat_template_kwargs":{"enable_thinking":true},"custom":{"id":9007199254740993}}`)
	request, err := inbound.TransformRequest(t.Context(), &httpclient.Request{Body: original})
	require.NoError(t, err)
	request.Model = "mapped-deployment-name"
	got, err := outbound.TransformRequest(t.Context(), request)
	require.NoError(t, err)
	require.Equal(t, "https://decisions.example/v1/decide", got.URL)
	require.Equal(t, http.MethodPost, got.Method)
	require.Equal(t, "test-key", got.Auth.APIKey)
	require.False(t, gjson.GetBytes(got.Body, "model").Exists())
	require.Equal(t, "9007199254740993", gjson.GetBytes(got.Body, "custom.id").Raw)
	require.Equal(t, "second", gjson.GetBytes(got.Body, "options.0").String())
	require.True(t, gjson.GetBytes(got.Body, "chat_template_kwargs.enable_thinking").Bool())
	require.Equal(t, "alias", gjson.GetBytes(request.SystemOne.AutoTrust.Body, "model").String())
	require.False(t, outbound.AllowPassThroughBody(t.Context(), request, got))
	require.False(t, outbound.AllowPassThroughResponse(t.Context(), request, got))

	got.Body, err = sjson.SetBytes(got.Body, "model", "override-model")
	require.NoError(t, err)
	got.Body, err = sjson.SetBytes(got.Body, "thinking", "off")
	require.NoError(t, err)
	got.JSONBody = got.Body
	got.Headers.Set(AutoTrustModelHeader, "override-header")
	got.Headers["x-axonhub-model"] = []string{"lowercase-header"}
	got.Headers.Set("X-Provider-Option", "preserved")
	finalized := outbound.FinalizeTransportRequest(got)
	require.False(t, gjson.GetBytes(finalized.Body, "model").Exists())
	require.Equal(t, "off", gjson.GetBytes(finalized.Body, "thinking").String())
	require.Equal(t, finalized.Body, finalized.JSONBody)
	require.Empty(t, finalized.Headers.Get(AutoTrustModelHeader))
	require.NotContains(t, finalized.Headers, "x-axonhub-model")
	require.Equal(t, "preserved", finalized.Headers.Get("X-Provider-Option"))
	require.Equal(t, "override-model", gjson.GetBytes(got.Body, "model").String())

	_, err = outbound.TransformRequest(t.Context(), &llm.Request{Model: "m", SystemOne: &llm.SystemOneRequest{State: "x"}})
	require.Error(t, err)
}

func TestAutoTrustEndpointURLs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, base, path, want string }{
		{name: "root", base: "https://example.com/", want: "https://example.com/v1/decide"},
		{name: "version", base: "https://example.com/v1/", want: "https://example.com/v1/decide"},
		{name: "custom prefix", base: "https://example.com/gateway/", path: "/v1/decide", want: "https://example.com/gateway/v1/decide"},
		{name: "exact endpoint", base: "https://example.com/native/decide##", want: "https://example.com/native/decide"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outbound, err := NewAutoTrustOutboundTransformerWithConfig(&Config{BaseURL: tc.base, EndpointPath: tc.path, APIKeyProvider: auth.NewStaticKeyProvider("key")})
			require.NoError(t, err)
			request, err := NewAutoTrustInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: []byte(`{"model":"m","kind":"noul","question":"q"}`)})
			require.NoError(t, err)
			got, err := outbound.TransformRequest(t.Context(), request)
			require.NoError(t, err)
			require.Equal(t, tc.want, got.URL)
		})
	}
}

func TestAutoTrustNativeResponseRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		body       string
		wantUsage  *llm.Usage
		wantChoice string
	}{
		{name: "choice with zero index and extensions", body: `{"kind":"choice","effective_kind":"choice","options":["z","a"],"probabilities":[1,0],"choice_index":0,"choice":"z","model":"loaded-model","thinking":{"reasoning":""},"usage":{"prompt_tokens":11,"completion_tokens":2,"total_tokens":13},"num_model_requests":3,"elapsed_seconds":0.2,"custom":9007199254740993}`, wantUsage: &llm.Usage{PromptTokens: 11, CompletionTokens: 2, TotalTokens: 13}, wantChoice: "z"},
		{name: "noul with zero usage", body: `{"kind":"noul","options":["false","true"],"probabilities":[0,1],"choice_index":1,"choice":"true","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`, wantUsage: &llm.Usage{}, wantChoice: "true"},
		{name: "score without usage", body: `{"kind":"score","options":["0","1","2","3","4","5"],"probabilities":[0,0,0,0,0.1,0.9],"choice_index":5,"choice":"5"}`, wantChoice: "5"},
		{name: "system2 usage without total", body: `{"kind":"noul","options":["false","true"],"probabilities":[0.8,0.2],"choice_index":0,"choice":"false","system":2,"usage":{"prompt_tokens":10,"completion_tokens":4}}`, wantUsage: &llm.Usage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14}, wantChoice: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outbound := &AutoTrustOutboundTransformer{}
			response, err := outbound.TransformResponse(t.Context(), &httpclient.Response{StatusCode: http.StatusOK, Body: []byte(tc.body)})
			require.NoError(t, err)
			require.Equal(t, llm.APIFormatAutoTrustDecide, response.APIFormat)
			require.Equal(t, tc.wantUsage, response.Usage)
			require.Empty(t, response.SystemOne.Answers)
			require.Equal(t, tc.wantChoice, response.SystemOne.AutoTrust.Choice)
			got, err := NewAutoTrustInboundTransformer().TransformResponse(t.Context(), response)
			require.NoError(t, err)
			require.Equal(t, tc.body, string(got.Body))
		})
	}
}

func TestAutoTrustRejectsInvalidSuccessResponse(t *testing.T) {
	t.Parallel()
	valid := `{"kind":"choice","options":["a","b"],"probabilities":[0.5,0.5],"choice_index":0,"choice":"a"}`
	for _, tc := range []struct {
		name  string
		path  string
		value any
	}{
		{name: "invalid kind", path: "kind", value: "box"},
		{name: "missing probabilities", path: "probabilities", value: nil},
		{name: "mismatched lengths", path: "probabilities", value: []float64{1}},
		{name: "null probability", path: "probabilities", value: []any{nil, 1}},
		{name: "negative probability", path: "probabilities", value: []float64{-0.1, 1.1}},
		{name: "null option", path: "options", value: []any{nil, "b"}},
		{name: "missing index", path: "choice_index", value: nil},
		{name: "out of bounds index", path: "choice_index", value: 2},
		{name: "negative index", path: "choice_index", value: -1},
		{name: "mismatched choice", path: "choice", value: "b"},
		{name: "negative usage", path: "usage.prompt_tokens", value: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := sjson.Set(valid, tc.path, tc.value)
			require.NoError(t, err)
			_, err = (&AutoTrustOutboundTransformer{}).TransformResponse(t.Context(), &httpclient.Response{StatusCode: http.StatusOK, Body: []byte(body)})
			require.ErrorIs(t, err, transformer.ErrInvalidResponse)
		})
	}
	for _, body := range []string{"", `{}`, `null`, valid + ` trailing`} {
		_, err := (&AutoTrustOutboundTransformer{}).TransformResponse(t.Context(), &httpclient.Response{StatusCode: http.StatusOK, Body: []byte(body)})
		require.ErrorIs(t, err, transformer.ErrInvalidResponse)
	}
}

func TestAutoTrustPreservesUpstreamError(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			body := []byte(`{"detail":[{"loc":["body","question"],"msg":"Field required"}]}`)
			headers := http.Header{"Retry-After": []string{"5"}}
			_, err := (&AutoTrustOutboundTransformer{}).TransformResponse(t.Context(), &httpclient.Response{StatusCode: status, Body: body, Headers: headers})
			var upstreamErr *httpclient.Error
			require.True(t, errors.As(err, &upstreamErr))
			require.Equal(t, status, upstreamErr.StatusCode)
			clientErr := NewAutoTrustInboundTransformer().TransformError(t.Context(), fmt.Errorf("upstream request: %w", err))
			require.Equal(t, status, clientErr.StatusCode)
			require.Equal(t, body, clientErr.Body)
			require.Equal(t, headers, clientErr.Headers)
		})
	}
}

func TestAutoTrustRejectsStream(t *testing.T) {
	t.Parallel()
	_, err := NewAutoTrustInboundTransformer().TransformStream(context.Background(), nil)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	_, err = (&AutoTrustOutboundTransformer{}).TransformStream(context.Background(), nil, nil)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
}
