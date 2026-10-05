package orchestrator

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

func TestPassThroughCodexImagesPreservesConversion(t *testing.T) {
	const responseBody = `{"id":"resp_image","object":"response","created_at":123,"model":"gpt-6-luna","status":"completed","output":[{"id":"img","type":"image_generation_call","status":"completed","result":"aW1hZ2U="}]}`
	for _, input := range []struct {
		name    string
		inbound *openai.ImageInboundTransformer
		body    string
		action  string
	}{
		{
			name: "generation", inbound: openai.NewImageGenerationInboundTransformer(), action: "generate",
			body: `{"model":"gpt-image-2","prompt":"draw a tree","size":"1024x1024"}`,
		},
		{
			name: "edit", inbound: openai.NewImageEditInboundTransformer(), action: "edit",
			body: `{"model":"gpt-image-2","prompt":"draw a tree","size":"1024x1024","image":"data:image/png;base64,aW1hZ2U="}`,
		},
	} {
		for _, upstream := range []struct {
			name    string
			baseURL string
		}{
			{name: "official SSE", baseURL: "https://chatgpt.com/backend-api/codex#"},
			{name: "relay JSON", baseURL: "https://relay.example/backend-api/codex#"},
		} {
			t.Run(input.name+"/"+upstream.name, func(t *testing.T) {
				codexOutbound, err := codex.NewOutboundTransformer(codex.Params{
					BaseURL: upstream.baseURL,
					TokenProvider: oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{
						AccessToken: "test-token",
					}),
				})
				require.NoError(t, err)
				state := &PersistenceState{
					CurrentCandidate: &ChannelModelsCandidate{Channel: &biz.Channel{
						Channel:  &ent.Channel{Settings: &objects.ChannelSettings{PassThroughBody: lo.ToPtr(true)}},
						Outbound: codexOutbound,
					}},
				}
				outbound := &PersistentOutboundTransformer{state: state, wrapped: codexOutbound}
				executor := &mockExecutor{
					response: &httpclient.Response{
						StatusCode: http.StatusOK, Headers: http.Header{"Content-Type": {"application/json"}},
						Body: []byte(responseBody),
					},
					streamEvents: []*httpclient.StreamEvent{
						{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"img","type":"image_generation_call","status":"in_progress"}}`)},
						{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"img","type":"image_generation_call","status":"completed","result":"aW1hZ2U="}}`)},
						{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":` + responseBody + `}`)},
					},
				}
				pipe := pipeline.NewFactory(executor).Pipeline(input.inbound, codexOutbound, pipeline.WithMiddlewares(
					pipeline.OnLlmRequest("capture-test-request", func(_ context.Context, req *llm.Request) (*llm.Request, error) {
						state.LlmRequest = req
						state.OriginalRequestStream = req.Stream
						return req, nil
					}),
					applyPassThroughResponse(outbound, nil),
					applyPassThroughStream(outbound, nil),
					applyPassThroughRequestBody(outbound, nil),
					pipeline.OnRawRequest("link-test-response", func(_ context.Context, req *httpclient.Request) (*httpclient.Request, error) {
						executor.response.Request = req
						return req, nil
					}),
					captureRawProviderResponse(outbound, nil),
					captureRawProviderStream(outbound, nil),
				))
				result, err := pipe.Process(t.Context(), &httpclient.Request{
					Method: http.MethodPost, Headers: http.Header{"Content-Type": {"application/json"}},
					Body: []byte(input.body),
				})
				require.NotNil(t, executor.lastRequest)
				body := executor.lastRequest.Body
				require.Equal(t, "image_generation", gjson.GetBytes(body, "tools.0.type").String())
				require.Equal(t, input.action, gjson.GetBytes(body, "tools.0.action").String())
				require.Equal(t, "gpt-image-2", gjson.GetBytes(body, "tools.0.model").String())
				require.Equal(t, "gpt-6-luna", gjson.GetBytes(body, "model").String())
				require.True(t, gjson.GetBytes(body, "input").IsArray())
				require.True(t, gjson.GetBytes(body, "stream").Bool())
				require.Equal(t, llm.APIFormatOpenAIResponse.String(), executor.lastRequest.APIFormat)
				require.Equal(t, llm.RequestTypeImage.String(), executor.lastRequest.RequestType)
				require.Contains(t, executor.lastRequest.URL, "/codex/responses")
				require.False(t, state.PassThroughApplied)
				require.NoError(t, err)
				require.NotNil(t, result.Response)
				require.Equal(t, "aW1hZ2U=", gjson.GetBytes(result.Response.Body, "data.0.b64_json").String())
				require.False(t, gjson.GetBytes(result.Response.Body, "output").Exists(), "Responses output must not leak into the Images response")
			})
		}
	}
}
