package biz

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	"github.com/looplj/axonhub/internal/objects"
)

// s3UploadPartSize is the multipart part size used for streaming uploads. The
// afero-s3 adapter left this at the SDK default (5MB), so a 512MB video stream
// fragmented into ~100 UploadPart (Class A) calls. A larger part size keeps the
// part count (and Class A op count) low for big payloads while small payloads
// still upload as a single PutObject.
const s3UploadPartSize = 16 * 1024 * 1024 // 16 MiB

// s3ObjectStore is the native (non-afero) ObjectStore implementation for S3 and
// S3-compatible stores. It issues exactly one S3 call per logical operation.
type s3ObjectStore struct {
	client   *awss3.Client
	uploader *manager.Uploader
	bucket   string
}

func newS3ObjectStore(ctx context.Context, cfg *objects.S3) (*s3ObjectStore, error) {
	client, err := newS3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}

	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = s3UploadPartSize
	})

	return &s3ObjectStore{
		client:   client,
		uploader: uploader,
		bucket:   cfg.BucketName,
	}, nil
}

func (o *s3ObjectStore) PutObject(ctx context.Context, key string, data []byte) error {
	_, err := o.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:        aws.String(o.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return fmt.Errorf("s3 put object %q: %w", key, err)
	}

	return nil
}

func (o *s3ObjectStore) PutObjectStream(ctx context.Context, key string, r io.Reader, _ int64) (int64, error) {
	// Count bytes as they flow through the uploader; manager.Uploader does not
	// report the written size directly.
	cr := &countingReader{r: r}

	_, err := o.uploader.Upload(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(key),
		Body:   cr,
	})
	if err != nil {
		return 0, fmt.Errorf("s3 upload %q: %w", key, err)
	}

	return cr.n, nil
}

func (o *s3ObjectStore) GetObject(ctx context.Context, key string) ([]byte, error) {
	out, err := o.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, fmt.Errorf("%w: %s", os.ErrNotExist, key)
		}

		return nil, fmt.Errorf("s3 get object %q: %w", key, err)
	}
	defer out.Body.Close()

	return io.ReadAll(out.Body)
}

func (o *s3ObjectStore) OpenObject(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	out, err := o.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, 0, fmt.Errorf("%w: %s", os.ErrNotExist, key)
		}

		return nil, 0, fmt.Errorf("s3 open object %q: %w", key, err)
	}

	return out.Body, aws.ToInt64(out.ContentLength), nil
}

func (o *s3ObjectStore) DeleteObject(ctx context.Context, key string) error {
	// DeleteObject is idempotent: deleting a missing key returns success (204),
	// so we never pre-Stat and never escalate to a ListObjectsV2.
	_, err := o.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete object %q: %w", key, err)
	}

	return nil
}

// isS3NotFound reports whether err represents a missing S3 object. GetObject
// returns *types.NoSuchKey; HeadObject returns *types.NotFound; S3-compatible
// stores may surface the code via a generic smithy.APIError.
func isS3NotFound(err error) bool {
	if err == nil {
		return false
	}

	var noSuchKey *s3types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	var notFound *s3types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}

	return false
}

// countingReader counts the number of bytes read through it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)

	return n, err
}
