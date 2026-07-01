package biz

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3fs "github.com/looplj/afero-s3"

	"github.com/looplj/axonhub/internal/objects"
)

// TestS3ObjectStoreIntegration exercises the native S3 ObjectStore against a real
// S3-compatible endpoint (e.g. MinIO). It is skipped unless AXONHUB_TEST_S3_ENDPOINT
// is set, so it never runs during a normal `go test ./...`.
//
//	docker run -d --rm --name minio -p 9000:9000 \
//	  -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
//	  minio/minio server /data
//	AXONHUB_TEST_S3_ENDPOINT=http://localhost:9000 go test ./internal/server/biz/ -run TestS3ObjectStoreIntegration -v
func TestS3ObjectStoreIntegration(t *testing.T) {
	cfg := s3TestConfig(t)
	ctx := context.Background()

	ensureBucket(ctx, t, cfg)

	store, err := newS3ObjectStore(ctx, cfg)
	if err != nil {
		t.Fatalf("newS3ObjectStore: %v", err)
	}

	t.Run("put/get round trip", func(t *testing.T) {
		key := "2/requests/5/request_body.json"
		data := []byte(`{"hello":"world"}`)

		if err := store.PutObject(ctx, key, data); err != nil {
			t.Fatalf("PutObject: %v", err)
		}

		got, err := store.GetObject(ctx, key)
		if err != nil {
			t.Fatalf("GetObject: %v", err)
		}

		if !bytes.Equal(got, data) {
			t.Fatalf("GetObject = %q, want %q", got, data)
		}
	})

	t.Run("get missing key maps to os.ErrNotExist", func(t *testing.T) {
		_, err := store.GetObject(ctx, "2/requests/5/missing.json")
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("GetObject(missing) error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("delete is idempotent (missing key + dir marker)", func(t *testing.T) {
		key := "2/requests/5/to-delete.json"

		if err := store.PutObject(ctx, key, []byte("x")); err != nil {
			t.Fatalf("PutObject: %v", err)
		}

		if err := store.DeleteObject(ctx, key); err != nil {
			t.Fatalf("DeleteObject: %v", err)
		}
		// Deleting again (now missing) must be a no-op, no error, no List.
		if err := store.DeleteObject(ctx, key); err != nil {
			t.Fatalf("DeleteObject(missing) = %v, want nil", err)
		}
		// Deleting a never-created directory-marker key must be a no-op.
		if err := store.DeleteObject(ctx, "2/requests/5"); err != nil {
			t.Fatalf("DeleteObject(dir marker) = %v, want nil", err)
		}
		// Confirm the object is actually gone.
		if _, err := store.GetObject(ctx, key); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("GetObject after delete = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("stream round trip returns byte count", func(t *testing.T) {
		key := "2/requests/5/stream.bin"
		payload := bytes.Repeat([]byte("a"), 2<<20) // 2 MiB (single-part)

		n, err := store.PutObjectStream(ctx, key, bytes.NewReader(payload), -1)
		if err != nil {
			t.Fatalf("PutObjectStream: %v", err)
		}

		if n != int64(len(payload)) {
			t.Fatalf("PutObjectStream wrote %d bytes, want %d", n, len(payload))
		}

		got, err := store.GetObject(ctx, key)
		if err != nil {
			t.Fatalf("GetObject: %v", err)
		}

		if !bytes.Equal(got, payload) {
			t.Fatalf("stream payload mismatch (got %d bytes)", len(got))
		}
	})

	t.Run("stream multipart (> part size) round trip", func(t *testing.T) {
		key := "2/requests/5/big.bin"
		payload := bytes.Repeat([]byte("b"), s3UploadPartSize+(1<<20)) // > 16 MiB -> multipart

		n, err := store.PutObjectStream(ctx, key, bytes.NewReader(payload), -1)
		if err != nil {
			t.Fatalf("PutObjectStream(multipart): %v", err)
		}

		if n != int64(len(payload)) {
			t.Fatalf("PutObjectStream wrote %d bytes, want %d", n, len(payload))
		}

		rc, size, err := store.OpenObject(ctx, key)
		if err != nil {
			t.Fatalf("OpenObject: %v", err)
		}
		defer rc.Close()

		if size != int64(len(payload)) {
			t.Fatalf("OpenObject size = %d, want %d", size, len(payload))
		}

		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read OpenObject: %v", err)
		}

		if !bytes.Equal(got, payload) {
			t.Fatalf("multipart payload mismatch (got %d bytes)", len(got))
		}
	})

	t.Run("object written via old afero path is readable via native store", func(t *testing.T) {
		client, err := newS3Client(ctx, cfg)
		if err != nil {
			t.Fatalf("newS3Client: %v", err)
		}

		afs := s3fs.NewFsFromClient(cfg.BucketName, client)

		// The pre-refactor path passed leading-slash keys; afero-s3 sanitized
		// them to no-leading-slash. The native store must hit the same object
		// via normalizeObjectKey.
		oldKey := "/2/requests/9/response_body.json"
		data := []byte(`{"compat":true}`)

		if err := afero.WriteFile(afs, oldKey, data, 0o777); err != nil {
			t.Fatalf("afero.WriteFile: %v", err)
		}

		got, err := store.GetObject(ctx, normalizeObjectKey(oldKey))
		if err != nil {
			t.Fatalf("native GetObject of afero-written key: %v", err)
		}

		if !bytes.Equal(got, data) {
			t.Fatalf("backward-compat read mismatch: got %q, want %q", got, data)
		}
	})
}

func s3TestConfig(t *testing.T) *objects.S3 {
	t.Helper()

	endpoint := os.Getenv("AXONHUB_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("AXONHUB_TEST_S3_ENDPOINT not set; skipping S3 integration test")
	}

	return &objects.S3{
		BucketName: getenvDefault("AXONHUB_TEST_S3_BUCKET", "axonhub-test"),
		Endpoint:   endpoint,
		Region:     getenvDefault("AXONHUB_TEST_S3_REGION", "us-east-1"),
		AccessKey:  getenvDefault("AXONHUB_TEST_S3_ACCESS_KEY", "minioadmin"),
		SecretKey:  getenvDefault("AXONHUB_TEST_S3_SECRET_KEY", "minioadmin"),
		PathStyle:  true,
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return def
}

func ensureBucket(ctx context.Context, t *testing.T, cfg *objects.S3) {
	t.Helper()

	client, err := newS3Client(ctx, cfg)
	if err != nil {
		t.Fatalf("newS3Client: %v", err)
	}

	_, err = client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: &cfg.BucketName})
	if err != nil &&
		!strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") &&
		!strings.Contains(err.Error(), "BucketAlreadyExists") {
		t.Logf("CreateBucket (continuing, may already exist): %v", err)
	}
}
