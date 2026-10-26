package qb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

// This file contains the real-PostgreSQL parity tests for the DISTINCT ON
// dashboard query rewrites (#2754), plus the gate-free non-PostgreSQL
// fallback tests.
//
// Parity semantics: for an identical adversarial fixture, the baseline
// (ThroughputModeRowNumber) and candidate (ThroughputModeDistinctOn) queries
// must return exactly the same multiset of rows for:
//
//   - BuildThroughputQuery by channel and by model (sufficient limit),
//   - BuildDailyModelThroughputQuery and BuildDailyChannelThroughputQuery
//     with a per-day limit that actually excludes groups,
//   - BuildDailyPerformanceStatsQuery for model and channel grouping.
//
// Parity is asserted two ways: a sorted in-Go multiset comparison, and
// PostgreSQL EXCEPT ALL in both directions (baseline EXCEPT candidate,
// candidate EXCEPT baseline) as single statements, so the shared $1
// parameter stays valid inside one statement.
//
// The fixture additionally carries hand-computed expected aggregate values so
// the edge cases (created_at ties between executions with different ids,
// status filtering, zero-latency filtering, production-nullable channel_id and
// metrics_latency_ms, duplicate usage-log rows, the exact window boundary,
// streaming effective latency including TTFT >= latency, multiple dates, and
// per-day limit cutoff) are proven by the fixture itself,
// independent of which implementation the parity check compares.
//
// The PostgreSQL parity test is gated on AXONHUB_TEST_PG_DSN and skips cleanly
// when unset. It creates a unique temporary schema pinned via search_path
// and drops it in t.Cleanup, matching the repo convention (see
// internal/server/biz/quota_routing_migration_postgres_test.go).

// paritySince is the inclusive lower bound of the dashboard window:
// 2024-03-01T02:00:00Z. Rows exactly at this instant are included; rows one
// millisecond before are not.
var paritySince = time.Date(2024, 3, 1, 2, 0, 0, 0, time.UTC)

// dayBase returns the UTC midnight of fixture day 1..4 (2024-03-01..04).
func dayBase(day int) time.Time {
	return time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(day-1) * 24 * time.Hour)
}

type parityChannel struct {
	id   int
	name string
	typ  string
}

type parityModel struct {
	id   string
	name string
}

type parityRequest struct {
	id      int
	modelID string
}

// parityExecution is one seeded request_executions row. Execution ids start
// at 301 so they cannot collide with request ids (1..23), keeping the
// created_at tie-break on id unambiguous.
type parityExecution struct {
	id          int
	requestID   int
	channelID   int
	status      string
	created     time.Time
	latencyMS   int64 // 0 means the zero-latency filter case
	ttftMS      int64 // 0 means metrics_first_token_latency_ms IS NULL
	stream      bool
	noChannel   bool // channel_id is written as IS NULL (nullable in production)
	latencyNull bool // metrics_latency_ms is written as IS NULL (nullable in production)
}

type parityUsage struct {
	requestID int
	completed int64
}

// buildParityFixture defines the adversarial seed data.
//
// Execution rows ("ch NULL" is an explicitly production-nullable channel_id;
// "NULL lat" is an explicitly NULL metrics_latency_ms, never a sentinel):
//
//	301 R1  Alpha  d1 03:00  completed lat 10000
//	302 R2  Alpha  d1 05:00  completed lat 500
//	303 R6  Alpha  d1 10:00  completed lat 600   \ created_at tie on R6;
//	304 R6  Alpha  d1 10:00  completed lat 3000  / higher id (304) must win
//	305 R7  Alpha  d1 11:00  failed    lat 800   \ status filter
//	306 R7  Alpha  d1 11:00  canceled  lat 900   / status filter
//	307 R8  Gamma  d1 12:00  completed lat 0     (zero-latency filter)
//	308 R4  Beta   d1 01:00  completed lat 0     (pre-window AND zero-latency)
//	309 R5  Gamma  d1 00:00  completed lat 2000  (pre-window)
//	310 R9  Beta   d1 04:00  completed lat 4000
//	311 R10 Alpha  d1 02:00  completed lat 2000 stream, ttft NULL (full latency)
//	312 R16 Alpha  d1 02:00  completed lat 300   (EXACT window boundary, included)
//	313 R17 Beta   d1 01:59:59.999 completed lat 4000 stream ttft 500 (1ms before, excluded)
//	314 R18 Alpha  d2 01:00  completed lat 1000
//	315 R19 Beta   d2 02:00  completed lat 500
//	316 R20 Alpha  d2 03:00  completed lat 2000 stream ttft 3000 (ttft >= lat -> 0)
//	317 R21 Beta   d2 04:00  completed lat 3000 stream ttft 4000 (ttft >= lat -> 0)
//	318 R22 Beta   d2 05:00  completed lat 1500 stream ttft 400   (ttft < lat -> 1100)
//	319 R23 Gamma  d3 01:00  completed lat 4000
//	320 R15 Beta   d3 02:00  completed lat 1000 stream ttft 500   (ttft < lat -> 500)
//	321 R16 Alpha  d3 03:00  completed lat 4000
//	322 R17 Alpha  d3 04:00  completed lat 4000
//	323 R18 Beta   d4 01:00  completed lat 2500 stream, ttft NULL (full latency)
//	324 R19 Beta   d4 02:00  completed lat 1000
//	325 R20 Beta   d4 03:00  completed lat 1000
//	326 R21 Alpha  d4 04:00  completed lat 1000
//	327 R22 Beta   d4 05:00  completed lat 1200 stream ttft 2000  (ttft >= lat -> 0)
//	328 R23 Gamma  d4 06:00  completed lat 800  stream ttft 1000  (ttft >= lat -> 0)
//	329 R11 ch NULL d3 07:00 completed lat 6000  (latest eligible execution of
//	                                   R11; channel_id NULL so by-model counts
//	                                   it while the by-channel inner join
//	                                   excludes it)
//	330 R17 Alpha  d3 05:00  completed NULL lat  (LATER than 322: the
//	                                   eligibility filter must drop it before
//	                                   latest-per-request selection, so 322
//	                                   keeps R17)
//
// Latest-execution-per-request selection (the dedup is over the whole window,
// so the day-2 attempts of R15..R22 lose to later day-3/4 attempts):
//
//	{1:301, 2:302, 6:304, 9:310, 10:311, 11:329,
//	 15:320, 16:321, 17:322, 18:323, 19:324, 20:325, 21:326, 22:327, 23:328}
//
// Channel totals (tokens / raw latency / effective latency / count / thr):
// unchanged by the NULL-channel row, which the inner join excludes; Beta
// includes two usage-log rows for R18 (250 + 125 tokens), so its latency is
// joined twice, matching usage_logs aggregation semantics.
//
//	Alpha {R1,R2,R6,R10,R16,R17,R21}     1780 / 24500 / 24500 / 7 / 72.653...
//	Beta  {R9,R15,R18x2,R19,R20,R22}     1410 / 13200 / 11500 / 6 / 122.608...
//	Gamma {R23}                            400 / 800  / 0      / 1 / 0
//
// Model totals:
//
//	m1 {R1,R6,R10,R16,R17,R21}           1680 / 24000 / 24000 / 6 / 70.0
//	m2 {R2,R9,R15,R18x2,R19,R20,R22} 1510 / 13700 / 12000 / 7 / 125.833...
//	m3 {R23,R11}                          700 / 6800  / 6000  / 2 / 116.666...
//
// Daily channel (day: group tokens / eff / thr); day 2 has no groups:
//
//	d1 Alpha {R1,R2,R6,R10}  280  / 15500 / 18.064...
//	d1 Beta  {R9}             80  / 4000  / 20.0
//	d3 Alpha {R16,R17}       900  / 8000  / 112.5
//	d3 Beta  {R15}           100  / 500   / 200.0
//	d4 Alpha {R21}           600  / 1000  / 600.0
//	d4 Beta  {R18x2,R19,R20,R22} 1230 / 7000 / 175.714...
//	d4 Gamma {R23}           400  / 0     / 0   (cut by per-day limit 2)
//
// Daily model (per-day limit 2):
//
//	d1 m1 {R1,R6,R10}     180  / 15000 / 12.0
//	d1 m2 {R2,R9}         180  / 4500  / 40.0
//	d3 m1 {R16,R17}       900  / 8000  / 112.5
//	d3 m2 {R15}           100  / 500   / 200.0
//	d4 m1 {R21}           600  / 1000  / 600.0
//	d4 m2 {R18x2,R19,R20,R22} 1230 / 7000 / 175.714...
//	d4 m3 {R23}           400  / 0     / 0   (cut by per-day limit 2)
//
// Daily performance stats (model grouping, throughput > 0 filter, ttft =
// average of rows with ttft > 0, integer division, NULL when none):
//
//	03-01 m1 {R1,R6,R10}       tokens 180  lat 15000 ttft NULL   req 3 thr 12.0
//	03-01 m2 {R2,R9}           tokens 180  lat 4500  ttft NULL   req 2 thr 40.0
//	03-03 m3 {R11}             tokens 300  lat 6000  ttft NULL   req 1 thr 50.0
//	03-03 m1 {R16,R17}         tokens 900  lat 8000  ttft NULL   req 2 thr 112.5
//	03-03 m2 {R15}             tokens 100  lat 1000  ttft [500]  req 1 thr 200.0
//	03-04 m1 {R21}             tokens 600  lat 1000  ttft NULL   req 1 thr 600.0
//	03-04 m2 {R18x2,R19,R20,R22} tokens 1230 lat 8200 ttft [2000] req 4 thr 175.714...
//	03-04 m3 {R23}             tokens 400  lat 800   ttft [1000]  req 1 thr 0 -> dropped
func buildParityFixture() (channels []parityChannel, models []parityModel, requests []parityRequest, executions []parityExecution, usages []parityUsage) {
	ch := func(id int, name, typ string) parityChannel { return parityChannel{id: id, name: name, typ: typ} }
	md := func(id, name string) parityModel { return parityModel{id: id, name: name} }
	rq := func(id int, modelID string) parityRequest { return parityRequest{id: id, modelID: modelID} }
	tx := func(id, requestID, channelID, ds, h int, latency, ttft int64, stream bool) parityExecution {
		return parityExecution{
			id: id, requestID: requestID, channelID: channelID, status: "completed",
			created:   dayBase(ds).Add(time.Duration(h) * time.Hour),
			latencyMS: latency, ttftMS: ttft, stream: stream,
		}
	}
	ul := func(requestID int, completed int64) parityUsage {
		return parityUsage{requestID: requestID, completed: completed}
	}

	channels = []parityChannel{
		ch(1, "Alpha", "openai"),
		ch(2, "Beta", "anthropic"),
		ch(3, "Gamma", "local"),
	}
	models = []parityModel{
		md("m1", "model-one"),
		md("m2", "model-two"),
		md("m3", "model-three"),
	}
	requests = []parityRequest{
		rq(1, "m1"), rq(2, "m2"), rq(3, "m1"), rq(4, "m2"), rq(5, "m1"),
		rq(6, "m1"), rq(7, "m3"), rq(8, "m2"), rq(9, "m2"), rq(10, "m1"),
		rq(11, "m3"), rq(12, "m2"), rq(13, "m1"), rq(14, "m1"), rq(15, "m2"),
		rq(16, "m1"), rq(17, "m1"), rq(18, "m2"), rq(19, "m2"), rq(20, "m2"),
		rq(21, "m1"), rq(22, "m2"), rq(23, "m3"),
	}
	executions = []parityExecution{
		tx(301, 1, 1, 1, 3, 10000, 0, false),
		tx(302, 2, 1, 1, 5, 500, 0, false),
		tx(303, 6, 1, 1, 10, 600, 0, false),  // tie, lower id
		tx(304, 6, 1, 1, 10, 3000, 0, false), // tie, higher id wins
		{
			id:        305,
			requestID: 7,
			channelID: 1,
			status:    "failed",
			created:   dayBase(1).Add(11 * time.Hour),
			latencyMS: 800,
		},
		{
			id:        306,
			requestID: 7,
			channelID: 1,
			status:    "canceled",
			created:   dayBase(1).Add(11 * time.Hour),
			latencyMS: 900,
		},
		tx(307, 8, 3, 1, 12, 0, 0, false),   // zero latency
		tx(308, 4, 2, 1, 1, 0, 0, false),    // pre-window, zero latency
		tx(309, 5, 3, 1, 0, 2000, 0, false), // pre-window
		tx(310, 9, 2, 1, 4, 4000, 0, false),
		tx(311, 10, 1, 1, 2, 2000, 0, true), // stream, NULL ttft -> full latency
		tx(312, 16, 1, 1, 2, 300, 0, false), // exact window boundary
		{
			id:        313,
			requestID: 17,
			channelID: 2,
			status:    "completed",
			created:   paritySince.Add(-time.Millisecond),
			latencyMS: 4000,
			ttftMS:    500,
			stream:    true,
		},
		tx(314, 18, 1, 2, 1, 1000, 0, false),
		tx(315, 19, 2, 2, 2, 500, 0, false),
		tx(316, 20, 1, 2, 3, 2000, 3000, true), // ttft >= lat -> 0
		tx(317, 21, 2, 2, 4, 3000, 4000, true), // ttft >= lat -> 0
		tx(318, 22, 2, 2, 5, 1500, 400, true),  // ttft < lat -> 1100
		tx(319, 23, 3, 3, 1, 4000, 0, false),
		tx(320, 15, 2, 3, 2, 1000, 500, true), // ttft < lat -> 500
		tx(321, 16, 1, 3, 3, 4000, 0, false),
		tx(322, 17, 1, 3, 4, 4000, 0, false),
		tx(323, 18, 2, 4, 1, 2500, 0, true), // stream, NULL ttft -> full latency
		tx(324, 19, 2, 4, 2, 1000, 0, false),
		tx(325, 20, 2, 4, 3, 1000, 0, false),
		tx(326, 21, 1, 4, 4, 1000, 0, false),
		tx(327, 22, 2, 4, 5, 1200, 2000, true), // ttft >= lat -> 0
		tx(328, 23, 3, 4, 6, 800, 1000, true),  // ttft >= lat -> 0
		{
			id:        329,
			requestID: 11,
			noChannel: true, // channel_id IS NULL: by-model keeps it, by-channel join drops it
			status:    "completed",
			created:   dayBase(3).Add(7 * time.Hour),
			latencyMS: 6000,
		},
		{
			id:          330,
			requestID:   17,
			channelID:   1,
			status:      "completed",
			created:     dayBase(3).Add(5 * time.Hour),
			latencyNull: true, // NULL latency: filtered before latest-per-request selection
		},
	}
	usages = []parityUsage{
		ul(1, 100), ul(2, 100), ul(3, 300), ul(4, 300), ul(5, 200),
		ul(6, 60), ul(7, 10), ul(8, 50), ul(9, 80), ul(10, 20),
		ul(11, 300), ul(12, 80), ul(13, 90), ul(14, 70), ul(15, 100),
		ul(16, 450), ul(17, 450), ul(18, 250), ul(18, 125),
		ul(19, 500), ul(20, 300), ul(21, 600), ul(22, 55), ul(23, 400),
	}
	return channels, models, requests, executions, usages
}

// newParitySchema creates a unique temporary schema on the DSN, creates the
// bare tables the qb queries read, seeds the fixture, and wires cleanup.
// usage_logs.request_id is intentionally non-unique here so the fixture
// exercises duplicate usage rows, as allowed by the production schema.
func newParitySchema(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	config, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)

	adminDB := stdlib.OpenDB(*config.Copy())
	require.NoError(t, adminDB.PingContext(context.Background()))

	schema := "qb_par_" + uuid.NewString()[:8]
	_, err = adminDB.ExecContext(context.Background(), `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		_ = adminDB.Close()
	})

	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	require.NoError(t, db.PingContext(context.Background()))
	t.Cleanup(func() { _ = db.Close() })

	ddl := []string{
		`CREATE TABLE channels (
			id INT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL
		)`,
		`CREATE TABLE models (
			model_id TEXT PRIMARY KEY,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE requests (
			id INT PRIMARY KEY,
			model_id TEXT NOT NULL
		)`,
		`CREATE TABLE request_executions (
			id BIGINT PRIMARY KEY,
			request_id INT NOT NULL,
			channel_id INT NULL,
			status TEXT NOT NULL,
			metrics_latency_ms BIGINT NULL,
			metrics_first_token_latency_ms BIGINT,
			stream BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE usage_logs (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			request_id INT NOT NULL,
			completion_tokens BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
	}
	ctx := context.Background()
	for _, stmt := range ddl {
		_, err = db.ExecContext(ctx, stmt)
		require.NoError(t, err, "DDL failed: %s", strings.TrimSpace(stmt))
	}

	channels, models, requests, executions, usages := buildParityFixture()
	for _, c := range channels {
		_, err = db.ExecContext(ctx, `INSERT INTO channels (id, name, type) VALUES ($1, $2, $3)`, c.id, c.name, c.typ)
		require.NoError(t, err)
	}
	for _, m := range models {
		_, err = db.ExecContext(ctx, `INSERT INTO models (model_id, name) VALUES ($1, $2)`, m.id, m.name)
		require.NoError(t, err)
	}
	for _, r := range requests {
		_, err = db.ExecContext(ctx, `INSERT INTO requests (id, model_id) VALUES ($1, $2)`, r.id, r.modelID)
		require.NoError(t, err)
	}
	for _, e := range executions {
		// NULL handling uses explicit flags, matched to the production schema
		// where channel_id and metrics_latency_ms are nullable; a literal 0
		// latency is the zero-latency filter case, never a NULL sentinel.
		var channel any
		if !e.noChannel {
			channel = e.channelID
		}
		var lat any
		if !e.latencyNull {
			lat = e.latencyMS
		}
		var ttft any
		if e.ttftMS > 0 {
			ttft = e.ttftMS
		}
		_, err = db.ExecContext(ctx,
			`INSERT INTO request_executions (id, request_id, channel_id, status, metrics_latency_ms, metrics_first_token_latency_ms, stream, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			e.id, e.requestID, channel, e.status, lat, ttft, e.stream, e.created,
		)
		require.NoError(t, err, "insert execution id=%d", e.id)
	}
	for _, u := range usages {
		_, err = db.ExecContext(ctx,
			`INSERT INTO usage_logs (request_id, completion_tokens, created_at) VALUES ($1, $2, $3)`,
			u.requestID, u.completed, dayBase(1),
		)
		require.NoError(t, err, "insert usage for request_id=%d", u.requestID)
	}
	return db
}

// collectRows renders every result row as a deterministic string and returns
// them sorted, i.e. a comparable multiset (ordering ignored, duplicates kept).
func collectRows(t *testing.T, rows *sql.Rows) []string {
	t.Helper()
	defer rows.Close()

	cols, err := rows.Columns()
	require.NoError(t, err)
	var out []string
	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		require.NoError(t, rows.Scan(ptrs...))
		out = append(out, renderRow(raw))
	}
	require.NoError(t, rows.Err())
	slices.Sort(out)
	return out
}

func renderRow(vals []any) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		switch x := v.(type) {
		case nil:
			parts[i] = "NULL"
		case float64:
			parts[i] = strconv.FormatFloat(x, 'g', -1, 64)
		case string:
			parts[i] = x
		case []byte:
			parts[i] = string(x)
		case time.Time:
			parts[i] = x.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
		default:
			parts[i] = fmt.Sprintf("%v", x)
		}
	}
	return strings.Join(parts, "\x00")
}

// assertPGParity runs both queries and asserts exact multiset parity: a sorted
// in-Go comparison, then PostgreSQL EXCEPT ALL in both directions, each as a
// single statement with the shared $1 bound once per statement.
func assertPGParity(t *testing.T, db *sql.DB, label, baselineSQL, candidateSQL string, arg any) {
	t.Helper()
	ctx := context.Background()

	baselineRows, err := db.QueryContext(ctx, baselineSQL, arg)
	require.NoError(t, err, "baseline query (%s)", label)
	candidateRows, err := db.QueryContext(ctx, candidateSQL, arg)
	require.NoError(t, err, "candidate query (%s)", label)

	require.True(t,
		reflect.DeepEqual(collectRows(t, baselineRows), collectRows(t, candidateRows)),
		"row multiset mismatch for %s", label)

	// EXCEPT ALL preserves duplicate rows; wrapping each side in a subquery
	// keeps the trailing ORDER BY / LIMIT legal inside the set operand while
	// $1 stays bound once per statement.
	eq := func(lhs, rhs string) {
		t.Helper()
		var n int
		stmt := fmt.Sprintf(
			"SELECT COUNT(*) FROM (SELECT * FROM (%s) AS left_side EXCEPT ALL SELECT * FROM (%s) AS right_side) AS d",
			lhs, rhs)
		require.NoError(t, db.QueryRowContext(ctx, stmt, arg).Scan(&n),
			"EXCEPT ALL ran (%s)", label)
		if n != 0 {
			diffs, derr := db.QueryContext(ctx,
				fmt.Sprintf("SELECT * FROM (%s) AS left_side EXCEPT ALL SELECT * FROM (%s) AS right_side", lhs, rhs), arg)
			require.NoError(t, derr)
			t.Fatalf("%s: %d rows differ (left EXCEPT right): %v", label, n, collectRows(t, diffs))
		}
	}
	eq(baselineSQL, candidateSQL)
	eq(candidateSQL, baselineSQL)
}

// TestThroughputQueryDistinctOnParity_Postgres checks exact row parity
// (ROW_NUMBER baseline vs DISTINCT ON candidate) for every targeted dashboard
// shape against a real PostgreSQL server with an adversarial fixture, plus
// hand-computed expected values that prove the fixture drives the edge cases.
//
// Gated on AXONHUB_TEST_PG_DSN; skips cleanly when unset.
func TestThroughputQueryDistinctOnParity_Postgres(t *testing.T) {
	dsn := os.Getenv("AXONHUB_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("AXONHUB_TEST_PG_DSN not set; skipping real-Postgres DISTINCT ON parity test")
	}

	db := newParitySchema(t, dsn)

	// Per-day limit 2 equals the group count on day 1 (2 groups) and is
	// below it on days 3 and 4 (3 groups each), so the per-day window limit
	// is evaluated on every day that has groups and actually cuts one group
	// on day 3 (m3) and one on day 4 (m3). Day 2 has no groups of its own:
	// its executions lose the whole-window latest-per-request dedup to later
	// attempts.
	t.Run("build_throughput_by_channel", func(t *testing.T) {
		assertPGParity(t, db, "throughput-by-channel",
			BuildThroughputQuery(true, ThroughputQueryByChannel, 20, ThroughputModeRowNumber),
			BuildThroughputQuery(true, ThroughputQueryByChannel, 20, ThroughputModeDistinctOn),
			paritySince)
	})

	t.Run("build_throughput_by_model", func(t *testing.T) {
		assertPGParity(t, db, "throughput-by-model",
			BuildThroughputQuery(true, ThroughputQueryByModel, 20, ThroughputModeRowNumber),
			BuildThroughputQuery(true, ThroughputQueryByModel, 20, ThroughputModeDistinctOn),
			paritySince)
	})

	t.Run("daily_model_per_day_limit", func(t *testing.T) {
		assertPGParity(t, db, "daily-model",
			BuildDailyModelThroughputQuery("postgres", "UTC", 0, 2, ThroughputModeRowNumber),
			BuildDailyModelThroughputQuery("postgres", "UTC", 0, 2, ThroughputModeDistinctOn),
			paritySince)
	})

	t.Run("daily_channel_per_day_limit", func(t *testing.T) {
		assertPGParity(t, db, "daily-channel",
			BuildDailyChannelThroughputQuery("postgres", "UTC", 0, 2, ThroughputModeRowNumber),
			BuildDailyChannelThroughputQuery("postgres", "UTC", 0, 2, ThroughputModeDistinctOn),
			paritySince)
	})

	t.Run("daily_performance_stats_model", func(t *testing.T) {
		assertPGParity(t, db, "daily-perf-model",
			BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByModel, "$1", ThroughputModeRowNumber),
			BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByModel, "$1", ThroughputModeDistinctOn),
			paritySince)
	})

	t.Run("daily_performance_stats_channel", func(t *testing.T) {
		assertPGParity(t, db, "daily-perf-channel",
			BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByChannel, "$1", ThroughputModeRowNumber),
			BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByChannel, "$1", ThroughputModeDistinctOn),
			paritySince)
	})

	// Everything below runs only the BASELINE query: independent expected
	// values prove the fixture actually exercises the edge cases.
	t.Run("expected_channel_totals", func(t *testing.T) {
		ctx := context.Background()
		rows, err := db.QueryContext(ctx,
			BuildThroughputQuery(true, ThroughputQueryByChannel, 20, ThroughputModeRowNumber),
			paritySince)
		require.NoError(t, err)
		defer rows.Close()

		type grp struct {
			id, name, typ string
			tokens, lat   int64
			reqCount      int64
			throughput    float64
		}
		got := map[string]grp{}
		for rows.Next() {
			var g grp
			var id int
			require.NoError(t, rows.Scan(&id, &g.name, &g.typ, &g.tokens, &g.lat, &g.reqCount, &g.throughput))
			g.id = strconv.Itoa(id)
			got[g.id] = g
		}
		require.NoError(t, rows.Err())
		// Exactly 3 groups: the NULL-channel execution (id 329, R11) must not
		// create a group — the inner join on channels drops it, while the
		// by-model totals below count it.
		require.Len(t, got, 3, "failed/canceled, zero-latency, and pre-window rows must not create groups")

		check := func(id, name, typ string, tokens, lat int64, reqCount int64, thr float64) {
			t.Helper()
			g, ok := got[id]
			require.True(t, ok, "missing %q, got %v", id, parityKeys(got))
			require.Equal(t, name, g.name)
			require.Equal(t, typ, g.typ)
			require.EqualValues(t, tokens, g.tokens)
			require.EqualValues(t, lat, g.lat)
			require.EqualValues(t, reqCount, g.reqCount)
			require.InDelta(t, thr, g.throughput, 1e-9, "throughput for %q", id)
		}

		// R6 tie must resolve to id 304 (latency 3000), not 303 (600): the
		// expected Alpha latency sum of 24500 only works out with 3000.
		check("1", "Alpha", "openai", 1780, 24500, 7, 72.65306122448979)
		// Beta raw latency is 13200: 4000 (R9) + 1000 (R15) + 2500 twice (R18)
		// + 1000 (R19) + 1000 (R20) + 1200 (R22). Effective latency is 11500:
		// R15 contributes 500 (ttft 500 < lat 1000), R22 contributes 0
		// (ttft 2000 >= lat 1200), and all other joined rows keep full latency.
		check("2", "Beta", "anthropic", 1410, 13200, 6, 122.6086956521739)
		// Gamma: R23 only (R5 pre-window, R8 zero-latency, R7 failed/canceled);
		// effective latency 0 because the single stream row folds to 0.
		check("3", "Gamma", "local", 400, 800, 1, 0)
	})

	t.Run("expected_model_totals", func(t *testing.T) {
		ctx := context.Background()
		rows, err := db.QueryContext(ctx,
			BuildThroughputQuery(true, ThroughputQueryByModel, 20, ThroughputModeRowNumber),
			paritySince)
		require.NoError(t, err)
		defer rows.Close()

		type row struct {
			id, name    string
			tokens, lat int64
			reqCount    int64
			throughput  float64
		}
		got := map[string]row{}
		for rows.Next() {
			var r row
			require.NoError(t, rows.Scan(&r.id, &r.name, &r.tokens, &r.lat, &r.reqCount, &r.throughput))
			got[r.id] = r
		}
		require.NoError(t, rows.Err())
		require.Len(t, got, 3)

		check := func(id, name string, tokens, lat int64, reqCount int64, thr float64) {
			t.Helper()
			r, ok := got[id]
			require.True(t, ok, "missing %q, got %v", id, parityKeys(got))
			require.Equal(t, name, r.name)
			require.EqualValues(t, tokens, r.tokens)
			require.EqualValues(t, lat, r.lat)
			require.EqualValues(t, reqCount, r.reqCount)
			require.InDelta(t, thr, r.throughput, 1e-9, "throughput for %q", id)
		}
		check("m1", "model-one", 1680, 24000, 6, 70.0)
		// R18 has two usage rows (250 and 125 tokens), so m2's token sum is
		// 1510. Its latency is also joined twice; effective latency is 12000.
		check("m2", "model-two", 1510, 13700, 7, 125.83333333333333)
		// m3 also groups R11's NULL-channel execution (id 329, latency 6000,
		// 300 tokens): model grouping joins through requests and models, so
		// the NULL channel_id is retained here. R23's stream row folds to 0
		// effective latency, leaving 6000.
		check("m3", "model-three", 700, 6800, 2, 116.66666666666667)
	})

	t.Run("expected_daily_channel", func(t *testing.T) {
		ctx := context.Background()
		rows, err := db.QueryContext(ctx,
			BuildDailyChannelThroughputQuery("postgres", "UTC", 0, 2, ThroughputModeRowNumber),
			paritySince)
		require.NoError(t, err)
		defer rows.Close()

		type row struct {
			date, name string
			tokens     int64
			reqCount   int64
			throughput float64
		}
		got := map[string]row{}
		for rows.Next() {
			var date string
			var id int
			var r row
			require.NoError(t, rows.Scan(&date, &id, &r.name, &r.tokens, &r.reqCount, &r.throughput))
			got[date+"\x00"+strconv.Itoa(id)] = r
		}
		require.NoError(t, rows.Err())

		// 6 rows: day 1 (2 groups), day 3 (2 groups) and day 4 (3 groups);
		// per-day limit 2 cuts day-4 Gamma (zero effective latency) and keeps
		// every day-1/day-3 group at its rank.
		require.Len(t, got, 6)

		check := func(key, name string, tokens, reqCount int64, thr float64) {
			t.Helper()
			r, ok := got[key]
			require.True(t, ok, "missing %q, got %v", key, parityKeys(got))
			require.Equal(t, name, r.name)
			require.EqualValues(t, tokens, r.tokens)
			require.EqualValues(t, reqCount, r.reqCount)
			require.InDelta(t, thr, r.throughput, 1e-9)
		}
		check("2024-03-01\x002", "Beta", 80, 1, 20.0)
		check("2024-03-01\x001", "Alpha", 280, 4, 18.06451612903226)
		check("2024-03-03\x001", "Alpha", 900, 2, 112.5)
		check("2024-03-03\x002", "Beta", 100, 1, 200.0)
		check("2024-03-04\x001", "Alpha", 600, 1, 600.0)
		// Day-4 Beta effective latency 7000 = 2500 (R18 twice) + 1000 (R19)
		// + 1000 (R20) + 0 (R22 folds to zero).
		check("2024-03-04\x002", "Beta", 1230, 4, 175.71428571428572)
		require.NotContains(t, got, "2024-03-04\x003", "per-day limit 2 must cut day-4 Gamma (zero effective latency)")
	})

	t.Run("expected_daily_model", func(t *testing.T) {
		ctx := context.Background()
		rows, err := db.QueryContext(ctx,
			BuildDailyModelThroughputQuery("postgres", "UTC", 0, 2, ThroughputModeRowNumber),
			paritySince)
		require.NoError(t, err)
		defer rows.Close()

		type row struct {
			date, name string
			tokens     int64
			reqCount   int64
			throughput float64
		}
		got := map[string]row{}
		for rows.Next() {
			var date, modelID string
			var r row
			require.NoError(t, rows.Scan(&date, &modelID, &r.name, &r.tokens, &r.reqCount, &r.throughput))
			got[date+"\x00"+modelID] = r
		}
		require.NoError(t, rows.Err())

		// 6 rows: day 1 (2 groups), day 3 (3 groups) and day 4 (3 groups);
		// per-day limit 2 cuts day-3 m3 (the NULL-channel execution, ranked
		// last at throughput 50) and day-4 m3 (zero effective latency). Day 2
		// has no groups: its executions lose the whole-window
		// latest-per-request dedup to later attempts.
		require.Len(t, got, 6)

		check := func(key, name string, tokens, reqCount int64, thr float64) {
			t.Helper()
			r, ok := got[key]
			require.True(t, ok, "missing %q, got %v", key, parityKeys(got))
			require.Equal(t, name, r.name)
			require.EqualValues(t, tokens, r.tokens)
			require.EqualValues(t, reqCount, r.reqCount)
			require.InDelta(t, thr, r.throughput, 1e-9)
		}
		check("2024-03-01\x00m2", "model-two", 180, 2, 40.0)
		check("2024-03-01\x00m1", "model-one", 180, 3, 12.0)
		check("2024-03-03\x00m1", "model-one", 900, 2, 112.5)
		check("2024-03-03\x00m2", "model-two", 100, 1, 200.0)
		check("2024-03-04\x00m1", "model-one", 600, 1, 600.0)
		// Day-4 m2 effective latency 7000 = 2500 (R18 twice) + 1000 (R19)
		// + 1000 (R20) + 0 (R22 folds to zero).
		check("2024-03-04\x00m2", "model-two", 1230, 4, 175.71428571428572)
		require.NotContains(t, got, "2024-03-03\x00m3", "per-day limit 2 must cut day-3 m3 (NULL-channel execution, throughput 50)")
		require.NotContains(t, got, "2024-03-04\x00m3", "per-day limit 2 must cut day-4 m3 (zero effective latency)")
		for _, day := range []string{"2024-03-02\x00m1", "2024-03-02\x00m2", "2024-03-02\x00m3"} {
			require.NotContains(t, got, day, "day 2 must have no groups (latest-per-request dedup is whole-window)")
		}
	})

	t.Run("expected_daily_performance_stats", func(t *testing.T) {
		ctx := context.Background()
		rows, err := db.QueryContext(ctx,
			BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByModel, "$1", ThroughputModeRowNumber),
			paritySince)
		require.NoError(t, err)
		defer rows.Close()

		type row struct {
			date       string
			tokens     int64
			lat        int64
			ttft       *float64
			reqCount   int64
			throughput float64
		}
		got := map[string]row{}
		for rows.Next() {
			var date, modelID string
			var r row
			require.NoError(t, rows.Scan(&date, &modelID, &r.tokens, &r.lat, &r.ttft, &r.reqCount, &r.throughput))
			got[date+"\x00"+modelID] = r
		}
		require.NoError(t, rows.Err())

		// 7 rows: 03-01 {m1,m2}, 03-03 {m1,m2,m3}, 03-04 {m1,m2}. The 03-03 m3
		// row is R11's NULL-channel execution (counted in performance stats
		// because grouping goes through the model); day-4 m3 is dropped by
		// the throughput > 0 filter (a single stream row folding to zero
		// effective latency); day 2 has no groups at all.
		require.Len(t, got, 7)

		perf := func(key string) row {
			t.Helper()
			r, ok := got[key]
			require.True(t, ok, "missing %q, got %v", key, parityKeys(got))
			return r
		}
		check := func(key string, tokens, lat int64, ttft *float64, reqCount int64, thr float64) {
			t.Helper()
			r := perf(key)
			require.EqualValues(t, tokens, r.tokens)
			require.EqualValues(t, lat, r.lat)
			require.True(t, reflect.DeepEqual(ttft, r.ttft), "ttft for %q: want %v, got %v", key, ttft, r.ttft)
			require.EqualValues(t, reqCount, r.reqCount)
			require.InDelta(t, thr, r.throughput, 1e-9, "throughput for %q", key)
		}

		// Day 03-04, m2: R18 has two usage rows, so its 2500-ms latency is
		// joined twice; R22 (ttft 2000 >= latency 1200) folds to 0. Raw
		// latency is 8200, effective latency is 7000, and positive TTFT average is
		// 2000.
		check("2024-03-04\x00m2", 1230, 8200, new(2000.0), 4, 175.71428571428572)
		// Day 03-03, m3: R11's NULL-channel execution (latency 6000, 300
		// tokens, no TTFT): throughput 50, above the positive filter.
		check("2024-03-03\x00m3", 300, 6000, nil, 1, 50.0)
		// Day 03-03, m2: R15 only (ttft 500 < latency 1000, effective 500).
		check("2024-03-03\x00m2", 100, 1000, new(500.0), 1, 200.0)
		// Day 03-04 / 03-03 m1 and both day-1 groups have no ttft rows at all.
		check("2024-03-04\x00m1", 600, 1000, nil, 1, 600.0)
		check("2024-03-03\x00m1", 900, 8000, nil, 2, 112.5)
		check("2024-03-01\x00m1", 180, 15000, nil, 3, 12.0)
		check("2024-03-01\x00m2", 180, 4500, nil, 2, 40.0)
	})

	t.Run("expected_daily_performance_stats_channel", func(t *testing.T) {
		ctx := context.Background()
		rows, err := db.QueryContext(ctx,
			BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByChannel, "$1", ThroughputModeRowNumber),
			paritySince)
		require.NoError(t, err)
		defer rows.Close()

		type row struct {
			tokens     int64
			lat        int64
			ttft       *float64
			reqCount   int64
			throughput float64
		}
		got := map[string]row{}
		for rows.Next() {
			var date string
			var channelID sql.NullInt64
			var r row
			require.NoError(t, rows.Scan(&date, &channelID, &r.tokens, &r.lat, &r.ttft, &r.reqCount, &r.throughput))
			channelKey := "NULL"
			if channelID.Valid {
				channelKey = strconv.FormatInt(channelID.Int64, 10)
			}
			got[date+"\x00"+channelKey] = r
		}
		require.NoError(t, rows.Err())

		// Channel grouping reads channel_id from executions; day-1 Alpha is
		// R1, R2, R6, and R10: 280 tokens / 15500 ms, no positive TTFT, 4 requests.
		require.Len(t, got, 7)
		r, ok := got["2024-03-01\x001"]
		require.True(t, ok, "missing day-1 channel 1, got %v", parityKeys(got))
		require.EqualValues(t, 280, r.tokens)
		require.EqualValues(t, 15500, r.lat)
		require.Nil(t, r.ttft)
		require.EqualValues(t, 4, r.reqCount)
		require.InDelta(t, 18.06451612903226, r.throughput, 1e-9)
		// Daily performance stats groups directly on execution.channel_id, so
		// the production-nullable channel produces one NULL-keyed group.
		nullChannel, ok := got["2024-03-03\x00NULL"]
		require.True(t, ok, "missing NULL channel group, got %v", parityKeys(got))
		require.EqualValues(t, 300, nullChannel.tokens)
		require.EqualValues(t, 6000, nullChannel.lat)
		require.Nil(t, nullChannel.ttft)
		require.EqualValues(t, 1, nullChannel.reqCount)
		require.InDelta(t, 50.0, nullChannel.throughput, 1e-9)
		require.NotContains(t, got, "2024-03-04\x003", "zero-effective-latency Gamma must be filtered")
	})

	// Check the inclusive timestamp predicate by execution ID. R16's id 312 is
	// exactly at paritySince; id 313 is one millisecond before it. Request 17
	// also has a later in-window execution (id 322), so its request ID alone
	// cannot prove that the pre-window row was excluded.
	t.Run("window_boundary", func(t *testing.T) {
		ctx := context.Background()
		rows, err := db.QueryContext(ctx, `
			SELECT id FROM request_executions
			WHERE status = 'completed' AND metrics_latency_ms > 0 AND created_at >= $1
			ORDER BY id`, paritySince)
		require.NoError(t, err)
		defer rows.Close()

		var ids []int64
		for rows.Next() {
			var id int64
			require.NoError(t, rows.Scan(&id))
			ids = append(ids, id)
		}
		require.NoError(t, rows.Err())

		require.Contains(t, ids, int64(312), "execution exactly at the window start must be included")
		require.NotContains(t, ids, int64(313), "execution one millisecond before the window start must be excluded")
		require.Contains(t, ids, int64(304), "tie pair's higher id must be present")
		require.NotContains(t, ids, int64(307), "zero-latency execution must be filtered")
		require.NotContains(t, ids, int64(305), "failed execution must be filtered")
		require.NotContains(t, ids, int64(306), "canceled execution must be filtered")
		require.NotContains(t, ids, int64(309), "pre-window execution must be filtered")
		// NULL channel_id is a join concern, not an eligibility one: the
		// latest eligible execution with a NULL channel stays in the set.
		require.Contains(t, ids, int64(329), "NULL-channel execution must pass eligibility")
		// The later NULL-latency execution of R17 must be filtered before
		// latest-per-request selection, so the earlier in-window row wins.
		require.NotContains(t, ids, int64(330), "NULL-latency execution must be filtered")
	})
}

func parityKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// TestThroughputQueryNonPostgresFallback runs without any database: for
// non-PostgreSQL inputs, ThroughputModeDistinctOn must produce the identical
// ROW_NUMBER SQL, and for the PostgreSQL marker the builders must emit real
// DISTINCT ON SQL with exactly one $1 placeholder (so the combined EXCEPT ALL
// parity statements bind a single argument per statement).
func TestThroughputQueryNonPostgresFallback(t *testing.T) {
	for _, queryType := range []ThroughputQueryType{ThroughputQueryByChannel, ThroughputQueryByModel} {
		rowNumber := BuildThroughputQuery(false, queryType, 10, ThroughputModeRowNumber)
		got := BuildThroughputQuery(false, queryType, 10, ThroughputModeDistinctOn)
		require.Equal(t, rowNumber, got,
			"useDollarPlaceholders=false with DistinctOn must fall back to ROW_NUMBER")
		require.NotContains(t, got, "DISTINCT ON")
		require.Contains(t, got, "?")
		require.Zero(t, strings.Count(got, "$1"))
	}

	for _, queryType := range []ThroughputQueryType{ThroughputQueryByChannel, ThroughputQueryByModel} {
		rowNumber := BuildThroughputQuery(true, queryType, 10, ThroughputModeRowNumber)
		got := BuildThroughputQuery(true, queryType, 10, ThroughputModeDistinctOn)
		require.NotEqual(t, rowNumber, got, "postgres marker with DistinctOn must differ from ROW_NUMBER")
		require.Contains(t, got, "DISTINCT ON (request_id)")
		require.Equal(t, 1, strings.Count(got, "$1"), "exactly one $1 placeholder")
	}

	for _, dialect := range []string{"mysql", "sqlite3", "sqlite"} {
		require.Equal(t,
			BuildDailyModelThroughputQuery(dialect, "UTC", 0, 10, ThroughputModeRowNumber),
			BuildDailyModelThroughputQuery(dialect, "UTC", 0, 10, ThroughputModeDistinctOn),
			"dialect %s", dialect)
		require.Equal(t,
			BuildDailyChannelThroughputQuery(dialect, "UTC", 0, 10, ThroughputModeRowNumber),
			BuildDailyChannelThroughputQuery(dialect, "UTC", 0, 10, ThroughputModeDistinctOn),
			"dialect %s", dialect)
		require.Equal(t,
			BuildDailyPerformanceStatsQuery(dialect, "UTC", 0, DailyThroughputByModel, "?", ThroughputModeRowNumber),
			BuildDailyPerformanceStatsQuery(dialect, "UTC", 0, DailyThroughputByModel, "?", ThroughputModeDistinctOn),
			"dialect %s", dialect)
		require.Equal(t,
			BuildDailyPerformanceStatsQuery(dialect, "UTC", 0, DailyThroughputByChannel, "?", ThroughputModeRowNumber),
			BuildDailyPerformanceStatsQuery(dialect, "UTC", 0, DailyThroughputByChannel, "?", ThroughputModeDistinctOn),
			"dialect %s", dialect)
	}

	require.Contains(t, BuildDailyModelThroughputQuery("postgres", "UTC", 0, 10, ThroughputModeDistinctOn), "DISTINCT ON (request_id)")
	require.Contains(t, BuildDailyChannelThroughputQuery("postgres", "UTC", 0, 10, ThroughputModeDistinctOn), "DISTINCT ON (request_id)")
	require.Contains(t, BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByModel, "$1", ThroughputModeDistinctOn), "DISTINCT ON (se.request_id)")
	require.Contains(t, BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByChannel, "$1", ThroughputModeDistinctOn), "DISTINCT ON (se.request_id)")

	// Exactly one $1 per production query so the parity statements bind one
	// argument.
	require.Equal(t, 1, strings.Count(BuildThroughputQuery(true, ThroughputQueryByChannel, 20, ThroughputModeDistinctOn), "$1"))
	require.Equal(t, 1, strings.Count(BuildThroughputQuery(true, ThroughputQueryByModel, 20, ThroughputModeDistinctOn), "$1"))
	require.Equal(t, 1, strings.Count(BuildDailyModelThroughputQuery("postgres", "UTC", 0, 2, ThroughputModeDistinctOn), "$1"))
	require.Equal(t, 1, strings.Count(BuildDailyChannelThroughputQuery("postgres", "UTC", 0, 2, ThroughputModeDistinctOn), "$1"))
	require.Equal(t, 1, strings.Count(BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByModel, "$1", ThroughputModeDistinctOn), "$1"))
}
