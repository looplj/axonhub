// 职责: 验证 input 尾部分块的原始字节保真、回落边界和连续追加成本。
// 关联文件:
//   internal/server/biz/request_body_chunks.go
//   internal/server/biz/request_body_chunks_test.go
// 注意事项: 原 CDC 对照固定旧分块策略；新增 payload 统计不包含初始历史。

package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
)

// legacyRequestBodyCDC retains the original whole-body policy as a fallback and cost oracle.
func legacyRequestBodyCDC(body []byte) [][]byte {
	var chunks [][]byte
	start := 0
	var fingerprint uint64
	for i, value := range body {
		fingerprint = fingerprint<<1 + requestBodyGear[value]
		size := i + 1 - start
		if size >= requestBodyChunkMax || size >= requestBodyChunkMin && fingerprint&requestBodyChunkMask == 0 {
			chunks = append(chunks, body[start:i+1])
			start = i + 1
			fingerprint = 0
		}
	}
	if start < len(body) {
		chunks = append(chunks, body[start:])
	}
	return chunks
}

func TestPrepareChunkedRequestBodyInputTailPreservesRawBytes(t *testing.T) {
	padding := strings.Repeat("p", requestBodyChunkThreshold)
	cases := map[string]string{
		"numbers whitespace and unknown fields": fmt.Sprintf(" \n{\n\t\"unknown\": {\"keep\":true}, \"input\" : [9007199254740993, -0, 1.000000000000000000001, 1e9999, true, false, null, [], {}], \"padding\":\"%s\"\n}\t", padding),
		"escaped input key and nested quotes":   fmt.Sprintf(`{"\u0069nput":["escaped \"quote\" and \\ slash, ] }",{"nested":[{"input":["nested"]},[]]}],"padding":"%s"}`, padding),
		"empty array":                           fmt.Sprintf("{\"padding\":\"%s\",\"input\":[ \n\t ]}", padding),
		"input last":                            fmt.Sprintf(`{"padding":"%s","unknown":[{},false],"input":[{},[],"",null]}`, padding),
		"quoted and nested input ignored":       fmt.Sprintf(`{"nested":{"input":"ignored"},"quoted":"\"input\":[not structural]","input":["%s",{"nested":["[", "]", "\\", "\""]}],"unknown":123}`, padding),
	}
	for _, size := range []int{1023, 1024, 1025, requestBodyChunkMin - 1, requestBodyChunkMin, requestBodyChunkMin + 1} {
		cases[fmt.Sprintf("item boundary %d", size)] = fmt.Sprintf(`{"input":["%s",null,{}],"padding":"%s"}`, strings.Repeat("x", size-2), padding)
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			body := []byte(raw)
			require.True(t, json.Valid(body))
			_, ok := requestBodyInputArray(body)
			require.True(t, ok)
			plan := PrepareChunkedRequestBody(body)
			require.NotNil(t, plan)
			require.Equal(t, body, bytes.Join(plan.chunks, nil))
			for _, chunk := range plan.chunks {
				require.NotEmpty(t, chunk)
				require.LessOrEqual(t, len(chunk), requestBodyChunkMax)
			}
			var manifest requestBodyManifest
			require.NoError(t, json.Unmarshal(plan.Manifest, &manifest))
			require.Equal(t, len(body), manifest.Size)
			require.Equal(t, requestBodyHash(body), manifest.SHA256)
			require.Equal(t, len(plan.chunks), manifest.Chunks)
			require.Equal(t, plan.Manifest, PrepareChunkedRequestBody(body).Manifest)
		})
	}
}

func TestPrepareChunkedRequestBodyInputTailFallsBackToLegacyCDC(t *testing.T) {
	padding := strings.Repeat("q", requestBodyChunkThreshold)
	cases := map[string]string{
		"duplicate input":         fmt.Sprintf(`{"input":[1],"input":[2],"padding":"%s"}`, padding),
		"escaped duplicate input": fmt.Sprintf(`{"input":[],"\u0069nput":[],"padding":"%s"}`, padding),
		"string input":            fmt.Sprintf(`{"input":"[%s]"}`, padding),
		"object input":            fmt.Sprintf(`{"input":{"nested":[]},"padding":"%s"}`, padding),
		"null input":              fmt.Sprintf(`{"input":null,"padding":"%s"}`, padding),
		"missing input":           fmt.Sprintf(`{"nested":{"input":[]},"padding":"%s"}`, padding),
		"empty object":            `{}` + strings.Repeat(" ", requestBodyChunkThreshold),
		"root array":              fmt.Sprintf(`[{"input":[]},"%s"]`, padding),
		"root string":             `"` + padding + `"`,
		"root primitive":          `null` + strings.Repeat(" ", requestBodyChunkThreshold),
		"only whitespace":         strings.Repeat(" \n\t", requestBodyChunkThreshold/3+1),
		"unterminated object":     fmt.Sprintf(`{"input":[],"padding":"%s"`, padding),
		"invalid trailing data":   fmt.Sprintf(`{"input":[],"padding":"%s"} trailing`, padding),
		"invalid array":           fmt.Sprintf(`{"input":[1,],"padding":"%s"}`, padding),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			body := []byte(raw)
			_, ok := requestBodyInputArray(body)
			require.False(t, ok)
			plan := PrepareChunkedRequestBody(body)
			require.NotNil(t, plan)
			require.Equal(t, body, bytes.Join(plan.chunks, nil))
			require.Equal(t, legacyRequestBodyCDC(body), plan.chunks)
		})
	}
}

func TestPrepareChunkedRequestBodyInputTailClosesAtMinimum(t *testing.T) {
	for _, size := range []int{1023, 1024, 1025} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			first := `"` + strings.Repeat("x", size-2) + `"`
			body := []byte(`{"input":[` + first + `,{}],"padding":"` + strings.Repeat("p", requestBodyChunkThreshold) + `"}`)
			plan := PrepareChunkedRequestBody(body)
			require.Equal(t, body, bytes.Join(plan.chunks, nil))
			want := first
			if size < 1024 {
				want += `,{}`
			}
			require.Greater(t, len(plan.chunks), 1)
			require.Equal(t, []byte(want), plan.chunks[1], "seal at the first item end reaching 1 KiB, before its following comma")
		})
	}
}

func TestPrepareChunkedRequestBodyInputTailBoundsAppendCostAndReferences(t *testing.T) {
	const seedItems, appendSteps, itemSize = 512, 32, 512
	const prefix, suffix = `{"role":"user","content":"`, `"}`
	contentSize := itemSize - len(prefix) - len(suffix)
	var seedBody struct {
		Input []struct{ Output string }
	}
	require.NoError(t, json.Unmarshal(chunkTestBody(), &seedBody))
	seedText := seedBody.Input[0].Output
	items := make([]string, seedItems, seedItems+appendSteps)
	for i := range items {
		items[i] = prefix + seedText[i*contentSize:(i+1)*contentSize] + suffix
	}
	random := rand.New(rand.NewSource(73))
	seen, legacySeen := make(map[string]bool), make(map[string]bool)
	newPayload, legacyPayload, appendedBytes := 0, 0, 0
	var seed, final *ChunkedRequestBody
	var previousHistorySize int
	for step := 0; step <= appendSteps; step++ {
		if step > 0 {
			text := make([]byte, contentSize)
			for i := range text {
				text[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[random.Intn(36)]
			}
			items = append(items, prefix+string(text)+suffix)
		}
		history := strings.Join(items, ",\n")
		body := []byte(fmt.Sprintf(" {\"before\":\"request-%04d\", \"input\" : [%s], \"after\":\"response-%04d\", \"model\":\"gpt-6\"} \n", step, history, step))
		plan := PrepareChunkedRequestBody(body)
		require.Equal(t, body, bytes.Join(plan.chunks, nil))
		for _, chunk := range plan.chunks {
			hash := requestBodyHash(chunk)
			if !seen[hash] && step > 0 {
				newPayload += len(chunk)
			}
			seen[hash] = true
		}
		for _, chunk := range legacyRequestBodyCDC(body) {
			hash := requestBodyHash(chunk)
			if !legacySeen[hash] && step > 0 {
				legacyPayload += len(chunk)
			}
			legacySeen[hash] = true
		}
		if step == 0 {
			seed = plan
		} else {
			appendedBytes += len(history) - previousHistorySize
		}
		previousHistorySize = len(history)
		final = plan
	}
	t.Logf("excluding seed: appended=%d tail_payload=%d (%.3fx) legacy_payload=%d (%.3fx), final_refs=%d items=%d", appendedBytes, newPayload, float64(newPayload)/float64(appendedBytes), legacyPayload, float64(legacyPayload)/float64(appendedBytes), len(final.chunks), len(items))
	require.Less(t, newPayload*2, legacyPayload, "small strict appends must materially reduce new payload, including changing envelope fields")
	require.Less(t, len(final.chunks), len(items)/8, "sealed history must retain coarse CDC chunks rather than one reference per item")
	finalHashes := make(map[string]bool)
	for _, chunk := range final.chunks {
		finalHashes[requestBodyHash(chunk)] = true
	}
	coarseChunks := 0
	for _, chunk := range seed.chunks {
		if len(chunk) >= requestBodyChunkMin {
			coarseChunks++
			require.True(t, finalHashes[requestBodyHash(chunk)], "appending must preserve each sealed coarse history block")
		}
	}
	require.Positive(t, coarseChunks)
}

func TestPrepareChunkedRequestBodyInputTailKeepsMaximumAndRoundTrips(t *testing.T) {
	cases := map[string][]int{
		"single oversized item":          {3*requestBodyChunkMax + 8192},
		"ends at forced coarse boundary": {2*requestBodyChunkMax - len(`{"content":""}`)},
		"items across boundaries": {1023, 1024, 1025, requestBodyChunkMin - 1, requestBodyChunkMin, requestBodyChunkMax - 1,
			requestBodyChunkMax, requestBodyChunkMax + 1},
	}
	for name, sizes := range cases {
		t.Run(name, func(t *testing.T) {
			items := make([]string, len(sizes))
			for i, size := range sizes {
				items[i] = `{"content":"` + strings.Repeat("x", size) + `"}`
			}
			body := []byte(`{"input":[` + strings.Join(items, ",") + `],"after":"changed envelope"}`)
			plan := PrepareChunkedRequestBody(body)
			require.Equal(t, body, bytes.Join(plan.chunks, nil))
			forced := 0
			for _, chunk := range plan.chunks {
				require.NotEmpty(t, chunk)
				require.LessOrEqual(t, len(chunk), requestBodyChunkMax)
				if len(chunk) == requestBodyChunkMax {
					forced++
				}
			}
			require.Positive(t, forced, "repeated content must exercise forced 64 KiB boundaries")
			client, ctx, projectID := newChunkTestClient(t)
			owner := saveChunkTestRequest(t, client, ctx, projectID, body)
			loaded, err := LoadChunkedRequestBody(ctx, client, projectID, owner.ID, 0, owner.RequestBody)
			require.NoError(t, err)
			require.Equal(t, body, []byte(loaded))
		})
	}
}

func TestChunkedRequestBodyLoadsLegacyCDCBoundaries(t *testing.T) {
	client, ctx, projectID := newChunkTestClient(t)
	body := chunkTestBody()
	chunks := legacyRequestBodyCDC(body)
	manifest, err := json.Marshal(requestBodyManifest{Version: 1, Size: len(body), SHA256: requestBodyHash(body), Chunks: len(chunks)})
	require.NoError(t, err)
	plan := &ChunkedRequestBody{Manifest: manifest, chunks: chunks}
	require.NotEqual(t, chunks, PrepareChunkedRequestBody(body).chunks, "the fixture must use the original chunk boundaries")
	var owner *ent.Request
	service := AbstractService{db: client}
	require.NoError(t, service.RunInTransaction(ctx, func(txCtx context.Context) error {
		tx := ent.FromContext(txCtx)
		var err error
		owner, err = tx.Request.Create().SetProjectID(projectID).SetModelID("legacy").SetRequestBody(plan.Manifest).
			SetRequestBodyChunked(true).SetStatus(request.StatusCompleted).Save(txCtx)
		if err != nil {
			return err
		}
		return plan.Save(txCtx, tx, projectID, owner.ID, 0)
	}))
	loaded, err := LoadChunkedRequestBody(ctx, client, projectID, owner.ID, 0, owner.RequestBody)
	require.NoError(t, err)
	require.Equal(t, body, []byte(loaded))
}
