package openai_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestImageEditJSONOmitsEmptyMask(t *testing.T) {
	for _, mask := range []string{`""`, `" \t\r\n "`, `null`} {
		t.Run(mask, func(t *testing.T) {
			// Given: an optional legacy mask without image content.
			request := &httpclient.Request{
				Headers: http.Header{"Content-Type": {"application/json"}},
				Body:    []byte(`{"model":"gpt-image-2","prompt":"edit","images":[{"file_id":"image-fixture"}],"mask":` + mask + `}`),
			}
			out, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
				PlatformType: openai.PlatformOpenAI, BaseURL: "https://example.invalid/v1",
				APIKeyProvider: auth.NewStaticKeyProvider("test-key"), ImageEditJSON: true,
			})
			require.NoError(t, err)
			// When: the public edit transformer parses the JSON request.
			in, err := openai.NewImageEditInboundTransformer().TransformRequest(t.Context(), request)
			// Then: neither internal nor native outbound requests contain a mask.
			require.NoError(t, err)
			require.Empty(t, in.Image.Mask)
			require.Nil(t, in.Image.MaskReference)
			wire, err := out.TransformRequest(t.Context(), in)
			require.NoError(t, err)
			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(wire.Body, &body))
			require.NotContains(t, body, "mask")
		})
	}
}

func TestImageEditJSONRejectsInvalidMask(t *testing.T) {
	for _, mask := range []string{`{}`, `{"image_url":""}`, `{"file_id":" "}`, `"not-an-image"`, `123`} {
		t.Run(mask, func(t *testing.T) {
			// Given: an invalid non-empty mask, not an omitted legacy string.
			request := &httpclient.Request{
				Headers: http.Header{"Content-Type": {"application/json"}},
				Body:    []byte(`{"prompt":"edit","images":[{"file_id":"image-fixture"}],"mask":` + mask + `}`),
			}
			// When
			in, err := openai.NewImageEditInboundTransformer().TransformRequest(t.Context(), request)
			// Then
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
			require.Nil(t, in)
		})
	}
}
