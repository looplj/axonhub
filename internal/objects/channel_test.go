package objects

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitDuration_Duration(t *testing.T) {
	tests := []struct {
		name     RateLimitDuration
		expected time.Duration
	}{
		{RateLimitDurationOneMin, time.Minute},
		{RateLimitDurationOneHour, time.Hour},
		{RateLimitDurationFiveHour, 5 * time.Hour},
		{RateLimitDurationOneWeek, 7 * 24 * time.Hour},
		{RateLimitDurationOneMonth, 30 * 24 * time.Hour},
		{"unknown", time.Minute}, // Default to 1 minute
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.name.Duration())
		})
	}
}

func TestChannelRateLimit_GetRPMDuration(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var rl *ChannelRateLimit
		assert.Equal(t, RateLimitDurationOneMin, rl.GetRPMDuration())
	})

	t.Run("nil RPMDuration field", func(t *testing.T) {
		rl := &ChannelRateLimit{
			RPM: int64Ptr(100),
		}
		assert.Equal(t, RateLimitDurationOneMin, rl.GetRPMDuration())
	})

	t.Run("5 hour duration", func(t *testing.T) {
		duration := RateLimitDurationFiveHour
		rl := &ChannelRateLimit{
			RPM:         int64Ptr(100),
			RPMDuration: &duration,
		}
		assert.Equal(t, RateLimitDurationFiveHour, rl.GetRPMDuration())
	})

	t.Run("1 month duration", func(t *testing.T) {
		duration := RateLimitDurationOneMonth
		rl := &ChannelRateLimit{
			RPM:         int64Ptr(100),
			RPMDuration: &duration,
		}
		assert.Equal(t, RateLimitDurationOneMonth, rl.GetRPMDuration())
	})
}

func TestChannelRateLimit_GetTPMDuration(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var rl *ChannelRateLimit
		assert.Equal(t, RateLimitDurationOneMin, rl.GetTPMDuration())
	})

	t.Run("nil TPMDuration field", func(t *testing.T) {
		rl := &ChannelRateLimit{
			TPM: int64Ptr(1000),
		}
		assert.Equal(t, RateLimitDurationOneMin, rl.GetTPMDuration())
	})

	t.Run("1 hour duration", func(t *testing.T) {
		duration := RateLimitDurationOneHour
		rl := &ChannelRateLimit{
			TPM:         int64Ptr(1000),
			TPMDuration: &duration,
		}
		assert.Equal(t, RateLimitDurationOneHour, rl.GetTPMDuration())
	})
}

func TestChannelRateLimit_GetModelConcurrentLimit(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var rl *ChannelRateLimit
		limit, hasCustom := rl.GetModelConcurrentLimit("gpt-4")
		assert.Equal(t, int64(0), limit)
		assert.False(t, hasCustom)
	})

	t.Run("empty settings", func(t *testing.T) {
		rl := &ChannelRateLimit{}
		limit, hasCustom := rl.GetModelConcurrentLimit("gpt-4")
		assert.Equal(t, int64(0), limit)
		assert.False(t, hasCustom)
	})

	t.Run("max concurrent only", func(t *testing.T) {
		rl := &ChannelRateLimit{
			MaxConcurrent: int64Ptr(100),
		}
		limit, hasCustom := rl.GetModelConcurrentLimit("gpt-4")
		assert.Equal(t, int64(100), limit)
		assert.False(t, hasCustom)
	})

	t.Run("per-model limit", func(t *testing.T) {
		rl := &ChannelRateLimit{
			MaxConcurrent: int64Ptr(100),
			ModelConcurrent: map[string]int64{
				"gpt-4": 50,
			},
		}
		limit, hasCustom := rl.GetModelConcurrentLimit("gpt-4")
		assert.Equal(t, int64(50), limit)
		assert.True(t, hasCustom)
	})

	t.Run("per-model limit case insensitive", func(t *testing.T) {
		rl := &ChannelRateLimit{
			MaxConcurrent: int64Ptr(100),
			ModelConcurrent: map[string]int64{
				"gpt-4": 50,
			},
		}
		limit, hasCustom := rl.GetModelConcurrentLimit("GPT-4")
		assert.Equal(t, int64(50), limit)
		assert.True(t, hasCustom)
	})

	t.Run("fallback to max concurrent when model not in map", func(t *testing.T) {
		rl := &ChannelRateLimit{
			MaxConcurrent: int64Ptr(100),
			ModelConcurrent: map[string]int64{
				"gpt-4": 50,
			},
		}
		limit, hasCustom := rl.GetModelConcurrentLimit("claude-3")
		assert.Equal(t, int64(100), limit)
		assert.False(t, hasCustom)
	})

	t.Run("model concurrent without max concurrent", func(t *testing.T) {
		rl := &ChannelRateLimit{
			ModelConcurrent: map[string]int64{
				"gpt-4": 50,
			},
		}
		limit, hasCustom := rl.GetModelConcurrentLimit("gpt-4")
		assert.Equal(t, int64(50), limit)
		assert.True(t, hasCustom)

		// Fallback for unknown model should return 0
		limit, hasCustom = rl.GetModelConcurrentLimit("claude-3")
		assert.Equal(t, int64(0), limit)
		assert.False(t, hasCustom)
	})
}

func TestChannelRateLimit_JSONSerialization(t *testing.T) {
	t.Run("backward compatibility - existing config without duration", func(t *testing.T) {
		jsonData := `{"rpm": 100, "tpm": 1000, "maxConcurrent": 10}`
		var rl ChannelRateLimit
		err := json.Unmarshal([]byte(jsonData), &rl)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), *rl.RPM)
		assert.Equal(t, int64(1000), *rl.TPM)
		assert.Equal(t, int64(10), *rl.MaxConcurrent)
		assert.Nil(t, rl.RPMDuration)
		assert.Nil(t, rl.TPMDuration)
		assert.Nil(t, rl.ModelConcurrent)
		// Default duration should be 1 minute
		assert.Equal(t, RateLimitDurationOneMin, rl.GetRPMDuration())
		assert.Equal(t, RateLimitDurationOneMin, rl.GetTPMDuration())
	})

	t.Run("new config with duration fields", func(t *testing.T) {
		jsonData := `{"rpm": 100, "rpmDuration": "5hr", "tpm": 1000, "tpmDuration": "1mo"}`
		var rl ChannelRateLimit
		err := json.Unmarshal([]byte(jsonData), &rl)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), *rl.RPM)
		assert.Equal(t, RateLimitDurationFiveHour, *rl.RPMDuration)
		assert.Equal(t, int64(1000), *rl.TPM)
		assert.Equal(t, RateLimitDurationOneMonth, *rl.TPMDuration)
	})

	t.Run("config with model concurrent", func(t *testing.T) {
		jsonData := `{"maxConcurrent": 10, "modelConcurrent": {"gpt-4": 5, "claude-3": 3}}`
		var rl ChannelRateLimit
		err := json.Unmarshal([]byte(jsonData), &rl)
		assert.NoError(t, err)
		assert.Equal(t, int64(10), *rl.MaxConcurrent)
		assert.Equal(t, int64(5), rl.ModelConcurrent["gpt-4"])
		assert.Equal(t, int64(3), rl.ModelConcurrent["claude-3"])
	})

	t.Run("full serialization roundtrip", func(t *testing.T) {
		rpmDuration := RateLimitDurationFiveHour
		tpmDuration := RateLimitDurationOneHour
		original := ChannelRateLimit{
			RPM:             int64Ptr(100),
			TPM:             int64Ptr(1000),
			MaxConcurrent:   int64Ptr(10),
			RPMDuration:     &rpmDuration,
			TPMDuration:     &tpmDuration,
			ModelConcurrent: map[string]int64{"gpt-4": 5},
		}

		data, err := json.Marshal(original)
		assert.NoError(t, err)

		var decoded ChannelRateLimit
		err = json.Unmarshal(data, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, *original.RPM, *decoded.RPM)
		assert.Equal(t, *original.TPM, *decoded.TPM)
		assert.Equal(t, *original.MaxConcurrent, *decoded.MaxConcurrent)
		assert.Equal(t, *original.RPMDuration, *decoded.RPMDuration)
		assert.Equal(t, *original.TPMDuration, *decoded.TPMDuration)
		assert.Equal(t, original.ModelConcurrent, decoded.ModelConcurrent)
	})
}

func int64Ptr(i int64) *int64 {
	return &i
}

// Test RateLimitDuration MarshalGQL/UnmarshalGQL

func TestRateLimitDuration_MarshalGQL(t *testing.T) {
	tests := []struct {
		name  RateLimitDuration
		wantW string
	}{
		{RateLimitDurationOneMin, `"ONE_MIN"`},
		{RateLimitDurationOneHour, `"ONE_HOUR"`},
		{RateLimitDurationFiveHour, `"FIVE_HOUR"`},
		{RateLimitDurationOneWeek, `"ONE_WEEK"`},
		{RateLimitDurationOneMonth, `"ONE_MONTH"`},
		{"unknown", `"ONE_MIN"`}, // default fallback
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			w := &bytes.Buffer{}
			tt.name.MarshalGQL(w)

			gotW := w.String()
			assert.Equal(t, tt.wantW, gotW)
		})
	}
}

func TestRateLimitDuration_UnmarshalGQL(t *testing.T) {
	tests := []struct {
		name    string
		v       any
		want    RateLimitDuration
		wantErr bool
	}{
		{"ONE_MIN", "ONE_MIN", RateLimitDurationOneMin, false},
		{"ONE_HOUR", "ONE_HOUR", RateLimitDurationOneHour, false},
		{"FIVE_HOUR", "FIVE_HOUR", RateLimitDurationFiveHour, false},
		{"ONE_WEEK", "ONE_WEEK", RateLimitDurationOneWeek, false},
		{"ONE_MONTH", "ONE_MONTH", RateLimitDurationOneMonth, false},
		{"invalid string", "invalid", RateLimitDuration(""), true},
		{"wrong type", 123, RateLimitDuration(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d RateLimitDuration
			err := d.UnmarshalGQL(tt.v)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, RateLimitDuration(""), d)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, d)
			}
		})
	}
}

func TestRateLimitDuration_IsValid(t *testing.T) {
	tests := []struct {
		name      RateLimitDuration
		wantValid bool
	}{
		{RateLimitDurationOneMin, true},
		{RateLimitDurationOneHour, true},
		{RateLimitDurationFiveHour, true},
		{RateLimitDurationOneMonth, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			assert.Equal(t, tt.wantValid, tt.name.IsValid())
		})
	}
}

func TestRateLimitDuration_UnmarshalJSON(t *testing.T) {
	t.Run("accepts Go constants", func(t *testing.T) {
		jsonData := []byte(`"1min"`)
		var d RateLimitDuration
		err := json.Unmarshal(jsonData, &d)

		require.NoError(t, err)
		assert.Equal(t, RateLimitDurationOneMin, d)
	})

	t.Run("accepts GraphQL enum names", func(t *testing.T) {
		jsonData := []byte(`"ONE_MIN"`)
		var d RateLimitDuration
		err := json.Unmarshal(jsonData, &d)

		require.NoError(t, err)
		assert.Equal(t, RateLimitDurationOneMin, d)
	})

	t.Run("accepts all Go constants", func(t *testing.T) {
		constants := []string{"1min", "1hr", "5hr", "1wk", "1mo"}
		expected := []RateLimitDuration{
			RateLimitDurationOneMin,
			RateLimitDurationOneHour,
			RateLimitDurationFiveHour,
			RateLimitDurationOneWeek,
			RateLimitDurationOneMonth,
		}

		for i, s := range constants {
			t.Run(s, func(t *testing.T) {
				jsonData := []byte(`"` + s + `"`)
				var d RateLimitDuration
				err := json.Unmarshal(jsonData, &d)

				require.NoError(t, err)
				assert.Equal(t, expected[i], d)
			})
		}
	})

	t.Run("accepts all GraphQL enum names", func(t *testing.T) {
		enums := []string{"ONE_MIN", "ONE_HOUR", "FIVE_HOUR", "ONE_WEEK", "ONE_MONTH"}
		expected := []RateLimitDuration{
			RateLimitDurationOneMin,
			RateLimitDurationOneHour,
			RateLimitDurationFiveHour,
			RateLimitDurationOneWeek,
			RateLimitDurationOneMonth,
		}

		for i, s := range enums {
			t.Run(s, func(t *testing.T) {
				jsonData := []byte(`"` + s + `"`)
				var d RateLimitDuration
				err := json.Unmarshal(jsonData, &d)

				require.NoError(t, err)
				assert.Equal(t, expected[i], d)
			})
		}
	})

	t.Run("rejects invalid values", func(t *testing.T) {
		jsonData := []byte(`"2hr"`)
		var d RateLimitDuration
		err := json.Unmarshal(jsonData, &d)

		assert.Error(t, err)
	})

	t.Run("rejects random invalid strings", func(t *testing.T) {
		jsonData := []byte(`"invalid"`)
		var d RateLimitDuration
		err := json.Unmarshal(jsonData, &d)

		assert.Error(t, err)
	})

	t.Run("backward compatibility - existing database format", func(t *testing.T) {
		// This simulates old JSON data stored in the database
		jsonData := `{"rpmDuration": "1min", "tpmDuration": "1hr"}`
		var rl ChannelRateLimit
		err := json.Unmarshal([]byte(jsonData), &rl)

		require.NoError(t, err)
		assert.Equal(t, RateLimitDurationOneMin, *rl.RPMDuration)
		assert.Equal(t, RateLimitDurationOneHour, *rl.TPMDuration)
	})
}

func TestComputeWindowStart_NilAnchor(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	result := ComputeWindowStart(now, time.Hour, nil)
	assert.Equal(t, time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_ZeroTimeAnchor(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	zeroTime := time.Time{}
	result := ComputeWindowStart(now, time.Hour, &zeroTime)
	assert.Equal(t, time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_NegativeDuration(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, -1*time.Hour, &anchor)
	assert.Equal(t, now.Truncate(-1*time.Hour), result)
}

func TestComputeWindowStart_ZeroDuration(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, 0, &anchor)
	assert.Equal(t, now.Truncate(0), result)
}

func TestComputeWindowStart_FutureAnchor(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	futureAnchor := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, time.Hour, &futureAnchor)
	assert.Equal(t, time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_FutureAnchor_FiveHourWindow(t *testing.T) {
	now := time.Date(2024, 1, 15, 1, 48, 0, 0, time.UTC)
	futureAnchor := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, 5*time.Hour, &futureAnchor)
	assert.Equal(t, time.Date(2024, 1, 15, 1, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_PastAnchor_ExactStep(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, time.Hour, &anchor)
	assert.Equal(t, time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_PastAnchor_PartialStep(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, time.Hour, &anchor)
	assert.Equal(t, time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_PastAnchor_FiveHourWindow(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 30, 0, 0, time.UTC)
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, 5*time.Hour, &anchor)
	assert.Equal(t, time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_AnchorAtStepBoundary(t *testing.T) {
	now := time.Date(2024, 1, 15, 5, 0, 0, 0, time.UTC)
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, 5*time.Hour, &anchor)
	assert.Equal(t, time.Date(2024, 1, 15, 5, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_AnchorJustBeforeStepBoundary(t *testing.T) {
	now := time.Date(2024, 1, 15, 4, 59, 59, 0, time.UTC)
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, 5*time.Hour, &anchor)
	assert.Equal(t, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_AnchorMidnight_OneMinuteWindow(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, time.Minute, &anchor)
	assert.Equal(t, time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_TruncateFallback(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC)
	nilResult := ComputeWindowStart(now, time.Hour, nil)
	truncateResult := now.Truncate(time.Hour)
	assert.Equal(t, truncateResult, nilResult)
}

func TestComputeWindowStart_WeeklyWindow(t *testing.T) {
	now := time.Date(2024, 1, 17, 10, 30, 0, 0, time.UTC) // Wed
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC) // Mon
	result := ComputeWindowStart(now, 7*24*time.Hour, &anchor)
	// 2 days 10.5 hours elapsed, 0 steps of 7 days → anchor + 0*7d = Mon Jan 15
	assert.Equal(t, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), result)
}

func TestComputeWindowStart_MonthlyWindow(t *testing.T) {
	now := time.Date(2024, 2, 10, 10, 30, 0, 0, time.UTC)
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeWindowStart(now, 30*24*time.Hour, &anchor)
	assert.Equal(t, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), result)
}
