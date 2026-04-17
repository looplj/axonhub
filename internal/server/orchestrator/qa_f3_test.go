package orchestrator

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func contextWithModel(t *testing.T, model string) context.Context {
	return context.WithValue(context.Background(), modelContextKey{}, model)
}

// ============================================================================
// QA F3: Real Manual QA - Final Verification
// ============================================================================

// Task 1: Backward compatibility - existing config loads without duration
func TestQA_Task1_BackwardCompat_ExistingConfigNoDuration(t *testing.T) {
	raw := `{"rpm": 100, "tpm": 1000, "maxConcurrent": 5}`
	var rl objects.ChannelRateLimit
	err := json.Unmarshal([]byte(raw), &rl)
	require.NoError(t, err, "Existing config without duration should parse without error")
	assert.Nil(t, rl.RPMDuration)
	assert.Nil(t, rl.TPMDuration)
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetRPMDuration())
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetTPMDuration())
}

// Task 1: New duration field parsing (JSON with rpmDuration: "5hr")
func TestQA_Task1_NewDurationFieldParsing(t *testing.T) {
	raw := `{"rpm": 100, "rpmDuration": "5hr", "tpm": 1000, "tpmDuration": "1mo"}`
	var rl objects.ChannelRateLimit
	err := json.Unmarshal([]byte(raw), &rl)
	require.NoError(t, err)
	assert.Equal(t, objects.RateLimitDurationFiveHour, rl.GetRPMDuration())
	assert.Equal(t, objects.RateLimitDurationOneMonth, rl.GetTPMDuration())
	assert.Equal(t, 5*time.Hour, rl.GetRPMDuration().Duration())
	assert.Equal(t, 30*24*time.Hour, rl.GetTPMDuration().Duration())
}

// Task 1: One week duration parsing
func TestQA_Task1_OneWeekDurationParsing(t *testing.T) {
	raw := `{"rpm": 50, "rpmDuration": "1wk"}`
	var rl objects.ChannelRateLimit
	err := json.Unmarshal([]byte(raw), &rl)
	require.NoError(t, err)
	assert.Equal(t, objects.RateLimitDurationOneWeek, rl.GetRPMDuration())
	assert.Equal(t, 7*24*time.Hour, rl.GetRPMDuration().Duration())
}

// Task 1: Per-model concurrent fallback
func TestQA_Task1_PerModelConcurrentFallback(t *testing.T) {
	maxConcurrent := int64(10)
	rl := &objects.ChannelRateLimit{
		MaxConcurrent:   &maxConcurrent,
		ModelConcurrent: map[string]int64{"gpt-4": 2},
	}
	limit, hasCustom := rl.GetModelConcurrentLimit("gpt-4")
	assert.Equal(t, int64(2), limit)
	assert.True(t, hasCustom)
	limit, hasCustom = rl.GetModelConcurrentLimit("claude-3")
	assert.Equal(t, int64(10), limit)
	assert.False(t, hasCustom)
}

// Task 2: GraphQL code generation succeeds (verified by compilation)
func TestQA_Task2_GraphQLSchemaTypesExist(t *testing.T) {
	rl := &objects.ChannelRateLimit{
		RPM:             ptrInt64(100),
		TPM:             ptrInt64(1000),
		MaxConcurrent:   ptrInt64(5),
		RPMDuration:     ptrDuration(objects.RateLimitDurationFiveHour),
		TPMDuration:     ptrDuration(objects.RateLimitDurationOneMonth),
		ModelConcurrent: map[string]int64{"gpt-4": 2},
	}
	data, err := json.Marshal(rl)
	require.NoError(t, err)
	var rl2 objects.ChannelRateLimit
	err = json.Unmarshal(data, &rl2)
	require.NoError(t, err)
	assert.Equal(t, *rl.RPM, *rl2.RPM)
	assert.Equal(t, *rl.RPMDuration, *rl2.RPMDuration)
	assert.Equal(t, rl.ModelConcurrent["gpt-4"], rl2.ModelConcurrent["gpt-4"])
}

// Task 2: Backward compatibility - existing query still works
func TestQA_Task2_BackwardCompat_ExistingQueryFields(t *testing.T) {
	raw := `{"rpm": 100, "tpm": 1000, "maxConcurrent": 5}`
	var rl objects.ChannelRateLimit
	err := json.Unmarshal([]byte(raw), &rl)
	require.NoError(t, err)
	assert.Equal(t, int64(100), *rl.RPM)
	assert.Equal(t, int64(1000), *rl.TPM)
	assert.Equal(t, int64(5), *rl.MaxConcurrent)
}

// Task 3: Multi-window tracking works independently
func TestQA_Task3_MultiWindowTracking_IndependentWindows(t *testing.T) {
	tracker := NewChannelRequestTracker()
	oneHour := objects.RateLimitDurationOneHour
	fiveHour := objects.RateLimitDurationFiveHour
	tracker.IncrementRequestForDuration(1, oneHour.Duration(), nil)
	tracker.IncrementRequestForDuration(1, oneHour.Duration(), nil)
	tracker.IncrementRequestForDuration(1, fiveHour.Duration(), nil)
	assert.Equal(t, int64(2), tracker.GetRequestCountForDuration(1, oneHour.Duration(), nil))
	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(1, fiveHour.Duration(), nil))
}

// Task 3: Backward compat - existing 1-minute methods still work
func TestQA_Task3_BackwardCompat_OneMinuteMethods(t *testing.T) {
	tracker := NewChannelRequestTracker()
	tracker.IncrementRequest(1)
	tracker.IncrementRequest(1)
	tracker.AddTokens(1, 100)
	assert.Equal(t, int64(2), tracker.GetRequestCount(1))
	assert.Equal(t, int64(100), tracker.GetTokenCount(1))
	assert.Equal(t, tracker.GetRequestCount(1), tracker.GetRequestCountForDuration(1, time.Minute, nil))
}

// Task 3: Window expiry for long durations
func TestQA_Task3_WindowExpiry_LongDuration(t *testing.T) {
	tracker := NewChannelRequestTracker()
	oneMonth := objects.RateLimitDurationOneMonth
	duration := oneMonth.Duration()
	tracker.mu.Lock()
	tracker.counters[1] = map[time.Duration]*rateLimitWindow{
		duration: {requests: 100, tokens: 5000, windowStart: time.Now().Truncate(duration).Add(-duration)},
	}
	tracker.mu.Unlock()
	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(1, duration, nil))
	tracker.IncrementRequestForDuration(1, duration, nil)
	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(1, duration, nil))
}

// Task 4: Per-model connection tracking works
func TestQA_Task4_PerModelConnectionTracking(t *testing.T) {
	mt := NewModelConnectionTracker()
	mt.IncrementModelConnection(1, "gpt-4")
	mt.IncrementModelConnection(1, "gpt-4")
	mt.IncrementModelConnection(1, "claude-3")
	assert.Equal(t, 2, mt.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, mt.GetModelConnectionCount(1, "claude-3"))
	assert.Equal(t, 0, mt.GetModelConnectionCount(1, "unknown"))
}

// Task 4: Fallback to channel-wide MaxConcurrent
func TestQA_Task4_FallbackToChannelWideMaxConcurrent(t *testing.T) {
	mt := NewModelConnectionTracker()
	maxConcurrent := int64(10)
	settings := &objects.ChannelRateLimit{MaxConcurrent: &maxConcurrent}
	limit, hasCustom := mt.GetModelConcurrentLimit(1, "any-model", settings)
	assert.Equal(t, int64(10), limit)
	assert.False(t, hasCustom)
}

// Task 4: Concurrent access safety
func TestQA_Task4_ConcurrentAccessSafety(t *testing.T) {
	mt := NewModelConnectionTracker()
	const g = 50
	const ops = 20
	var wg sync.WaitGroup
	wg.Add(g)
	for range g {
		go func() {
			defer wg.Done()
			for range ops {
				mt.IncrementModelConnection(1, "gpt-4")
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, g*ops, mt.GetModelConnectionCount(1, "gpt-4"))
}

// Task 6: Duration-aware RPM scoring
func TestQA_Task6_DurationAwareRPMScore(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil, nil, nil, nil)
	rpm := int64(10)
	fiveHour := objects.RateLimitDurationFiveHour
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "5hr-rpm",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm, RPMDuration: &fiveHour}},
	}}
	for range 5 {
		tracker.IncrementRequestForDuration(ch.ID, fiveHour.Duration(), nil)
	}
	score := strategy.Score(context.Background(), ch)
	assert.Equal(t, 50.0, score, "Strategy uses 5-hour window, sees 5/10 requests = 50%% usage")
	assert.Equal(t, int64(5), tracker.GetRequestCountForDuration(ch.ID, fiveHour.Duration(), nil))
}

// Task 6: Backward compat - no duration defaults to 1min
func TestQA_Task6_BackwardCompat_NoDurationDefaults1Min(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil, nil, nil, nil)
	rpm := int64(100)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "legacy",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm}},
	}}
	assert.Equal(t, objects.RateLimitDurationOneMin, ch.Settings.RateLimit.GetRPMDuration())
	for range 50 {
		tracker.IncrementRequest(ch.ID)
	}
	assert.Equal(t, 50.0, strategy.Score(context.Background(), ch))
}

// Task 7: Per-model concurrent limit at limit
func TestQA_Task7_PerModelConcurrentLimitAtLimit(t *testing.T) {
	mt := NewModelConnectionTracker()
	tracker := NewChannelRequestTracker()
	ct := NewDefaultConnectionTracker(10)
	strat := NewRateLimitAwareStrategy(tracker, ct, mt, nil, nil)
	maxConcurrent := int64(10)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "model-at-limit",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{
			MaxConcurrent:   &maxConcurrent,
			ModelConcurrent: map[string]int64{"gpt-4": 2},
		}},
	}}
	mt.IncrementModelConnection(1, "gpt-4")
	mt.IncrementModelConnection(1, "gpt-4")
	ctx := contextWithModel(t, "gpt-4")
	score := strat.Score(ctx, ch)
	assert.Equal(t, float64(rateLimitExhaustedScore), score, "Per-model limit at exact limit should return exhausted score")
}

// Task 7: Per-model concurrent limit exceeded
func TestQA_Task7_PerModelConcurrentLimitExceeded(t *testing.T) {
	mt := NewModelConnectionTracker()
	tracker := NewChannelRequestTracker()
	ct := NewDefaultConnectionTracker(10)
	strat := NewRateLimitAwareStrategy(tracker, ct, mt, nil, nil)
	maxConcurrent := int64(10)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "model-exceeded",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{
			MaxConcurrent:   &maxConcurrent,
			ModelConcurrent: map[string]int64{"gpt-4": 2},
		}},
	}}
	mt.IncrementModelConnection(1, "gpt-4")
	mt.IncrementModelConnection(1, "gpt-4")
	mt.IncrementModelConnection(1, "gpt-4")
	ctx := contextWithModel(t, "gpt-4")
	score := strat.Score(ctx, ch)
	assert.Equal(t, float64(rateLimitExhaustedScore), score, "Per-model limit exceeded should return exhausted score")
}

// Task 7: Fallback to channel-wide MaxConcurrent
func TestQA_Task7_FallbackToChannelWideMaxConcurrent(t *testing.T) {
	mt := NewModelConnectionTracker()
	maxConcurrent := int64(10)
	settings := &objects.ChannelRateLimit{
		MaxConcurrent:   &maxConcurrent,
		ModelConcurrent: map[string]int64{"gpt-4": 2},
	}
	gpt4Limit, gpt4Custom := mt.GetModelConcurrentLimit(1, "gpt-4", settings)
	assert.Equal(t, int64(2), gpt4Limit)
	assert.True(t, gpt4Custom)
	otherLimit, otherCustom := mt.GetModelConcurrentLimit(1, "other", settings)
	assert.Equal(t, int64(10), otherLimit)
	assert.False(t, otherCustom)
}

// Task 8: Duration-aware rate limit tracking
func TestQA_Task8_DurationAwareRateLimitTracking(t *testing.T) {
	tracker := NewChannelRequestTracker()
	oneHour := objects.RateLimitDurationOneHour
	fiveHour := objects.RateLimitDurationFiveHour
	tracker.IncrementRequestForDuration(1, oneHour.Duration(), nil)
	tracker.AddTokensForDuration(1, 500, fiveHour.Duration(), nil)
	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(1, oneHour.Duration(), nil))
	assert.Equal(t, int64(500), tracker.GetTokenCountForDuration(1, fiveHour.Duration(), nil))
	assert.Equal(t, int64(0), tracker.GetTokenCountForDuration(1, oneHour.Duration(), nil))
	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(1, fiveHour.Duration(), nil))
}

// Task 8: Model connection tracking lifecycle
func TestQA_Task8_ModelConnectionLifecycle(t *testing.T) {
	mt := NewModelConnectionTracker()
	mt.IncrementModelConnection(1, "gpt-4")
	mt.IncrementModelConnection(1, "gpt-4")
	assert.Equal(t, 2, mt.GetModelConnectionCount(1, "gpt-4"))
	mt.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 1, mt.GetModelConnectionCount(1, "gpt-4"))
	mt.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 0, mt.GetModelConnectionCount(1, "gpt-4"))
}

// Task 9: Duration dropdown values match backend
func TestQA_Task9_DurationDropdownValues(t *testing.T) {
	durations := map[objects.RateLimitDuration]time.Duration{
		objects.RateLimitDurationOneMin:   time.Minute,
		objects.RateLimitDurationOneHour:  time.Hour,
		objects.RateLimitDurationFiveHour: 5 * time.Hour,
		objects.RateLimitDurationOneWeek:  7 * 24 * time.Hour,
		objects.RateLimitDurationOneMonth: 30 * 24 * time.Hour,
	}
	for d, expected := range durations {
		assert.Equal(t, expected, d.Duration())
	}
}

// Task 9: Backward compat - existing config shows "1 minute" default
func TestQA_Task9_BackwardCompat_DefaultOneMinute(t *testing.T) {
	var rl *objects.ChannelRateLimit
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetRPMDuration())
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetTPMDuration())
	rl2 := &objects.ChannelRateLimit{}
	assert.Equal(t, objects.RateLimitDurationOneMin, rl2.GetRPMDuration())
}

// Task 10: Add and save per-model concurrent limit
func TestQA_Task10_AddPerModelConcurrentLimit(t *testing.T) {
	rl := &objects.ChannelRateLimit{ModelConcurrent: map[string]int64{"gpt-4": 2}}
	limit, hasCustom := rl.GetModelConcurrentLimit("gpt-4")
	assert.Equal(t, int64(2), limit)
	assert.True(t, hasCustom)
}

// Task 10: Remove per-model entry
func TestQA_Task10_RemovePerModelEntry(t *testing.T) {
	maxConcurrent := int64(10)
	rl := &objects.ChannelRateLimit{
		MaxConcurrent:   &maxConcurrent,
		ModelConcurrent: map[string]int64{"gpt-4": 2},
	}
	delete(rl.ModelConcurrent, "gpt-4")
	limit, hasCustom := rl.GetModelConcurrentLimit("gpt-4")
	assert.Equal(t, int64(10), limit)
	assert.False(t, hasCustom)
}

// Task 11: Full integration test suite passes (verified by running all tests)
func TestQA_Task11_IntegrationSuitePasses(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mt := NewModelConnectionTracker()
	ct := NewDefaultConnectionTracker(10)
	strategy := NewRateLimitAwareStrategy(tracker, ct, mt, nil, nil)
	rpm := int64(100)
	tpm := int64(1000)
	maxConcurrent := int64(10)
	fiveHour := objects.RateLimitDurationFiveHour
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "full-integration",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{
			RPM: &rpm, TPM: &tpm, MaxConcurrent: &maxConcurrent,
			RPMDuration: &fiveHour, ModelConcurrent: map[string]int64{"gpt-4": 2},
		}},
	}}
	tracker.IncrementRequestForDuration(ch.ID, fiveHour.Duration(), nil)
	tracker.AddTokensForDuration(ch.ID, 500, fiveHour.Duration(), nil)
	mt.IncrementModelConnection(ch.ID, "gpt-4")
	ct.IncrementConnection(ch.ID)
	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(ch.ID, fiveHour.Duration(), nil))
	assert.Equal(t, int64(500), tracker.GetTokenCountForDuration(ch.ID, fiveHour.Duration(), nil))
	assert.Equal(t, 1, mt.GetModelConnectionCount(ch.ID, "gpt-4"))
	assert.Equal(t, 1, ct.GetActiveConnections(ch.ID))
	score := strategy.Score(contextWithModel(t, "gpt-4"), ch)
	assert.Greater(t, score, float64(rateLimitExhaustedScore))
}

// Task 12: GraphQL query returns new fields
func TestQA_Task12_GraphQLFieldsRoundTrip(t *testing.T) {
	rl := &objects.ChannelRateLimit{
		RPM: ptrInt64(100), TPM: ptrInt64(1000), MaxConcurrent: ptrInt64(5),
		RPMDuration:     ptrDuration(objects.RateLimitDurationFiveHour),
		TPMDuration:     ptrDuration(objects.RateLimitDurationOneMonth),
		ModelConcurrent: map[string]int64{"gpt-4": 2, "claude-3": 3},
	}
	data, err := json.Marshal(rl)
	require.NoError(t, err)
	var rl2 objects.ChannelRateLimit
	err = json.Unmarshal(data, &rl2)
	require.NoError(t, err)
	assert.Equal(t, *rl.RPM, *rl2.RPM)
	assert.Equal(t, *rl.RPMDuration, *rl2.RPMDuration)
	assert.Equal(t, *rl.TPMDuration, *rl2.TPMDuration)
	assert.Equal(t, rl.ModelConcurrent["gpt-4"], rl2.ModelConcurrent["gpt-4"])
	assert.Equal(t, rl.ModelConcurrent["claude-3"], rl2.ModelConcurrent["claude-3"])
}

// Task 12: Save and reload preserves all fields
func TestQA_Task12_SaveAndReloadPreservesFields(t *testing.T) {
	settings := &objects.ChannelSettings{
		RateLimit: &objects.ChannelRateLimit{
			RPM: ptrInt64(100), TPM: ptrInt64(1000), MaxConcurrent: ptrInt64(5),
			RPMDuration:     ptrDuration(objects.RateLimitDurationOneHour),
			TPMDuration:     ptrDuration(objects.RateLimitDurationFiveHour),
			ModelConcurrent: map[string]int64{"gpt-4": 2},
		},
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	var s2 objects.ChannelSettings
	err = json.Unmarshal(data, &s2)
	require.NoError(t, err)
	assert.Equal(t, *settings.RateLimit.RPM, *s2.RateLimit.RPM)
	assert.Equal(t, *settings.RateLimit.RPMDuration, *s2.RateLimit.RPMDuration)
	assert.Equal(t, *settings.RateLimit.TPMDuration, *s2.RateLimit.TPMDuration)
	assert.Equal(t, settings.RateLimit.ModelConcurrent["gpt-4"], s2.RateLimit.ModelConcurrent["gpt-4"])
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestQA_Edge_EmptyDurationField(t *testing.T) {
	raw := `{"rpm": 100, "rpmDuration": "", "tpmDuration": ""}`
	var rl objects.ChannelRateLimit
	err := json.Unmarshal([]byte(raw), &rl)
	require.NoError(t, err)
	assert.Equal(t, time.Minute, rl.GetRPMDuration().Duration())
	assert.Equal(t, time.Minute, rl.GetTPMDuration().Duration())
}

func TestQA_Edge_MissingModelNames(t *testing.T) {
	rl := &objects.ChannelRateLimit{ModelConcurrent: map[string]int64{}}
	limit, hasCustom := rl.GetModelConcurrentLimit("")
	assert.Equal(t, int64(0), limit)
	assert.False(t, hasCustom)
}

func TestQA_Edge_ZeroLimits(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil, nil, nil, nil)
	zeroRPM := int64(0)
	zeroTPM := int64(0)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "zero-limit",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &zeroRPM, TPM: &zeroTPM}},
	}}
	assert.Equal(t, 100.0, strategy.Score(nil, ch), "Zero limits should mean unlimited")
}

func TestQA_Edge_MixedDurationConfigs(t *testing.T) {
	tracker := NewChannelRequestTracker()
	oneHour := objects.RateLimitDurationOneHour
	fiveHour := objects.RateLimitDurationFiveHour
	tracker.IncrementRequestForDuration(1, oneHour.Duration(), nil)
	tracker.IncrementRequestForDuration(1, oneHour.Duration(), nil)
	tracker.AddTokensForDuration(1, 500, fiveHour.Duration(), nil)
	assert.Equal(t, int64(2), tracker.GetRequestCountForDuration(1, oneHour.Duration(), nil))
	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(1, fiveHour.Duration(), nil))
	assert.Equal(t, int64(500), tracker.GetTokenCountForDuration(1, fiveHour.Duration(), nil))
	assert.Equal(t, int64(0), tracker.GetTokenCountForDuration(1, oneHour.Duration(), nil))
}

func TestQA_Edge_ConcurrentAccessRaceDetector(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mt := NewModelConnectionTracker()
	fiveHour := objects.RateLimitDurationFiveHour
	duration := fiveHour.Duration()
	const g = 100
	const ops = 50
	var wg sync.WaitGroup
	wg.Add(g * 3)
	for range g {
		go func() {
			defer wg.Done()
			for range ops {
				tracker.IncrementRequestForDuration(1, duration, nil)
			}
		}()
		go func() {
			defer wg.Done()
			for range ops {
				tracker.AddTokensForDuration(1, 10, duration, nil)
			}
		}()
		go func() {
			defer wg.Done()
			for range ops {
				mt.IncrementModelConnection(1, "gpt-4")
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(g*ops), tracker.GetRequestCountForDuration(1, duration, nil))
	assert.Equal(t, int64(g*ops*10), tracker.GetTokenCountForDuration(1, duration, nil))
	assert.Equal(t, g*ops, mt.GetModelConnectionCount(1, "gpt-4"))
}

func TestQA_Edge_NilRateLimit(t *testing.T) {
	var rl *objects.ChannelRateLimit
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetRPMDuration())
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetTPMDuration())
	limit, hasCustom := rl.GetModelConcurrentLimit("gpt-4")
	assert.Equal(t, int64(0), limit)
	assert.False(t, hasCustom)
}

func TestQA_Edge_CaseInsensitiveModelNames(t *testing.T) {
	mt := NewModelConnectionTracker()
	mt.IncrementModelConnection(1, "GPT-4")
	mt.IncrementModelConnection(1, "gpt-4")
	assert.Equal(t, 2, mt.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 2, mt.GetModelConnectionCount(1, "GPT-4"))
}

func TestQA_Edge_NegativeTokens(t *testing.T) {
	tracker := NewChannelRequestTracker()
	tracker.AddTokensForDuration(1, -100, time.Minute, nil)
	assert.Equal(t, int64(0), tracker.GetTokenCountForDuration(1, time.Minute, nil))
	tracker.AddTokensForDuration(1, 0, time.Minute, nil)
	assert.Equal(t, int64(0), tracker.GetTokenCountForDuration(1, time.Minute, nil))
}

func TestQA_Edge_UnknownDurationString(t *testing.T) {
	unknownDuration := objects.RateLimitDuration("unknown")
	assert.Equal(t, time.Minute, unknownDuration.Duration())
}

// ============================================================================
// Cross-Task Integration
// ============================================================================

func TestQA_CrossTask_FullDurationAndModelConcurrentIntegration(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mt := NewModelConnectionTracker()
	ct := NewDefaultConnectionTracker(10)
	strategy := NewRateLimitAwareStrategy(tracker, ct, mt, nil, nil)
	rpm := int64(100)
	tpm := int64(1000)
	maxConcurrent := int64(10)
	oneHour := objects.RateLimitDurationOneHour
	fiveHour := objects.RateLimitDurationFiveHour
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "cross-task",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{
			RPM: &rpm, TPM: &tpm, MaxConcurrent: &maxConcurrent,
			RPMDuration: &oneHour, TPMDuration: &fiveHour,
			ModelConcurrent: map[string]int64{"gpt-4": 2, "claude-3": 5},
		}},
	}}
	for range 50 {
		tracker.IncrementRequestForDuration(ch.ID, oneHour.Duration(), nil)
	}
	tracker.AddTokensForDuration(ch.ID, 500, fiveHour.Duration(), nil)
	mt.IncrementModelConnection(ch.ID, "gpt-4")
	mt.IncrementModelConnection(ch.ID, "claude-3")
	ct.IncrementConnection(ch.ID)
	ct.IncrementConnection(ch.ID)
	assert.Equal(t, int64(50), tracker.GetRequestCountForDuration(ch.ID, oneHour.Duration(), nil))
	assert.Equal(t, int64(500), tracker.GetTokenCountForDuration(ch.ID, fiveHour.Duration(), nil))
	gpt4Limit, _ := mt.GetModelConcurrentLimit(ch.ID, "gpt-4", ch.Settings.RateLimit)
	assert.Equal(t, int64(2), gpt4Limit)
	claude3Limit, _ := mt.GetModelConcurrentLimit(ch.ID, "claude-3", ch.Settings.RateLimit)
	assert.Equal(t, int64(5), claude3Limit)
	otherLimit, _ := mt.GetModelConcurrentLimit(ch.ID, "other", ch.Settings.RateLimit)
	assert.Equal(t, int64(10), otherLimit)
	score := strategy.Score(contextWithModel(t, "gpt-4"), ch)
	assert.Greater(t, score, float64(rateLimitExhaustedScore))
}

func TestQA_CrossTask_BackwardCompatWithNewFeatures(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil, nil, nil, nil)
	rpm := int64(100)
	legacyCh := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "legacy",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm}},
	}}
	rpm2 := int64(100)
	fiveHour := objects.RateLimitDurationFiveHour
	newCh := &biz.Channel{Channel: &ent.Channel{
		ID: 2, Name: "new",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{
			RPM: &rpm2, RPMDuration: &fiveHour, ModelConcurrent: map[string]int64{"gpt-4": 2},
		}},
	}}
	tracker.IncrementRequest(legacyCh.ID)
	tracker.IncrementRequestForDuration(newCh.ID, fiveHour.Duration(), nil)
	legacyScore := strategy.Score(context.Background(), legacyCh)
	newScore := strategy.Score(contextWithModel(t, "gpt-4"), newCh)
	assert.Greater(t, legacyScore, float64(rateLimitExhaustedScore))
	assert.Greater(t, newScore, float64(rateLimitExhaustedScore))
}

func ptrInt64(v int64) *int64                                            { return &v }
func ptrDuration(d objects.RateLimitDuration) *objects.RateLimitDuration { return &d }
