// 职责: 重放逐轮请求和执行正文，测量持久化耗时、内容复用及数据库空间。
// 关联文件:
//   internal/server/biz/request.go
//   internal/server/biz/request_body_chunks.go
//   internal/server/biz/system.go
//   internal/server/biz/data_storage.go
// 注意事项:
//   私有 JSONL 仅从显式配置的文件读取，输出不包含正文。
//   PostgreSQL 仅接受本机专用测试库；逻辑正文量与数据库分配空间分别报告。

package biz

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

type payloadReplayTurn struct {
	RequestBody   json.RawMessage `json:"request_body"`
	ExecutionBody json.RawMessage `json:"execution_body"`
}

type payloadReplayMetrics struct {
	seen                                       map[string]struct{}
	previous                                   [2][]byte
	lanes                                      [2]payloadReplayLaneMetrics
	input, changed, unique, inline, manifests  int64
	references, chunked, sanitized, normalized int64
}

type payloadReplayLaneMetrics struct {
	inputChanged, envelopeChanged, seedInput, seedPayload  int64
	smallAppendCount, smallAppendInput, smallAppendPayload int64
}

type payloadReplayInputDelta struct {
	available, seed, appended                                  bool
	inputChanged, envelopeChanged, removedItems, removedFields int64
}

// The changed middle span removes a common prefix and a non-overlapping suffix.
// It estimates new content, not minimum edit distance; pure insertion is exact.
func payloadReplayChangedSpan(previous, current []byte) int {
	prefix := 0
	for prefix < min(len(previous), len(current)) && previous[prefix] == current[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < min(len(previous), len(current))-prefix && previous[len(previous)-1-suffix] == current[len(current)-1-suffix] {
		suffix++
	}
	return len(current) - prefix - suffix
}

// Item and field spans count new-side bytes, excluding JSON separators and deleted
// bytes. Unchanged outer items are removed first; changed corresponding middle
// items then use a byte span estimate. Strict append retains every previous item.
func payloadReplayResponsesDelta(previous, current []byte) payloadReplayInputDelta {
	var before, after map[string]json.RawMessage
	var oldItems, newItems []json.RawMessage
	delta := payloadReplayInputDelta{seed: len(previous) == 0}
	if json.Unmarshal(current, &after) != nil || len(after["input"]) == 0 || after["input"][0] != '[' || json.Unmarshal(after["input"], &newItems) != nil {
		return delta
	}
	if !delta.seed && (json.Unmarshal(previous, &before) != nil || len(before["input"]) == 0 || before["input"][0] != '[' || json.Unmarshal(before["input"], &oldItems) != nil) {
		return delta
	}
	delta.available = true
	prefix := 0
	for prefix < min(len(oldItems), len(newItems)) && bytes.Equal(oldItems[prefix], newItems[prefix]) {
		prefix++
	}
	suffix := 0
	for suffix < min(len(oldItems), len(newItems))-prefix && bytes.Equal(oldItems[len(oldItems)-1-suffix], newItems[len(newItems)-1-suffix]) {
		suffix++
	}
	delta.appended = !delta.seed && prefix == len(oldItems) && len(newItems) > len(oldItems)
	oldMiddle, newMiddle := oldItems[prefix:len(oldItems)-suffix], newItems[prefix:len(newItems)-suffix]
	delta.removedItems = int64(max(0, len(oldMiddle)-len(newMiddle)))
	for i, item := range newMiddle {
		var oldItem []byte
		if i < len(oldMiddle) {
			oldItem = oldMiddle[i]
		}
		delta.inputChanged += int64(payloadReplayChangedSpan(oldItem, item))
	}
	for key, value := range after {
		if key != "input" {
			delta.envelopeChanged += int64(payloadReplayChangedSpan(before[key], value))
		}
	}
	for key := range before {
		if _, exists := after[key]; key != "input" && !exists {
			delta.removedFields++
		}
	}
	return delta
}

func payloadReplayRatio(numerator, denominator int64) float64 {
	if denominator > 0 {
		return float64(numerator) / float64(denominator)
	}
	if numerator > 0 {
		return math.Inf(1)
	}
	return 0
}

func (m *payloadReplayMetrics) observe(t *testing.T, lane int, body []byte, chunked bool, manifestBytes int) {
	t.Helper()
	m.input += int64(len(body))
	m.changed += int64(payloadReplayChangedSpan(m.previous[lane], body))
	m.previous[lane] = bytes.Clone(body)
	if !chunked {
		m.inline += int64(len(body))
		return
	}
	plan := PrepareChunkedRequestBody(body)
	require.NotNil(t, plan, "persisted chunked body must have a reproducible plan")
	m.chunked++
	m.manifests += int64(manifestBytes)
	m.references += int64(len(plan.chunks))
	for _, chunk := range plan.chunks {
		hash := requestBodyHash(chunk)
		if _, exists := m.seen[hash]; !exists {
			m.seen[hash] = struct{}{}
			m.unique += int64(len(chunk))
		}
	}
}

func TestRequestBodyPayloadReplayAppendOnly(t *testing.T) {
	var input bytes.Buffer
	random := rand.New(rand.NewSource(37))
	text := make([]byte, 0, 64*1024)
	text = append(text, make([]byte, 64*1024)...)
	fill := func(data []byte) {
		for i := range data {
			data[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[random.Intn(36)]
		}
	}
	fill(text)
	var finalBody []byte
	for range 16 {
		finalBody = append(append([]byte(`{"model":"test","input":[{"role":"user","content":"`), text...), []byte(`"}]}`)...)
		input.WriteString(`{"request_body":`)
		input.Write(finalBody)
		input.WriteString(`,"execution_body":`)
		input.Write(finalBody)
		input.WriteString("}\n")
		addition := make([]byte, 2048)
		fill(addition)
		text = append(text, addition...)
	}
	metrics := runPayloadReplay(t, &input, "deterministic_append", "")
	require.Equal(t, int64(2*len(finalBody)), metrics.changed, "append-only changed spans equal initial content plus inserted content")
	require.Equal(t, int64(15*2048), metrics.lanes[0].inputChanged-metrics.lanes[0].seedInput)
	require.Zero(t, metrics.lanes[0].smallAppendCount, "growth inside one item is not a strict item append")
	if metrics.chunked > 0 {
		require.Zero(t, metrics.inline)
		require.Less(t, metrics.unique, metrics.input, "identical execution bodies must reuse request chunks")
	} else {
		require.Equal(t, metrics.input, metrics.inline, "the same runner also supports an inline baseline")
	}
}

func TestRequestBodyPayloadReplayInputDelta(t *testing.T) {
	before := []byte(`{"client_metadata":"before","input":[{"content":"history"}],"last":"before"}`)
	appended := []byte(`{"client_metadata":"after","input":[{"content":"history"},{"content":"tool result"}],"last":"after"}`)
	delta := payloadReplayResponsesDelta(before, appended)
	require.True(t, delta.available && delta.appended && !delta.seed)
	require.Equal(t, int64(len(`{"content":"tool result"}`)), delta.inputChanged)
	require.Greater(t, int64(payloadReplayChangedSpan(before, appended)), delta.inputChanged+delta.envelopeChanged, "raw span includes unchanged history between envelope edits")
	changed := payloadReplayResponsesDelta(before, []byte(`{"client_metadata":"before","input":[{"content":"history++"}],"last":"before"}`))
	require.False(t, changed.appended)
	require.Equal(t, int64(2), changed.inputChanged)
	removed := payloadReplayResponsesDelta(before, []byte(`{"input":[]}`))
	require.Zero(t, removed.inputChanged+removed.envelopeChanged)
	require.Equal(t, int64(1), removed.removedItems)
	require.Equal(t, int64(2), removed.removedFields)
}

func TestRequestBodyPayloadReplayFile(t *testing.T) {
	path := os.Getenv("AXONHUB_TEST_PAYLOAD_REPLAY_FILE")
	if path == "" {
		t.Skip("AXONHUB_TEST_PAYLOAD_REPLAY_FILE is not configured")
	}
	file, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	runPayloadReplay(t, file, "explicit_jsonl", os.Getenv("AXONHUB_TEST_PAYLOAD_PG_DSN"))
}

func runPayloadReplay(t *testing.T, input io.Reader, source, pgDSN string) payloadReplayMetrics {
	t.Helper()
	client, db, backend := newPayloadReplayDatabase(t, pgDSN)
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	project := client.Project.Create().SetName("payload-replay").SaveX(ctx)
	ctx = contexts.WithProjectID(ctx, project.ID)
	storageRow := client.DataStorage.Create().SetName("primary").SetDescription("payload replay").
		SetPrimary(true).SetType("database").SetSettings(&objects.DataStorageSettings{}).SaveX(ctx)
	channelRow := client.Channel.Create().SetName("payload-replay").SetType(channel.TypeOpenai).
		SetBaseURL("https://example.test").SetCredentials(objects.ChannelCredentials{APIKey: "test"}).
		SetSupportedModels([]string{"test"}).SetDefaultTestModel("test").SaveX(ctx)
	system := NewSystemService(SystemServiceParams{Ent: client})
	channels := NewChannelServiceForTest(client)
	t.Cleanup(channels.Stop)
	t.Cleanup(channels.modelCache.Stop)
	storage := NewDataStorageService(DataStorageServiceParams{SystemService: system, Client: client})
	svc := NewRequestService(client, system.CacheConfig, system, NewUsageLogService(client, system, channels), storage, NewLiveStreamRegistry())
	policy := defaultStoragePolicy
	policy.StoreRequestBody = true
	require.NoError(t, system.SetStoragePolicy(ctx, &policy))
	require.NoError(t, system.SetDefaultDataStorageID(ctx, storageRow.ID))
	_, err := system.StoragePolicy(ctx)
	require.NoError(t, err)
	_, err = storage.GetDefaultDataStorage(ctx)
	require.NoError(t, err)
	_, err = storage.GetDataStorageByID(ctx, storageRow.ID)
	require.NoError(t, err)
	before := payloadReplayAllocatedBytes(t, ctx, db, backend)
	metrics := payloadReplayMetrics{seen: make(map[string]struct{})}
	sourceHash := sha256.New()
	reader := bufio.NewReader(io.TeeReader(input, sourceHash))
	var durations []time.Duration
	for turn := 1; ; turn++ {
		line, readErr := reader.ReadBytes('\n')
		if len(line) == 0 && readErr == io.EOF {
			break
		}
		require.True(t, readErr == nil || readErr == io.EOF, "cannot read replay turn=%d", turn)
		require.True(t, json.Valid(line), "invalid JSONL turn=%d", turn)
		var pair payloadReplayTurn
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		require.True(t, decoder.Decode(&pair) == nil, "invalid replay schema turn=%d", turn)
		require.True(t, len(pair.RequestBody) > 0 && len(pair.ExecutionBody) > 0, "both bodies are required turn=%d", turn)

		started := time.Now()
		req, err := svc.CreateRequest(ctx, &llm.Request{Model: "test"}, &httpclient.Request{JSONBody: pair.RequestBody}, llm.APIFormatOpenAIResponse)
		if err != nil {
			t.Fatalf("request persistence failed turn=%d error_type=%T", turn, err)
		}
		execution, err := svc.CreateRequestExecution(ctx, &Channel{Channel: channelRow}, "test", req,
			httpclient.Request{JSONBody: pair.ExecutionBody}, llm.APIFormatOpenAIResponse, false)
		elapsed := time.Since(started)
		require.True(t, err == nil, "execution persistence failed turn=%d error_type=%T", turn, err)
		durations = append(durations, elapsed)

		previousInput, previousChanged := metrics.input, metrics.changed
		previousUnique, previousInline := metrics.unique, metrics.inline
		for lane, raw := range [][]byte{pair.RequestBody, pair.ExecutionBody} {
			body := stripJSONNul(raw)
			metrics.sanitized += int64(len(raw) - len(body))
			var loaded objects.JSONRawMessage
			var loadErr error
			var chunked bool
			var storedBytes int
			if lane == 0 {
				fresh, err := client.Request.Get(ctx, req.ID)
				require.NoError(t, err)
				loaded, loadErr = svc.LoadRequestBody(ctx, fresh)
				chunked, storedBytes = fresh.RequestBodyChunked, len(fresh.RequestBody)
			} else {
				fresh, err := client.RequestExecution.Get(ctx, execution.ID)
				require.NoError(t, err)
				loaded, loadErr = svc.LoadRequestExecutionRequestBody(ctx, fresh)
				chunked, storedBytes = fresh.RequestBodyChunked, len(fresh.RequestBody)
			}
			require.True(t, loadErr == nil, "roundtrip load failed turn=%d lane=%d", turn, lane)
			if !chunked && !bytes.Equal(body, loaded) {
				if backend == dialect.Postgres {
					var equivalent bool
					err := db.QueryRowContext(ctx, "SELECT $1::jsonb = $2::jsonb", string(body), string(loaded)).Scan(&equivalent)
					require.True(t, err == nil && equivalent, "inline jsonb roundtrip mismatch turn=%d lane=%d", turn, lane)
				} else {
					serialized, err := json.Marshal(body)
					require.True(t, err == nil && bytes.Equal(serialized, loaded), "inline JSON serialization mismatch turn=%d lane=%d", turn, lane)
				}
				metrics.normalized++
			} else {
				require.True(t, bytes.Equal(body, loaded), "exact byte roundtrip mismatch turn=%d lane=%d", turn, lane)
			}
			delta := payloadReplayResponsesDelta(metrics.previous[lane], body)
			beforeUnique, beforeInline := metrics.unique, metrics.inline
			metrics.observe(t, lane, body, chunked, storedBytes)
			newUnique, newInline := metrics.unique-beforeUnique, metrics.inline-beforeInline
			laneMetrics := &metrics.lanes[lane]
			laneMetrics.inputChanged += delta.inputChanged
			laneMetrics.envelopeChanged += delta.envelopeChanged
			if delta.seed {
				laneMetrics.seedInput, laneMetrics.seedPayload = delta.inputChanged, newUnique+newInline
			} else if delta.appended && delta.inputChanged > 0 && delta.inputChanged <= 4096 {
				laneMetrics.smallAppendCount++
				laneMetrics.smallAppendInput += delta.inputChanged
				laneMetrics.smallAppendPayload += newUnique + newInline
			}
			t.Logf("turn=%d lane=%s input_metric_available=%t initial_seed=%t input_appended=%t input_changed_item_span_estimate_bytes=%d envelope_changed_field_span_estimate_bytes=%d input_removed_middle_items=%d envelope_removed_fields=%d new_unique_chunk_payload_bytes=%d new_inline_payload_bytes=%d payload_to_input_change_ratio=%.6f",
				turn, [2]string{"request", "execution"}[lane], delta.available, delta.seed, delta.appended, delta.inputChanged, delta.envelopeChanged,
				delta.removedItems, delta.removedFields, newUnique, newInline, payloadReplayRatio(newUnique+newInline, delta.inputChanged))
		}
		bodyBytes, changed := metrics.input-previousInput, metrics.changed-previousChanged
		newUnique, newInline := metrics.unique-previousUnique, metrics.inline-previousInline
		t.Logf("turn=%d request_bytes=%d execution_bytes=%d body_bytes=%d raw_middle_span_estimate_bytes=%d new_unique_chunk_payload_bytes=%d new_inline_payload_bytes=%d reused_byte_rate=%.6f incremental_payload_to_raw_span_ratio=%.6f cumulative_payload_bytes=%d cumulative_manifest_bytes=%d cumulative_reference_count=%d persist_pair_ms=%.3f",
			turn, len(pair.RequestBody), len(pair.ExecutionBody), bodyBytes, changed, newUnique, newInline,
			1-float64(newUnique+newInline)/float64(bodyBytes), payloadReplayRatio(newUnique+newInline, changed), metrics.unique+metrics.inline, metrics.manifests, metrics.references, float64(elapsed)/float64(time.Millisecond))
	}
	require.NotEmpty(t, durations, "replay requires at least one pair")
	var storedUnique, storedReferences int64
	lengthFunction := "length"
	if backend == dialect.Postgres {
		lengthFunction = "octet_length"
	}
	require.NoError(t, db.QueryRowContext(ctx, "SELECT coalesce(sum("+lengthFunction+"(data)), 0) FROM request_body_chunks").Scan(&storedUnique))
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM request_body_chunk_refs").Scan(&storedReferences))
	require.Equal(t, metrics.unique, storedUnique, "logical unique payload must match committed chunk data")
	require.Equal(t, metrics.references, storedReferences, "reference count must match committed rows")
	after := payloadReplayAllocatedBytes(t, ctx, db, backend)
	slices.Sort(durations)
	percentile := func(percent int) float64 {
		return float64(durations[(percent*len(durations)+99)/100-1]) / float64(time.Millisecond)
	}
	t.Logf("source=%s source_sha256=%x turns=%d backend=%s chunked_bodies=%d sanitized_nul_escape_bytes=%d canonicalized_inline_bodies=%d body_bytes=%d raw_middle_span_estimate_bytes=%d logical_unique_chunk_payload_bytes=%d logical_inline_payload_bytes=%d manifest_bytes=%d references=%d persist_pair_p50_ms=%.3f persist_pair_p95_ms=%.3f allocated_before_bytes=%d allocated_after_bytes=%d allocated_delta_bytes=%d",
		source, sourceHash.Sum(nil), len(durations), backend, metrics.chunked, metrics.sanitized, metrics.normalized, metrics.input, metrics.changed,
		metrics.unique, metrics.inline, metrics.manifests, metrics.references, percentile(50), percentile(95), before, after, after-before)
	for lane, summary := range metrics.lanes {
		t.Logf("lane=%s initial_seed_input_item_bytes=%d initial_seed_payload_bytes=%d cumulative_input_changed_item_span_estimate_bytes=%d cumulative_envelope_changed_field_span_estimate_bytes=%d small_append_count=%d small_append_input_item_bytes=%d small_append_payload_bytes=%d small_append_payload_to_input_ratio=%.6f",
			[2]string{"request", "execution"}[lane], summary.seedInput, summary.seedPayload, summary.inputChanged, summary.envelopeChanged,
			summary.smallAppendCount, summary.smallAppendInput, summary.smallAppendPayload, payloadReplayRatio(summary.smallAppendPayload, summary.smallAppendInput))
	}
	t.Log("input_metric=new_side_item_spans_after_common_item_prefix_suffix; strict_append=all_previous_items_unchanged_then_new_items; small_append=nonseed_strict_append_1_to_4096_input_item_bytes; raw_span_may_include_unchanged_history_between_edits; field_spans_exclude_keys_separators_deleted_bytes; spans_are_not_minimum_edit_distance")
	t.Log("timing=CreateRequest_through_committed_CreateRequestExecution; excluded=setup_input_read_statistics_roundtrip; logical_payload_excludes_manifests_indexes_row_overhead; allocated_bytes_exclude_WAL_journal")
	return metrics
}

func newPayloadReplayDatabase(t *testing.T, pgDSN string) (*ent.Client, *sql.DB, string) {
	t.Helper()
	if pgDSN == "" {
		client := enttest.Open(t, dialect.SQLite, "file:"+filepath.Join(t.TempDir(), "replay.sqlite")+"?_fk=0&_journal_mode=DELETE&_synchronous=FULL",
			enttest.WithMigrateOptions(migrate.WithForeignKeys(false)))
		t.Cleanup(func() { _ = client.Close() })
		db := client.Driver().(*entsql.Driver).DB()
		db.SetMaxOpenConns(1)
		t.Log("database=persistent_temporary_SQLite journal=DELETE synchronous=FULL allocation_scope=whole_database_with_indexes")
		return client, db, dialect.SQLite
	}
	endpoint, err := url.Parse(pgDSN)
	require.True(t, err == nil, "invalid test PostgreSQL DSN")
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, endpoint.Hostname())
	require.Equal(t, "/axonhub_payload_test", endpoint.Path, "restricted to the dedicated local test database")
	bootstrap, err := sql.Open("pgx", pgDSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = bootstrap.Close() })
	var database string
	require.NoError(t, bootstrap.QueryRowContext(t.Context(), "SELECT current_database()").Scan(&database))
	require.Equal(t, "axonhub_payload_test", database)
	schemaName := fmt.Sprintf("payload_replay_%d", time.Now().UnixNano())
	_, err = bootstrap.ExecContext(t.Context(), `CREATE SCHEMA "`+schemaName+`"`)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := bootstrap.ExecContext(ctx, `DROP SCHEMA "`+schemaName+`" CASCADE`)
		require.NoError(t, err)
	})
	query := endpoint.Query()
	query.Set("search_path", schemaName)
	endpoint.RawQuery = query.Encode()
	db, err := sql.Open("pgx", endpoint.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Schema.Create(authz.WithTestBypass(t.Context()), migrate.WithForeignKeys(false)))
	t.Log("database=local_dedicated_PostgreSQL_test_schema allocation_scope=requests_executions_chunks_refs_with_indexes_and_TOAST")
	return client, db, dialect.Postgres
}

func payloadReplayAllocatedBytes(t *testing.T, ctx context.Context, db *sql.DB, backend string) int64 {
	t.Helper()
	if backend == dialect.Postgres {
		var allocated int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT pg_total_relation_size('requests') + pg_total_relation_size('request_executions') + pg_total_relation_size('request_body_chunks') + pg_total_relation_size('request_body_chunk_refs')`).Scan(&allocated))
		return allocated
	}
	var pages, size int64
	require.NoError(t, db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pages))
	require.NoError(t, db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&size))
	return pages * size
}
