package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingTokenGetter struct{}

func (failingTokenGetter) Get(context.Context) (*oauth.OAuthCredentials, error) {
	return nil, errors.New("refresh failed")
}

func TestCodexImageEditJSON(t *testing.T) {
	// Given: the official JSON image-reference schema, including file IDs and a mask.
	raw := &httpclient.Request{Headers: http.Header{"Content-Type": {"application/json"}, "Session-Id": {"image-session"}}, Body: []byte(`{"model":"gpt-image-2","prompt":"make blue","images":[{"image_url":"https://example.com/image.png"},{"file_id":"file-fixture"}],"mask":{"file_id":"mask-fixture"},"n":2,"quality":"high","input_fidelity":"high","size":"1024x1024","user":"fixture-user","output_format":"webp","output_compression":0,"moderation":"low","background":"transparent"}`)}
	in := openai.NewImageEditInboundTransformer()
	out, err := NewImageOutboundTransformer(ImageParams{BaseURL: "https://relay.example/backend#", TokenProvider: oauth.NewAPIKeyTokenProvider(auth.NewStaticKeyProvider("image-token").Get)})
	require.NoError(t, err)
	// When: a public JSON edit is translated to the Codex outbound.
	req, err := in.TransformRequest(t.Context(), raw)
	require.NoError(t, err)
	hreq, err := out.TransformRequest(t.Context(), req)
	// Then: all machine-consumed fields survive, with no Responses tool payload.
	require.NoError(t, err)
	require.Equal(t, "https://relay.example/backend/images/edits", hreq.URL)
	require.Equal(t, "application/json", hreq.Headers.Get("Content-Type"))
	require.Equal(t, "application/json", hreq.Headers.Get("Accept"))
	require.Equal(t, "image-token", hreq.Auth.APIKey)
	require.Equal(t, "image-session", hreq.Headers.Get("Session-Id"))
	var got, want map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(hreq.Body, &got))
	require.NoError(t, json.Unmarshal(raw.Body, &want))
	require.Equal(t, want, got)
	require.Equal(t, llm.APIFormatOpenAIImageEdit.String(), hreq.APIFormat)
}

func TestCodexImageEditJSONInvalidReferences(t *testing.T) {
	for _, body := range []string{
		`{"prompt":"edit","images":[{}]}`, `{"prompt":"edit","images":[{"image_url":"https://example.com/a.png","file_id":"file-a"}]}`,
		`{"prompt":"edit","images":[{"image_url":" "}]}`, `{"prompt":"edit","images":[{"file_id":"file-a","unknown":1}]}`,
		`{"prompt":"edit","images":[]}`, `{"prompt":"edit","images":[{"file_id":"file-a"}],"mask":{}}`,
	} {
		t.Run(body, func(t *testing.T) {
			// Given / When
			_, err := openai.NewImageEditInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Headers: http.Header{"Content-Type": {"application/json"}}, Body: []byte(body)})
			// Then
			require.Error(t, err)
		})
	}
}

func TestCodexImagesRejectStreamingBeforeTokenRefresh(t *testing.T) {
	out, err := NewImageOutboundTransformer(ImageParams{TokenProvider: failingTokenGetter{}, BaseURL: "https://relay.example/v1"})
	require.NoError(t, err)
	stream := true
	_, err = out.TransformRequest(t.Context(), &llm.Request{
		Model: "gpt-image-2", RequestType: llm.RequestTypeImage, APIFormat: llm.APIFormatOpenAIImageGeneration,
		Stream: &stream, Image: &llm.ImageRequest{Prompt: "draw"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, transformer.ErrInvalidRequest)
	assert.NotContains(t, err.Error(), "refresh failed")
}

func TestCodexImageMultipartJSONDriverCapturesURLAuthImageMaskAnd403(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"forbidden"}}`))
	}))
	defer server.Close()

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	require.NoError(t, form.WriteField("model", "gpt-image-2"))
	require.NoError(t, form.WriteField("prompt", "make blue"))
	imagePart, err := form.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="image"; filename="input.png"`}, "Content-Type": {"image/png"}})
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("\x89PNG image"))
	require.NoError(t, err)
	maskPart, err := form.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="mask"; filename="mask.png"`}, "Content-Type": {"image/png"}})
	require.NoError(t, err)
	_, err = maskPart.Write([]byte("\x89PNG mask"))
	require.NoError(t, err)
	require.NoError(t, form.Close())

	in := openai.NewImageEditInboundTransformer()
	llmReq, err := in.TransformRequest(t.Context(), &httpclient.Request{Method: http.MethodPost, Headers: http.Header{"Content-Type": {form.FormDataContentType()}}, Body: body.Bytes()})
	require.NoError(t, err)
	out, err := NewImageOutboundTransformer(ImageParams{TokenProvider: oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{AccessToken: "image-token"}), BaseURL: server.URL})
	require.NoError(t, err)
	hreq, err := out.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)
	httpReq, err := http.NewRequestWithContext(t.Context(), hreq.Method, hreq.URL, bytes.NewReader(hreq.Body))
	require.NoError(t, err)
	for key, values := range hreq.Headers {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}
	httpReq.Header.Set("Authorization", "Bearer "+hreq.Auth.APIKey)
	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, "/v1/images/edits", gotPath)
	require.Equal(t, "Bearer image-token", gotAuth)
	require.Contains(t, string(gotBody), `"images":[{"image_url":"data:image/png;base64,`)
	require.Contains(t, string(gotBody), `"mask":{"image_url":"data:image/png;base64,`)
}
