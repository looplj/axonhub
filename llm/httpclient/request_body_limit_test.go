package httpclient

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

// TestInitMaxRequestBodySize 验证 MB 换算和错误配置兜底，防止乘法溢出绕过读取上限。
func TestInitMaxRequestBodySize(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int64
	}{
		{"", DefaultMaxRequestBodySize},
		{" ", DefaultMaxRequestBodySize},
		{"1", 1 << 20},
		{"64", 64 << 20},
		{"128", 128 << 20},
		{" 256 ", 256 << 20},
		{"0", DefaultMaxRequestBodySize},
		{"-1", DefaultMaxRequestBodySize},
		{"1.5", DefaultMaxRequestBodySize},
		{"128MB", DefaultMaxRequestBodySize},
		{"invalid", DefaultMaxRequestBodySize},
		{strconv.FormatInt(math.MaxInt64, 10), DefaultMaxRequestBodySize},
		{strconv.FormatInt((math.MaxInt64-1)/(1<<20)+1, 10), DefaultMaxRequestBodySize},
		{"9223372036854775808", DefaultMaxRequestBodySize},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("AXONHUB_MAX_REQUEST_BODY_SIZE_MB", tc.value)
			require.Equal(t, tc.want, initMaxRequestBodySize())
		})
	}
}

// TestReadHTTPRequest_ConfiguredLimit 覆盖原始体、所有压缩编码和恰好等于上限的边界。
func TestReadHTTPRequest_ConfiguredLimit(t *testing.T) {
	for _, encoding := range []string{"", "gzip", "deflate", "raw-deflate", "zstd"} {
		for _, size := range []int{4095, 4096, 4097} {
			t.Run(encoding+"/"+strconv.Itoa(size), func(t *testing.T) {
				t.Parallel()
				body := bytes.Repeat([]byte("a"), size)
				encoded := body
				if encoding != "" {
					var buf bytes.Buffer
					var writer io.WriteCloser
					var err error
					switch encoding {
					case "gzip":
						writer = gzip.NewWriter(&buf)
					case "deflate":
						writer = zlib.NewWriter(&buf)
					case "raw-deflate":
						writer, err = flate.NewWriter(&buf, flate.DefaultCompression)
					case "zstd":
						writer, err = zstd.NewWriter(&buf)
					}
					require.NoError(t, err)
					_, err = writer.Write(body)
					require.NoError(t, err)
					require.NoError(t, writer.Close())
					encoded = buf.Bytes()
				}
				req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(encoded))
				header := encoding
				if encoding == "raw-deflate" {
					header = "deflate"
				}
				req.Header.Set("Content-Encoding", header)
				req = req.WithContext(WithRequestBodyLimit(req.Context(), 4096))
				got, err := ReadHTTPRequest(req)
				if size > 4096 {
					require.ErrorIs(t, err, ErrRequestBodyTooLarge)
					return
				}
				require.NoError(t, err)
				require.Equal(t, body, got.Body)
			})
		}
	}
}

// TestRequestBodyLimitIsolation 确认每个请求独立持有上限，避免不同调用修改进程级配置产生数据竞争。
func TestRequestBodyLimitIsolation(t *testing.T) {
	for _, limit := range []int64{64, 8192} {
		t.Run(strconv.FormatInt(limit, 10), func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(bytes.Repeat([]byte("a"), 4096)))
			req = req.WithContext(WithRequestBodyLimit(req.Context(), limit))
			_, err := ReadHTTPRequest(req)
			if limit < 4096 {
				require.ErrorIs(t, err, ErrRequestBodyTooLarge)
			} else {
				require.NoError(t, err)
			}
		})
	}
	require.Equal(t, maxRequestBodySize, RequestBodyLimit(context.Background()))
	for _, invalid := range []int64{0, -1, math.MaxInt64} {
		require.Equal(t, maxRequestBodySize, RequestBodyLimit(WithRequestBodyLimit(context.Background(), invalid)))
	}
}
