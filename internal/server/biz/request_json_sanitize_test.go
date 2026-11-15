// 职责: 验证写入数据库的 JSON 副本可以安全存入 PostgreSQL jsonb 列。
// 关联文件:
//
//	internal/server/biz/request.go
//	internal/server/biz/request_body_chunks.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	只校验存储副本；不得把这里的行为当成发往上游请求体的规范化。
//	不含 jsonb 禁用转义的内容必须原样保留，包括合法代理对与 emoji。
//	PG 用例只在显式配置本机测试库时运行，默认跳过。

package biz

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestSanitizeJSONForStorage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		// 孤立 UTF-16 代理项转义：jsonb 会以 22P05 拒绝，替换为 U+FFFD。
		{name: "lone high surrogate in value", in: `{"a":"\ud800"}`, want: `{"a":"\ufffd"}`},
		{name: "lone low surrogate in value", in: `{"a":"\udc00"}`, want: `{"a":"\ufffd"}`},
		{name: "lone high surrogate in key", in: `{"\ud800":1}`, want: `{"\ufffd":1}`},
		{name: "lone high surrogate at array boundary", in: `["\ud800",1]`, want: `["\ufffd",1]`},
		{name: "nested value surrogate", in: `{"a":{"b":["\ud800"]}}`, want: `{"a":{"b":["\ufffd"]}}`},
		{name: "surrogate before escaped quote", in: `{"a":"\"\ud800"}`, want: `{"a":"\"\ufffd"}`},
		{name: "surrogate followed by ascii escape", in: `{"a":"\ud800\u0041"}`, want: `{"a":"\ufffd\u0041"}`},
		{name: "high surrogate followed by high then low", in: `{"a":"\ud800\ud800\udc00"}`, want: `{"a":"\ufffd\ud800\udc00"}`},
		{name: "high surrogate followed by max pair", in: `{"a":"\ud800\udbff\udfff"}`, want: `{"a":"\ufffd\udbff\udfff"}`},
		{name: "two low surrogates", in: `{"a":"\udc00\udc00"}`, want: `{"a":"\ufffd\ufffd"}`},
		{name: "nul between two surrogate halves", in: `{"a":"\ud800\u0000\udc00"}`, want: `{"a":"\ufffd\ufffd"}`},

		// 合法代理对与普通 Unicode 必须原样保留。
		{name: "valid surrogate pair", in: `{"a":"\ud83d\ude00"}`, want: `{"a":"\ud83d\ude00"}`},
		{name: "valid pair uppercase kept", in: `{"a":"\uD83D\uDE00"}`, want: `{"a":"\uD83D\uDE00"}`},
		{name: "valid max pair", in: `{"a":"\udbff\udfff"}`, want: `{"a":"\udbff\udfff"}`},
		{name: "raw emoji unchanged", in: `{"a":"😀"}`, want: `{"a":"😀"}`},
		{name: "replacement char escape unchanged", in: `{"a":"\ufffd"}`, want: `{"a":"\ufffd"}`},
		{name: "accented escape unchanged", in: `{"a":"caf\u00e9"}`, want: `{"a":"caf\u00e9"}`},

		// 反斜杠奇偶：文本形式不得被当作转义。
		{name: "escaped backslash before surrogate text", in: `{"a":"\\ud800"}`, want: `{"a":"\\ud800"}`},
		{name: "odd backslashes before surrogate", in: `{"a":"\\\ud800"}`, want: `{"a":"\\\ufffd"}`},
		{name: "even backslashes before surrogate", in: `{"a":"\\\\ud800"}`, want: `{"a":"\\\\ud800"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := string(sanitizeJSONForStorage(objects.JSONRawMessage(tt.in)))
			require.Equal(t, tt.want, got)
			requireJSONBStorable(t, []byte(got))
		})
	}
}

func TestSanitizeJSONForStorageIdempotent(t *testing.T) {
	t.Parallel()

	inputs := []string{
		`{"a":"\u0000\ud800\udc00\ud800"}`,
		`{"\ud800":{"b":"\\\udc00"}}`,
		`["\ud800","\ud83d\ude00",{"c":"\u0000"}]`,
	}

	for _, in := range inputs {
		once := sanitizeJSONForStorage(objects.JSONRawMessage(in))
		twice := sanitizeJSONForStorage(once)
		require.Equal(t, string(once), string(twice), "sanitize must be idempotent for %q", in)
		requireJSONBStorable(t, once)
	}
}

func TestSanitizeJSONForStorageInvalidInputUnchanged(t *testing.T) {
	t.Parallel()

	// 非法 JSON 不由本函数修复；只要不 panic、不越界重写。
	inputs := []string{
		`{"a":"\ud80"}`,
		`{"a":"\ud80z"}`,
		`{"a":"\u`,
		`{"a":"\`,
		`{\`,
	}

	for _, in := range inputs {
		require.NotPanics(t, func() {
			require.Equal(t, in, string(sanitizeJSONForStorage(objects.JSONRawMessage(in))), "input %q", in)
		})
	}
}

// TestMarshalStreamEventForStorageLoneSurrogate 覆盖 response_chunks：存储在
// jsonb 数组里的每个 chunk 也必须可接受，而事件原始字节必须保持不变以供回放。
func TestMarshalStreamEventForStorageLoneSurrogate(t *testing.T) {
	t.Parallel()

	data := []byte(`{"delta":"\ud800\udc00\ud800"}`)
	original := bytes.Clone(data)
	chunk := &httpclient.StreamEvent{Type: "message", Data: data}

	raw, err := marshalStreamEventForStorage(chunk)
	require.NoError(t, err)
	require.Error(t, jsonbStorable(raw), "the marshaled event keeps the lone surrogate before sanitizing")

	stored := sanitizeJSONForStorage(raw)
	require.NoError(t, jsonbStorable(stored))
	require.Contains(t, string(stored), `\ud800\udc00`, "valid pair must survive")
	require.Contains(t, string(stored), `\ufffd`)

	require.Equal(t, original, chunk.Data, "sanitizing the stored copy must not mutate the event data")
	require.Equal(t, original, data)
}

// TestSanitizeJSONForStoragePostgresJSONB 是可选的本地验收：显式配置
// AXONHUB_TEST_PAYLOAD_PG_DSN 时才连接本机测试库，默认跳过。
func TestSanitizeJSONForStoragePostgresJSONB(t *testing.T) {
	dsn := os.Getenv("AXONHUB_TEST_PAYLOAD_PG_DSN")
	if dsn == "" {
		t.Skip("AXONHUB_TEST_PAYLOAD_PG_DSN is not configured")
	}

	endpoint, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, endpoint.Hostname(),
		"optional fixture only connects to a local database")

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx := t.Context()
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE jsonb_surrogate_probe (body jsonb)`)
	require.NoError(t, err)

	raw := `{"a":"\ud800"}`
	_, err = db.ExecContext(ctx, `INSERT INTO jsonb_surrogate_probe (body) VALUES ($1::jsonb)`, raw)
	require.Error(t, err, "raw lone surrogate must be rejected by jsonb")

	_, err = db.ExecContext(ctx, `INSERT INTO jsonb_surrogate_probe (body) VALUES ($1::jsonb)`,
		string(sanitizeJSONForStorage(objects.JSONRawMessage(raw))))
	require.NoError(t, err)
}

// requireJSONBStorable 校验存储副本满足 PostgreSQL jsonb 的转义规则。
func requireJSONBStorable(t *testing.T, body []byte) {
	t.Helper()

	require.NoError(t, jsonbStorable(body), "stored copy must be jsonb-safe: %s", body)
}

// jsonbStorable 按 jsonb 的规则独立检查：语法合法、无 U+0000 转义、
// 无未成对 UTF-16 代理项转义。
func jsonbStorable(body []byte) error {
	if !json.Valid(body) {
		return fmt.Errorf("invalid JSON")
	}

	for i := 0; i < len(body); {
		if body[i] != '\\' {
			i++

			continue
		}

		if i+1 >= len(body) {
			return fmt.Errorf("dangling backslash")
		}

		if body[i+1] != 'u' {
			i += 2

			continue
		}

		if i+6 > len(body) {
			return fmt.Errorf("truncated unicode escape")
		}

		value, ok := jsonHex4(body[i+2 : i+6])
		if !ok {
			return fmt.Errorf("invalid unicode escape")
		}

		if value == 0 {
			return fmt.Errorf("NUL escape must not reach jsonb")
		}

		if isJSONHighSurrogate(value) {
			if i+12 > len(body) || body[i+6] != '\\' || body[i+7] != 'u' {
				return fmt.Errorf("high surrogate without a following escape")
			}

			low, ok := jsonHex4(body[i+8 : i+12])
			if !ok || !isJSONLowSurrogate(low) {
				return fmt.Errorf("high surrogate must be followed by a low surrogate")
			}

			i += 12

			continue
		}

		if isJSONLowSurrogate(value) {
			return fmt.Errorf("lone low surrogate must not reach jsonb")
		}

		i += 6
	}

	return nil
}
