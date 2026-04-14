package biz

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetricsSamplingConfig_Defaults(t *testing.T) {
	config := defaultMetricsSamplingConfig

	require.True(t, config.Enabled, "Enabled should default to true")
	require.False(t, config.AlwaysSample, "AlwaysSample should default to false")
	require.Equal(t, 10, config.RequestRateThreshold, "RequestRateThreshold should default to 10")
	require.Equal(t, 100.0, config.ScoreThreshold, "ScoreThreshold should default to 100")
	require.Equal(t, 5, config.AlternativeCount, "AlternativeCount should default to 5")
	require.Equal(t, 0.20, config.SamplingRate, "SamplingRate should default to 0.20")
}

func TestMetricsSamplingJSON_Marshal(t *testing.T) {
	config := MetricsSamplingConfig{
		Enabled:              true,
		AlwaysSample:         true,
		RequestRateThreshold: 60,
		ScoreThreshold:       150.5,
		AlternativeCount:     10,
		SamplingRate:         0.25,
	}

	jsonBytes, err := json.Marshal(config)
	require.NoError(t, err, "should marshal without error")

	expected := `{"enabled":true,"always_sample":true,"request_rate_threshold":60,"score_threshold":150.5,"alternative_count":10,"sampling_rate":0.25}`
	require.JSONEq(t, expected, string(jsonBytes), "JSON should match expected")
}

func TestMetricsSamplingJSON_Unmarshal(t *testing.T) {
	jsonData := `{"enabled":true,"always_sample":false,"request_rate_threshold":45,"score_threshold":200,"alternative_count":8,"sampling_rate":0.15}`

	var config MetricsSamplingConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	require.NoError(t, err, "should unmarshal without error")

	require.True(t, config.Enabled)
	require.False(t, config.AlwaysSample)
	require.Equal(t, 45, config.RequestRateThreshold)
	require.Equal(t, 200.0, config.ScoreThreshold)
	require.Equal(t, 8, config.AlternativeCount)
	require.Equal(t, 0.15, config.SamplingRate)
}

func TestMetricsSamplingJSON_RoundTrip(t *testing.T) {
	original := MetricsSamplingConfig{
		Enabled:              true,
		AlwaysSample:         false,
		RequestRateThreshold: 25,
		ScoreThreshold:       75.5,
		AlternativeCount:     3,
		SamplingRate:         0.05,
	}

	jsonBytes, err := json.Marshal(original)
	require.NoError(t, err)

	var restored MetricsSamplingConfig
	err = json.Unmarshal(jsonBytes, &restored)
	require.NoError(t, err)

	require.Equal(t, original.Enabled, restored.Enabled)
	require.Equal(t, original.AlwaysSample, restored.AlwaysSample)
	require.Equal(t, original.RequestRateThreshold, restored.RequestRateThreshold)
	require.Equal(t, original.ScoreThreshold, restored.ScoreThreshold)
	require.Equal(t, original.AlternativeCount, restored.AlternativeCount)
	require.Equal(t, original.SamplingRate, restored.SamplingRate)
}
