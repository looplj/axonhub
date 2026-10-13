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

func TestImagesDoNotUseResponsesBridge(t *testing.T) {
	const responseBody = `{"created":123,"data":[{"b64_json":"aW1hZ2U="}]}`
	for _, input := range []struct {
		name    string
		inbound *openai.ImageInboundTransformer
		body    string
		format  llm.APIFormat
	}{
		{
			name: "generation", inbound: openai.NewImageGenerationInboundTransformer(), format: llm.APIFormatOpenAIImageGeneration,
			body: `{"model":"gpt-image-2","prompt":"draw a tree","size":"1024x1024"}`,
		},
		{
			name: "edit", inbound: openai.NewImageEditInboundTransformer(), format: llm.APIFormatOpenAIImageEdit,
			body: `{"model":"gpt-image-2","prompt":"draw a tree","size":"1024x1024","image":"data:image/png;base64,aW1hZ2U=","mask":"data:image/png;base64,bWFzaw=="}`,
		},
	} {
		for _, upstream := range []struct {
			name    string
			baseURL string
		}{
			{name: "official JSON", baseURL: "https://chatgpt.com/backend-api/codex#"},
			{name: "relay JSON", baseURL: "https://relay.example/backend-api/codex#"},
		} {
			t.Run(input.name+"/"+upstream.name, func(t *testing.T) {
				codexOutbound, err := codex.NewImageOutboundTransformer(codex.ImageParams{
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
				require.False(t, gjson.GetBytes(body, "tools").Exists())
				require.Equal(t, "gpt-image-2", gjson.GetBytes(body, "model").String())
				require.Equal(t, "draw a tree", gjson.GetBytes(body, "prompt").String())
				require.Equal(t, input.format.String(), executor.lastRequest.APIFormat)
				if input.format == llm.APIFormatOpenAIImageEdit {
					require.Equal(t, "data:image/png;base64,aW1hZ2U=", gjson.GetBytes(body, "images.0.image_url").String())
					require.False(t, gjson.GetBytes(body, "image").Exists())
					require.Equal(t, "data:image/png;base64,bWFzaw==", gjson.GetBytes(body, "mask.image_url").String())
				}
				require.Equal(t, llm.RequestTypeImage.String(), executor.lastRequest.RequestType)
				require.Contains(t, executor.lastRequest.URL, "/codex/images/")
				require.Equal(t, input.format == llm.APIFormatOpenAIImageGeneration, state.PassThroughApplied)
				require.NoError(t, err)
				require.NotNil(t, result.Response)
				require.Equal(t, "aW1hZ2U=", gjson.GetBytes(result.Response.Body, "data.0.b64_json").String())
				require.False(t, gjson.GetBytes(result.Response.Body, "output").Exists(), "Responses output must not leak into the Images response")
			})
		}
	}
}
