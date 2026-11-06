package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRequestStreams(t *testing.T) {
	for _, tt := range []struct {
		name, path, body string
		stream           bool
	}{
		{"chat", "/v1/chat/completions", `{"stream":true}`, true},
		{"completion", "/v1/completions", `{"stream":true}`, true},
		{"responses", "/v1/responses", `{"stream":true}`, true},
		{"codex", "/codex/responses", `{"stream":true}`, true},
		{"anthropic", "/anthropic/v1/messages", `{"stream":true}`, true},
		{"nonstream", "/v1/chat/completions", `{"stream":false}`, false},
		{"default", "/v1/responses", `{}`, false},
		{"gemini action", "/gemini/v1beta/models/model:streamGenerateContent", `{}`, true},
		{"gemini base path", "/custom/gemini/v1beta/models/model:streamGenerateContent", `{}`, true},
		{"gemini alias", "/v1beta/models/model:streamGenerateContent", `{}`, true},
		{"gemini ignores body stream", "/v1beta/models/model:generateContent", `{"stream":true}`, false},
		{"speech binary", "/v1/audio/speech", `{"stream_format":"audio"}`, true},
		{"speech sse", "/v1/audio/speech", `{"stream_format":"sse"}`, true},
		{"speech default", "/v1/audio/speech", `{"stream":true}`, false},
		{"images reject streaming", "/v1/images/generations", `{"stream":true}`, false},
		{"embeddings", "/v1/embeddings", `{"stream":true}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.stream, requestStreams(&httpclient.Request{Path: tt.path, Body: []byte(tt.body)}))
		})
	}
}

func TestRequestStreamsMultipartAudio(t *testing.T) {
	for _, path := range []string{"/v1/audio/transcriptions", "/v1/audio/translations"} {
		for _, field := range []string{"true", "yes", "1", "false", ""} {
			t.Run(path+"/"+field, func(t *testing.T) {
				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				file, err := writer.CreateFormFile("file", "audio.wav")
				require.NoError(t, err)
				_, err = file.Write([]byte("audio"))
				require.NoError(t, err)
				require.NoError(t, writer.WriteField("stream", field))
				require.NoError(t, writer.Close())
				request := &httpclient.Request{Path: path, Headers: http.Header{"Content-Type": {writer.FormDataContentType()}}, Body: body.Bytes()}
				require.Equal(t, field == "true" || field == "yes" || field == "1", requestStreams(request))
			})
		}
	}
}

func TestVideoHandlersSelectNonstreamTimeout(t *testing.T) {
	for _, handler := range []struct {
		name   string
		handle gin.HandlerFunc
	}{
		{"openai", (&OpenAIHandlers{}).CreateVideo},
		{"doubao", (&DoubaoHandlers{}).CreateTask},
	} {
		for _, tt := range []struct {
			name            string
			request, stream time.Duration
		}{
			{"finite", time.Second, 3 * time.Second},
			{"unlimited", 0, time.Second},
			{"finite with unlimited stream", time.Second, 0},
		} {
			t.Run(handler.name+"/"+tt.name, func(t *testing.T) {
				router := gin.New()
				router.Use(middleware.WithLLMTimeout(tt.request, tt.stream))
				router.POST("/", func(c *gin.Context) {
					// Empty bodies return before touching the orchestrator. The handler
					// must still select its own budget rather than retaining the provisional one.
					handler.handle(c)
					deadline, ok := c.Request.Context().Deadline()
					require.Equal(t, tt.request != 0, ok)
					if ok {
						require.WithinDuration(t, time.Now().Add(tt.request), deadline, 100*time.Millisecond)
					}
				})
				result := httptest.NewRecorder()
				router.ServeHTTP(result, httptest.NewRequest(http.MethodPost, "/", nil))
				require.Equal(t, http.StatusBadRequest, result.Code)
			})
		}
	}
}
