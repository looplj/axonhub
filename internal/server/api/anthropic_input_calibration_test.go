package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func calibrationBody(messages ...string) []byte {
	items := make([]map[string]any, 0, len(messages))
	for _, text := range messages {
		items = append(items, map[string]any{"role": "user", "content": text})
	}
	body, _ := json.Marshal(map[string]any{"model": "alias", "messages": items})
	return body
}

func calibrationFingerprint(t *testing.T, scope string, messages ...string) inputUsageFingerprint {
	t.Helper()
	f, ok := fingerprintInputUsage(calibrationBody(messages...), scope)
	require.True(t, ok)
	return f
}

// TestInputCalibrationPrediction checks measured-error transfer, stale ancestors,
// identity isolation and the prohibition on learning from in-flight requests.
func TestInputCalibrationPrediction(t *testing.T) {
	now := time.Now()
	parent := calibrationFingerprint(t, "route-a", "opening")
	next := calibrationFingerprint(t, "route-a", "opening", "next")
	cache := newInputUsageCache()
	cache.record(parent, 100, 120, now)
	count, mode := cache.predict(next, 150, now.Add(time.Second), now.Add(time.Second))
	require.Equal(t, int64(170), count)
	require.Equal(t, "calibrated", mode)
	count, mode = cache.predict(next, 150, now.Add(-time.Second), now)
	require.Equal(t, int64(150), count)
	require.Equal(t, "no_match", mode)
	other := calibrationFingerprint(t, "route-b", "opening", "next")
	count, mode = cache.predict(other, 150, now.Add(time.Second), now.Add(time.Second))
	require.Equal(t, int64(150), count)
	require.Equal(t, "no_match", mode)
	cache.record(next, 150, 160, now.Add(time.Second))
	fork := calibrationFingerprint(t, "route-a", "opening", "rewritten")
	count, mode = cache.predict(fork, 180, now.Add(2*time.Second), now.Add(2*time.Second))
	require.Equal(t, int64(180), count)
	require.Equal(t, "stale_prefix", mode)
	continuation := calibrationFingerprint(t, "route-a", "opening", "next", "continued")
	count, mode = cache.predict(continuation, 180, now.Add(2*time.Second), now.Add(2*time.Second))
	require.Equal(t, int64(190), count)
	require.Equal(t, "calibrated", mode)
	count, mode = cache.predict(continuation, 180, now.Add(2*time.Hour), now.Add(2*time.Hour))
	require.Equal(t, int64(180), count)
	require.Equal(t, "no_match", mode)
	require.Empty(t, cache.samples)
}

// Equivalent transport representations match, but changes within tool arguments
// or schemas must remain visible even when a user field is named cache_control.
func TestInputCalibrationFingerprint(t *testing.T) {
	a, ok := fingerprintInputUsage([]byte(`{"model":"m","system":"policy","messages":[{"role":"user","content":"hi"}],"metadata":{"user_id":"a"},"stream":true}`), "scope")
	require.True(t, ok)
	b, ok := fingerprintInputUsage([]byte(`{"stream":false,"metadata":{"user_id":"b"},"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}]}],"system":[{"type":"text","text":"policy","cache_control":{"type":"ephemeral"}}],"model":"m"}`), "scope")
	require.True(t, ok)
	require.Equal(t, a, b)
	for _, body := range []string{`{`, `null`, `{"messages":[]} `, `{"messages":[{}]} {}`} {
		_, ok := fingerprintInputUsage([]byte(body), "scope")
		require.False(t, ok)
	}
	for _, location := range []string{"input", "input_schema"} {
		body := fmt.Sprintf(`{"messages":[{"role":"assistant","content":[{"type":"tool_use","%s":{"cache_control":"%%s"}}]}]}`, location)
		a, _ := fingerprintInputUsage([]byte(fmt.Sprintf(body, "a")), "scope")
		b, _ := fingerprintInputUsage([]byte(fmt.Sprintf(body, "b")), "scope")
		require.NotEqual(t, a.prefixes, b.prefixes)
	}
	a, _ = fingerprintInputUsage([]byte(`{"tools":[{"name":"tool","input_schema":{"cache_control":"a"}}],"messages":[{"role":"user","content":"hi"}]}`), "scope")
	b, _ = fingerprintInputUsage([]byte(`{"tools":[{"name":"tool","input_schema":{"cache_control":"b"}}],"messages":[{"role":"user","content":"hi"}]}`), "scope")
	require.NotEqual(t, a.scope, b.scope)
}

func TestInputCalibrationCapacityAndConcurrency(t *testing.T) {
	cache := newInputUsageCache()
	now := time.Now()
	f := calibrationFingerprint(t, "scope", "a")
	for i := range inputCalibrationMaxEntries + 1 {
		cache.record(f, 100, 101, now.Add(time.Duration(i)))
	}
	require.Len(t, cache.samples, inputCalibrationMaxEntries)
	require.Equal(t, inputCalibrationMaxEntries, cache.prefixes)
	// Prefix memory has its own limit, independent of the number of entries.
	f.prefixes = make([][sha256.Size]byte, inputCalibrationMaxMessages)
	for range inputCalibrationMaxEntries {
		cache.record(f, 100, 101, now)
	}
	require.LessOrEqual(t, cache.prefixes, inputCalibrationMaxPrefixes)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("calibration panic: %v", r)
				}
			}()
			for range 25 {
				cache.record(f, 100, 101, time.Now())
				cache.predict(f, 150, time.Now(), time.Now())
			}
		})
	}
	wg.Wait()
	require.LessOrEqual(t, cache.prefixes, inputCalibrationMaxPrefixes)
}

func TestFinalAnthropicInputTokens(t *testing.T) {
	for _, tc := range []struct {
		body string
		want int64
	}{
		{`{"input_tokens":10,"cache_read_input_tokens":20,"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":30}}`, 60},
		{`{"input_tokens":0,"cache_read_input_tokens":20}`, 20},
		{`{"output_tokens":10}`, 0},
		{`{"input_tokens":-1}`, 0},
		{`{"input_tokens":1.5}`, 0},
		{`{"input_tokens":"10"}`, 0},
		{`{"input_tokens":9223372036854775807,"cache_read_input_tokens":1}`, 0},
	} {
		require.Equal(t, tc.want, finalAnthropicInputTokens(gjson.Parse(tc.body)), tc.body)
	}
	cache := newInputUsageCache()
	now := time.Now()
	cache.record(calibrationFingerprint(t, "s", "a"), 1, math.MaxInt64, now)
	count, mode := cache.predict(calibrationFingerprint(t, "s", "a", "b"), 10, now, now)
	require.Equal(t, int64(10), count)
	require.Equal(t, "overflow", mode)
}

type calibrationErrorStream struct {
	streams.Stream[*httpclient.StreamEvent]

	err error
}

func (s calibrationErrorStream) Err() error { return s.err }

// Learning waits for clean EOF after valid terminal events, while every final
// event remains byte-for-byte unchanged for persistence and downstream clients.
func TestInputCalibrationStreamLearning(t *testing.T) {
	for _, tc := range []struct {
		name               string
		stop, terminal     bool
		err                error
		cancel, closeEarly bool
	}{
		{name: "complete", stop: true, terminal: true},
		{name: "missing stop", terminal: true},
		{name: "missing terminal", stop: true},
		{name: "upstream failure", stop: true, terminal: true, err: errors.New("failed")},
		{name: "cancelled", stop: true, terminal: true, cancel: true},
		{name: "closed early", stop: true, terminal: true, closeEarly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := newInputUsageCache()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			ctx = contexts.WithProjectID(ctx, 1)
			ctx = contexts.WithAPIKey(ctx, &ent.APIKey{ID: 2})
			route := orchestrator.ResponseRoute{ChannelID: 3, Model: "model", APIFormat: "openai/responses"}
			calibration := newInputUsageCalibration(ctx, cache, route, time.Now())
			require.NotNil(t, calibration)
			events := []*httpclient.StreamEvent{{Data: []byte(`{"type":"message_start","message":{"model":"model","usage":{"input_tokens":0}}}`)}}
			final := `{"type":"message_delta","usage":{"input_tokens":30,"cache_read_input_tokens":70}}`
			if tc.terminal {
				final = `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":30,"cache_read_input_tokens":70}}`
			}
			events = append(events, &httpclient.StreamEvent{Data: []byte(final)})
			if tc.stop {
				events = append(events, &httpclient.StreamEvent{Data: []byte(`{"type":"message_stop"}`)})
			}
			wrapped := wrapAnthropicInputUsage(calibrationErrorStream{Stream: streams.SliceStream(events), err: tc.err}, &httpclient.Request{Path: "/v1/messages", Body: calibrationBody("first")}, calibration)
			require.True(t, wrapped.Next())
			require.Positive(t, gjson.GetBytes(wrapped.Current().Data, "message.usage.input_tokens").Int())
			if tc.cancel {
				cancel()
			}
			if tc.closeEarly {
				require.NoError(t, wrapped.Close())
			} else {
				require.True(t, wrapped.Next())
				require.Equal(t, final, string(wrapped.Current().Data))
				require.Empty(t, cache.samples)
				for wrapped.Next() {
				}
				require.False(t, wrapped.Next())
			}
			if tc.name == "complete" {
				require.Len(t, cache.samples, 1)
				require.Equal(t, int64(100), cache.samples[0].actual)
				next := newInputUsageCalibration(ctx, cache, route, time.Now())
				raw, err := estimateAnthropicInput(calibrationBody("first", "next"))
				require.NoError(t, err)
				require.Equal(t, raw+100-calibration.raw, next.estimate(calibrationBody("first", "next"), "model", raw))
				require.Equal(t, "calibrated", next.mode)
			} else {
				require.Empty(t, cache.samples)
			}
		})
	}
	require.Nil(t, newInputUsageCalibration(t.Context(), newInputUsageCache(), orchestrator.ResponseRoute{ChannelID: 1, Model: "m"}, time.Now()))
}
