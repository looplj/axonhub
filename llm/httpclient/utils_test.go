package httpclient

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadHTTPRequest_NoContentEncoding(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got, err := ReadHTTPRequest(req)
	require.NoError(t, err)
	assert.Equal(t, body, got.Body)
	assert.Equal(t, "", got.Headers.Get("Content-Encoding"))
}

func TestReadHTTPRequest_IdentityEncoding(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "identity")

	got, err := ReadHTTPRequest(req)
	require.NoError(t, err)
	assert.Equal(t, body, got.Body)
	assert.Equal(t, "identity", got.Headers.Get("Content-Encoding"))
}

func TestReadHTTPRequest_GzipEncoding(t *testing.T) {
	originalBody := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(originalBody)
	require.NoError(t, err)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	got, err := ReadHTTPRequest(req)
	require.NoError(t, err)
	assert.Equal(t, originalBody, got.Body)
	assert.Equal(t, "", got.Headers.Get("Content-Encoding"))
	assert.Equal(t, "", got.Headers.Get("Content-Length"))
}

func TestReadHTTPRequest_GzipEncodingXGzip(t *testing.T) {
	originalBody := []byte(`{"model":"gpt-4"}`)

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(originalBody)
	require.NoError(t, err)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "x-gzip")

	got, err := ReadHTTPRequest(req)
	require.NoError(t, err)
	assert.Equal(t, originalBody, got.Body)
	assert.Equal(t, "", got.Headers.Get("Content-Encoding"))
}

func TestReadHTTPRequest_DeflateEncoding(t *testing.T) {
	originalBody := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)

	var buf bytes.Buffer
	writer, err := flate.NewWriter(&buf, flate.DefaultCompression)
	require.NoError(t, err)
	_, err = writer.Write(originalBody)
	require.NoError(t, err)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "deflate")

	got, err := ReadHTTPRequest(req)
	require.NoError(t, err)
	assert.Equal(t, originalBody, got.Body)
	assert.Equal(t, "", got.Headers.Get("Content-Encoding"))
	assert.Equal(t, "", got.Headers.Get("Content-Length"))
}

func TestReadHTTPRequest_DeflateZlibEncoding(t *testing.T) {
	originalBody := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)

	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	_, err := writer.Write(originalBody)
	require.NoError(t, err)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "deflate")

	got, err := ReadHTTPRequest(req)
	require.NoError(t, err)
	assert.Equal(t, originalBody, got.Body)
	assert.Equal(t, "", got.Headers.Get("Content-Encoding"))
	assert.Equal(t, "", got.Headers.Get("Content-Length"))
}

func TestReadHTTPRequest_ZstdEncoding(t *testing.T) {
	originalBody := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)

	encoder, err := zstd.NewWriter(nil)
	require.NoError(t, err)
	compressedBody := encoder.EncodeAll(originalBody, nil)
	encoder.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(compressedBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")

	got, err := ReadHTTPRequest(req)
	require.NoError(t, err)
	assert.Equal(t, originalBody, got.Body)
	assert.Equal(t, "", got.Headers.Get("Content-Encoding"))
	assert.Equal(t, "", got.Headers.Get("Content-Length"))
}

func TestReadHTTPRequest_EncodingCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		compress func(t *testing.T, body []byte) []byte
	}{
		{
			name:     "gzip uppercase",
			encoding: "GZIP",
			compress: func(t *testing.T, body []byte) []byte {
				var buf bytes.Buffer
				writer := gzip.NewWriter(&buf)
				_, err := writer.Write(body)
				require.NoError(t, err)
				writer.Close()
				return buf.Bytes()
			},
		},
		{
			name:     "deflate uppercase",
			encoding: "DEFLATE",
			compress: func(t *testing.T, body []byte) []byte {
				var buf bytes.Buffer
				writer, err := flate.NewWriter(&buf, flate.DefaultCompression)
				require.NoError(t, err)
				_, err = writer.Write(body)
				require.NoError(t, err)
				writer.Close()
				return buf.Bytes()
			},
		},
		{
			name:     "zstd with spaces",
			encoding: "  ZSTD  ",
			compress: func(t *testing.T, body []byte) []byte {
				encoder, err := zstd.NewWriter(nil)
				require.NoError(t, err)
				compressed := encoder.EncodeAll(body, nil)
				encoder.Close()
				return compressed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalBody := []byte(`{"model":"gpt-4"}`)
			compressedBody := tt.compress(t, originalBody)

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(compressedBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", tt.encoding)

			got, err := ReadHTTPRequest(req)
			require.NoError(t, err)
			assert.Equal(t, originalBody, got.Body)
		})
	}
}

func TestReadHTTPRequest_UnsupportedContentEncoding(t *testing.T) {
	body := []byte(`{"model":"gpt-4"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "br")

	_, err := ReadHTTPRequest(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported content encoding")
}

func TestReadHTTPRequest_InvalidGzipData(t *testing.T) {
	invalidData := []byte("this is not valid gzip compressed data")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(invalidData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	_, err := ReadHTTPRequest(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create gzip reader")
}

func TestReadHTTPRequest_InvalidDeflateData(t *testing.T) {
	invalidData := []byte("this is not valid deflate compressed data")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(invalidData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "deflate")

	_, err := ReadHTTPRequest(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decompress deflate body")
}

func TestReadHTTPRequest_InvalidZstdData(t *testing.T) {
	invalidData := []byte("this is not valid zstd compressed data")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(invalidData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")

	_, err := ReadHTTPRequest(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode zstd compressed body")
}

func TestReadHTTPRequest_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")

	got, err := ReadHTTPRequest(req)
	require.NoError(t, err)
	assert.Empty(t, got.Body)
}

func TestReadHTTPRequest_EmptyBodyWithContentEncoding(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")

	got, err := ReadHTTPRequest(req)
	require.NoError(t, err)
	assert.Empty(t, got.Body)
}

func TestDecodeRequestBody_NoEncoding(t *testing.T) {
	body := []byte(`{"test":"data"}`)
	headers := http.Header{}

	got, err := decodeRequestBody(body, headers)
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestDecodeRequestBody_IdentityEncoding(t *testing.T) {
	body := []byte(`{"test":"data"}`)
	headers := http.Header{}
	headers.Set("Content-Encoding", "identity")

	got, err := decodeRequestBody(body, headers)
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestDecodeRequestBody_GzipEncoding(t *testing.T) {
	originalBody := []byte(`{"test":"data"}`)

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(originalBody)
	require.NoError(t, err)
	writer.Close()

	headers := http.Header{}
	headers.Set("Content-Encoding", "gzip")
	headers.Set("Content-Length", "100")

	got, err := decodeRequestBody(buf.Bytes(), headers)
	require.NoError(t, err)
	assert.Equal(t, originalBody, got)
	assert.Equal(t, "", headers.Get("Content-Encoding"))
	assert.Equal(t, "", headers.Get("Content-Length"))
}

func TestDecodeRequestBody_DeflateEncoding(t *testing.T) {
	originalBody := []byte(`{"test":"data"}`)

	var buf bytes.Buffer
	writer, err := flate.NewWriter(&buf, flate.DefaultCompression)
	require.NoError(t, err)
	_, err = writer.Write(originalBody)
	require.NoError(t, err)
	writer.Close()

	headers := http.Header{}
	headers.Set("Content-Encoding", "deflate")

	got, err := decodeRequestBody(buf.Bytes(), headers)
	require.NoError(t, err)
	assert.Equal(t, originalBody, got)
	assert.Equal(t, "", headers.Get("Content-Encoding"))
}

func TestDecodeRequestBody_ZstdEncoding(t *testing.T) {
	originalBody := []byte(`{"test":"data"}`)

	encoder, err := zstd.NewWriter(nil)
	require.NoError(t, err)
	compressedBody := encoder.EncodeAll(originalBody, nil)
	encoder.Close()

	headers := http.Header{}
	headers.Set("Content-Encoding", "zstd")

	got, err := decodeRequestBody(compressedBody, headers)
	require.NoError(t, err)
	assert.Equal(t, originalBody, got)
	assert.Equal(t, "", headers.Get("Content-Encoding"))
}

func TestMergeHTTPHeaders_AcceptNotOverridden(t *testing.T) {
	// The transformer-owned Accept (e.g. */* for TTS binary audio) must not be
	// overridden by the inbound client's Accept.
	dest := http.Header{}
	dest.Set("Accept", "*/*")
	dest.Set("Content-Type", "application/json")

	src := http.Header{}
	src.Set("Accept", "text/event-stream")
	src.Set("X-Custom", "client-value")

	merged := MergeHTTPHeaders(dest, src)
	assert.Equal(t, "*/*", merged.Get("Accept"))
	assert.Equal(t, "client-value", merged.Get("X-Custom"))
}
