package openai_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/doubao"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/modelscope"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openrouter"
	"github.com/stretchr/testify/require"
)

func TestImageReferencesRejectUnresolvedInputsBeforeUpstream(t *testing.T) {
	for name, constructor := range map[string]func(string, string) (transformer.Outbound, error){
		"gemini": gemini.NewOutboundTransformer, "doubao": doubao.NewOutboundTransformer, "openrouter": openrouter.NewOutboundTransformer,
		"modelscope": modelscope.NewOutboundTransformer,
	} {
		for _, input := range []string{`{"image_url":"https://example.invalid/a.png"}`, `{"file_id":"file-fixture"}`, `{"image_url":"data:image/png;base64,aW1hZ2U="}`, `{"image_url":"data:image/png;base64,aW1hZ2U="},{"file_id":"file-fixture"}`} {
			t.Run(name+"/"+input, func(t *testing.T) {
				// Given: public JSON edits, parsed once into references and decoded bytes.
				in, err := openai.NewImageEditInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Headers: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"model":"image-model","prompt":"edit","images":[` + input + `]}`)})
				require.NoError(t, err)
				out, err := constructor("https://example.invalid/v1", "test-key")
				require.NoError(t, err)
				// When
				req, err := out.TransformRequest(t.Context(), in)
				// Then: unresolved references cannot become an upstream generation request.
				if len(in.Image.Images) < len(in.Image.ImageReferences) {
					require.ErrorIs(t, err, transformer.ErrInvalidRequest)
					require.Nil(t, req)
				} else {
					require.NoError(t, err)
					require.Contains(t, string(req.Body), "aW1hZ2U=")
				}
			})
		}
	}
}

func TestImageReferencesRejectUnresolvedMask(t *testing.T) {
	for name, constructor := range map[string]func(string, string) (transformer.Outbound, error){
		"gemini": gemini.NewOutboundTransformer, "doubao": doubao.NewOutboundTransformer, "openrouter": openrouter.NewOutboundTransformer,
		"modelscope": modelscope.NewOutboundTransformer,
	} {
		for _, mask := range []string{`{"image_url":"https://example.invalid/mask.png"}`, `{"file_id":"mask-fixture"}`} {
			t.Run(name+"/"+mask, func(t *testing.T) {
				// Given
				in, err := openai.NewImageEditInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Headers: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"model":"image-model","prompt":"edit","images":[{"image_url":"data:image/png;base64,aW1hZ2U="}],"mask":` + mask + `}`)})
				require.NoError(t, err)
				out, err := constructor("https://example.invalid/v1", "test-key")
				require.NoError(t, err)
				// When
				req, err := out.TransformRequest(t.Context(), in)
				// Then
				require.ErrorIs(t, err, transformer.ErrInvalidRequest)
				require.Nil(t, req)
			})
		}
	}
}

func TestImageEditMultipartPreservesModeration(t *testing.T) {
	// Given: a public multipart edit with a non-default moderation setting.
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	require.NoError(t, form.WriteField("model", "gpt-image-2"))
	require.NoError(t, form.WriteField("prompt", "edit"))
	require.NoError(t, form.WriteField("moderation", "low"))
	part, err := form.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="image"; filename="fixture.png"`}, "Content-Type": {"image/png"}})
	require.NoError(t, err)
	_, err = part.Write([]byte("\x89PNG\r\n\x1a\nimage"))
	require.NoError(t, err)
	require.NoError(t, form.Close())
	in, err := openai.NewImageEditInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Headers: http.Header{"Content-Type": {form.FormDataContentType()}}, Body: body.Bytes()})
	require.NoError(t, err)
	out, err := openai.NewOutboundTransformer("https://example.invalid/v1", "test-key")
	require.NoError(t, err)
	// When
	req, err := out.TransformRequest(t.Context(), in)
	require.NoError(t, err)
	capture := &http.Request{Header: req.Headers, Body: http.NoBody}
	capture.Body = io.NopCloser(bytes.NewReader(req.Body))
	require.NoError(t, capture.ParseMultipartForm(1<<20))
	defer capture.MultipartForm.RemoveAll()
	// Then: both wire and request logging retain moderation.
	require.Equal(t, "low", capture.FormValue("moderation"))
	var logged struct {
		Moderation string `json:"moderation"`
	}
	require.NoError(t, json.Unmarshal(req.JSONBody, &logged))
	require.Equal(t, "low", logged.Moderation)
}

func TestImageEditJSONStyleReachesUnsupportedRejection(t *testing.T) {
	// Given: a recognized but unsupported JSON edit option through the public inbound.
	in, err := openai.NewImageEditInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Headers: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"model":"gpt-image-2","prompt":"edit","images":[{"file_id":"file-fixture"}],"style":"vivid"}`)})
	require.NoError(t, err)
	out, err := openai.NewOutboundTransformerWithConfig(&openai.Config{PlatformType: openai.PlatformOpenAI, BaseURL: "https://example.invalid/v1", APIKeyProvider: auth.NewStaticKeyProvider("test-key"), ImageEditJSON: true})
	require.NoError(t, err)
	// When
	req, err := out.TransformRequest(t.Context(), in)
	// Then
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Nil(t, req)
}
