package biz

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/looplj/axonhub/internal/objects"
)

// putObjectCapture records a PutObject request as received by the mock endpoint.
type putObjectCapture struct {
	method  string
	headers http.Header
	body    []byte
}

// newPutObjectCaptureServer returns an HTTPS mock S3 endpoint that captures each
// request and replies with a minimal valid PutObject response. An HTTPS (TLS)
// endpoint is essential to the regression: the SDK only switches to an
// aws-chunked trailing checksum when req.IsHTTPS(), which is why the plain-HTTP
// MinIO integration test never exercised this path.
func newPutObjectCaptureServer(t *testing.T) (*httptest.Server, func() []putObjectCapture) {
	t.Helper()

	var (
		mu       sync.Mutex
		captured []putObjectCapture
	)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		mu.Lock()
		captured = append(captured, putObjectCapture{
			method:  r.Method,
			headers: r.Header.Clone(),
			body:    body,
		})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><PutObjectOutput><ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag></PutObjectOutput>`))
	}))
	t.Cleanup(server.Close)

	return server, func() []putObjectCapture {
		mu.Lock()
		defer mu.Unlock()

		return append([]putObjectCapture(nil), captured...)
	}
}

// TestS3NoTrailingChecksumOverTLS pins the #981 regression: with the SDK default
// RequestChecksumCalculationWhenSupported, PutObject over HTTPS computes a CRC32
// and wraps the body in aws-chunked (Content-Encoding: aws-chunked,
// x-amz-content-sha256: STREAMING-UNSIGNED-PAYLOAD-TRAILER). Aliyun OSS's
// S3-compatible API accepts that request with HTTP 200 but stores a 0-byte
// object; Cloudflare R2 supports the trailer, so it worked. The client must send
// a plain signed payload instead.
func TestS3NoTrailingChecksumOverTLS(t *testing.T) {
	server, requests := newPutObjectCaptureServer(t)

	cfg := &objects.S3{
		BucketName: "axonhub-test",
		Endpoint:   server.URL,
		Region:     "us-east-1",
		AccessKey:  "test-access-key",
		SecretKey:  "test-secret-key",
		PathStyle:  true,
	}

	ctx := context.Background()

	store, err := newS3ObjectStoreWithHTTPClient(ctx, cfg, server.Client())
	if err != nil {
		t.Fatalf("newS3ObjectStoreWithHTTPClient: %v", err)
	}

	// PutObject: the byte Save path.
	payload := bytes.Repeat([]byte("a"), 32*1024)
	if err := store.PutObject(ctx, "2/requests/5/request_body.json", payload); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	assertPlainSignedPayload(t, singleRequest(t, requests()), payload)

	// PutObjectStream is the backup path that logged a correct client-side byte
	// count from countingReader while the server stored nothing; it must not use
	// an aws-chunked trailer either.
	streamPayload := bytes.Repeat([]byte("b"), 32*1024)
	// -1 mirrors SaveDataFromReader, which cannot always know the size upfront.
	n, err := store.PutObjectStream(ctx, "2/requests/5/stream.bin", bytes.NewReader(streamPayload), -1)
	if err != nil {
		t.Fatalf("PutObjectStream: %v", err)
	}

	if n != int64(len(streamPayload)) {
		t.Fatalf("PutObjectStream wrote %d bytes, want %d", n, len(streamPayload))
	}

	all := requests()
	if len(all) != 2 {
		t.Fatalf("captured %d requests, want 2", len(all))
	}

	assertPlainSignedPayload(t, all[1], streamPayload)
}

func singleRequest(t *testing.T, reqs []putObjectCapture) putObjectCapture {
	t.Helper()

	if len(reqs) != 1 {
		t.Fatalf("captured %d requests, want 1", len(reqs))
	}

	return reqs[0]
}

// assertPlainSignedPayload checks that a captured PutObject used a plain signed
// payload rather than the aws-chunked streaming trailer that OSS silently drops.
func assertPlainSignedPayload(t *testing.T, req putObjectCapture, payload []byte) {
	t.Helper()

	if req.method != http.MethodPut {
		t.Fatalf("request method = %s, want PUT", req.method)
	}

	if sha := req.headers.Get("X-Amz-Content-Sha256"); strings.Contains(sha, "STREAMING-UNSIGNED-PAYLOAD-TRAILER") {
		t.Errorf("x-amz-content-sha256 = %q, want a plain (non-streaming) payload hash", sha)
	}

	if enc := req.headers.Get("Content-Encoding"); strings.Contains(enc, "aws-chunked") {
		t.Errorf("Content-Encoding = %q, want no aws-chunked", enc)
	}

	if trailer := req.headers.Get("X-Amz-Trailer"); trailer != "" {
		t.Errorf("x-amz-trailer = %q, want empty", trailer)
	}

	if got, want := len(req.body), len(payload); got != want {
		t.Errorf("received body length = %d, want %d (aws-chunked framing adds bytes)", got, want)
	}

	if !bytes.Equal(req.body, payload) {
		t.Error("received body differs from the original payload")
	}
}
