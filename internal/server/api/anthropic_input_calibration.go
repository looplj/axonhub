package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/orchestrator"
)

// These limits bound retention and memory, not the predicted token count.
const (
	inputCalibrationTTL         = time.Hour
	inputCalibrationMaxEntries  = 1024
	inputCalibrationMaxPrefixes = 65536
	inputCalibrationMaxMessages = 4096
)

type inputUsageFingerprint struct {
	scope    [sha256.Size]byte
	prefixes [][sha256.Size]byte
}

type inputUsageSample struct {
	inputUsageFingerprint

	raw, actual int64
	completed   time.Time
}

// inputUsageCache stores hashes and measured counts only. It is process-local;
// restarting or evicting entries simply restores uncalibrated estimation.
type inputUsageCache struct {
	mu       sync.Mutex
	samples  []inputUsageSample
	prefixes int
}

func newInputUsageCache() *inputUsageCache { return &inputUsageCache{} }

func (c *inputUsageCache) expire(now time.Time) {
	for len(c.samples) > 0 && !now.Before(c.samples[0].completed.Add(inputCalibrationTTL)) {
		c.removeOldest()
	}
}

func (c *inputUsageCache) removeOldest() {
	c.prefixes -= len(c.samples[0].prefixes)
	c.samples[0] = inputUsageSample{}
	c.samples = c.samples[1:]
}

// calibrationPrefix compares cumulative hashes, so matching a prefix covers
// every preceding message, not just the last message or a caller's session ID.
func calibrationPrefix(prefix, full [][sha256.Size]byte) bool {
	return len(prefix) > 0 && len(prefix) <= len(full) && prefix[len(prefix)-1] == full[len(prefix)-1]
}

// predict uses only samples completed before this request began. A newer
// incompatible descendant invalidates an old matching ancestor's correction.
func (c *inputUsageCache) predict(f inputUsageFingerprint, raw int64, started, now time.Time) (int64, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expire(now)
	anchor := -1
	for i, s := range c.samples {
		if s.scope != f.scope || s.completed.After(started) || len(s.prefixes) >= len(f.prefixes) || !calibrationPrefix(s.prefixes, f.prefixes) {
			continue
		}
		if anchor < 0 || len(s.prefixes) >= len(c.samples[anchor].prefixes) {
			anchor = i
		}
	}
	if anchor < 0 {
		return raw, "no_match"
	}
	a := c.samples[anchor]
	for _, s := range c.samples {
		if s.scope == a.scope && s.completed.After(a.completed) && !s.completed.After(started) && len(s.prefixes) > len(a.prefixes) && calibrationPrefix(a.prefixes, s.prefixes) && !calibrationPrefix(s.prefixes, f.prefixes) {
			return raw, "stale_prefix"
		}
	}
	correction := a.actual - a.raw
	if correction > 0 && raw > math.MaxInt64-correction {
		return raw, "overflow"
	}
	return max(int64(0), raw+correction), "calibrated"
}

func (c *inputUsageCache) record(f inputUsageFingerprint, raw, actual int64, now time.Time) {
	if raw <= 0 || actual <= 0 || len(f.prefixes) == 0 || len(f.prefixes) > inputCalibrationMaxMessages {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expire(now)
	for len(c.samples) >= inputCalibrationMaxEntries || c.prefixes+len(f.prefixes) > inputCalibrationMaxPrefixes {
		c.removeOldest()
	}
	// Time is sampled under the lock by the caller's stream; insertion order is
	// not completion order when concurrent streams race to acquire this lock.
	sample := inputUsageSample{inputUsageFingerprint: f, raw: raw, actual: actual, completed: now}
	sample.prefixes = slices.Clone(f.prefixes)
	c.samples = append(c.samples, sample)
	slices.SortStableFunc(c.samples, func(a, b inputUsageSample) int { return a.completed.Compare(b.completed) })
	c.prefixes += len(f.prefixes)
}

// normalizeCalibrationContent removes transport cache annotations only at
// content-block boundaries. Tool arguments and JSON schemas remain significant.
func normalizeCalibrationContent(v any) any {
	if text, ok := v.(string); ok {
		return []any{map[string]any{"type": "text", "text": text}}
	}
	blocks, ok := v.([]any)
	if !ok {
		return v
	}
	for _, item := range blocks {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		delete(block, "cache_control")
		kind, _ := block["type"].(string)
		if kind == "tool_result" || kind == "mcp_tool_result" {
			block["content"] = normalizeCalibrationContent(block["content"])
		}
	}
	return blocks
}

// fingerprintInputUsage retains no prompt text. Unknown control fields remain
// in the scope, so changes fail back to raw estimation instead of sharing state.
func fingerprintInputUsage(body []byte, identity any) (inputUsageFingerprint, bool) {
	var request map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&request) != nil || request == nil {
		return inputUsageFingerprint{}, false
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return inputUsageFingerprint{}, false
	}
	messages, ok := request["messages"].([]any)
	if !ok || len(messages) == 0 || len(messages) > inputCalibrationMaxMessages {
		return inputUsageFingerprint{}, false
	}
	delete(request, "messages")
	delete(request, "metadata")
	delete(request, "stream")
	if system, ok := request["system"]; ok {
		request["system"] = normalizeCalibrationContent(system)
	}
	if tools, ok := request["tools"].([]any); ok {
		for _, v := range tools {
			if tool, ok := v.(map[string]any); ok {
				delete(tool, "cache_control")
			}
		}
	}
	scope, err := json.Marshal([]any{identity, request})
	if err != nil {
		return inputUsageFingerprint{}, false
	}
	f := inputUsageFingerprint{scope: sha256.Sum256(scope), prefixes: make([][sha256.Size]byte, 0, len(messages))}
	var previous [sha256.Size]byte
	for _, v := range messages {
		m, ok := v.(map[string]any)
		if !ok {
			return inputUsageFingerprint{}, false
		}
		delete(m, "cache_control")
		m["content"] = normalizeCalibrationContent(m["content"])
		encoded, err := json.Marshal(m)
		if err != nil {
			return inputUsageFingerprint{}, false
		}
		h := sha256.New()
		_, _ = h.Write(previous[:])
		_, _ = h.Write(encoded)
		copy(previous[:], h.Sum(nil))
		f.prefixes = append(f.prefixes, previous)
	}
	return f, true
}

type inputUsageCalibration struct {
	ctx            context.Context
	cache          *inputUsageCache
	identity       any
	started        time.Time
	fingerprint    inputUsageFingerprint
	raw, predicted int64
	mode           string
}

// newInputUsageCalibration requires authenticated scope and the selected route;
// unidentified requests still receive the existing stateless estimate.
func newInputUsageCalibration(ctx context.Context, cache *inputUsageCache, route orchestrator.ResponseRoute, started time.Time) *inputUsageCalibration {
	key, ok := contexts.GetAPIKey(ctx)
	project, hasProject := contexts.GetProjectID(ctx)
	if cache == nil || !ok || key == nil || key.ID <= 0 || !hasProject || project <= 0 || route.ChannelID <= 0 || route.Model == "" {
		return nil
	}
	return &inputUsageCalibration{ctx: ctx, cache: cache, started: started, identity: struct {
		Project, Key int
		Route        orchestrator.ResponseRoute
	}{project, key.ID, route}}
}

func (c *inputUsageCalibration) estimate(body []byte, reportedModel string, raw int64) int64 {
	f, ok := fingerprintInputUsage(body, []any{c.identity, reportedModel})
	if !ok {
		return raw
	}
	c.fingerprint = f
	c.raw = raw
	c.predicted, c.mode = c.cache.predict(f, raw, c.started, time.Now())
	return c.predicted
}

// complete logs numeric evidence for the local trial and trains only from a
// cleanly completed stream's authoritative final input usage, including caches.
func (c *inputUsageCalibration) complete(actual int64) {
	if c.raw <= 0 || actual <= 0 || c.ctx.Err() != nil {
		return
	}
	c.cache.record(c.fingerprint, c.raw, actual, time.Now())
	log.Info(c.ctx, "Anthropic input usage calibration",
		log.String("calibration", c.mode), log.Int64("raw_input_tokens", c.raw),
		log.Int64("estimated_input_tokens", c.predicted), log.Int64("actual_input_tokens", actual))
}
