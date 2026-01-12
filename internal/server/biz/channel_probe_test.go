package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShouldRunProbe(t *testing.T) {
	tests := []struct {
		name          string
		frequency     ProbeFrequency
		now           time.Time
		lastExecution time.Time
		expected      bool
	}{
		{
			name:          "1 minute frequency - first execution (zero time)",
			frequency:     ProbeFrequency1Min,
			now:           time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			lastExecution: time.Time{},
			expected:      true,
		},
		{
			name:          "1 minute frequency - same interval",
			frequency:     ProbeFrequency1Min,
			now:           time.Date(2024, time.January, 1, 12, 0, 30, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "1 minute frequency - new interval",
			frequency:     ProbeFrequency1Min,
			now:           time.Date(2024, time.January, 1, 12, 1, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      true,
		},
		{
			name:          "5 minute frequency - same interval",
			frequency:     ProbeFrequency5Min,
			now:           time.Date(2024, time.January, 1, 12, 3, 30, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "5 minute frequency - new interval",
			frequency:     ProbeFrequency5Min,
			now:           time.Date(2024, time.January, 1, 12, 5, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      true,
		},
		{
			name:          "30 minute frequency - same interval",
			frequency:     ProbeFrequency30Min,
			now:           time.Date(2024, time.January, 1, 12, 15, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "30 minute frequency - new interval",
			frequency:     ProbeFrequency30Min,
			now:           time.Date(2024, time.January, 1, 12, 30, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      true,
		},
		{
			name:          "1 hour frequency - same interval",
			frequency:     ProbeFrequency1Hour,
			now:           time.Date(2024, time.January, 1, 12, 30, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "1 hour frequency - new interval",
			frequency:     ProbeFrequency1Hour,
			now:           time.Date(2024, time.January, 1, 13, 0, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      true,
		},
		{
			name:          "1 minute frequency - exact boundary",
			frequency:     ProbeFrequency1Min,
			now:           time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "5 minute frequency - exact boundary",
			frequency:     ProbeFrequency5Min,
			now:           time.Date(2024, time.January, 1, 12, 5, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 5, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "1 minute frequency - within same minute",
			frequency:     ProbeFrequency1Min,
			now:           time.Date(2024, time.January, 1, 12, 0, 59, 999999999, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "1 minute frequency - just crossed boundary",
			frequency:     ProbeFrequency1Min,
			now:           time.Date(2024, time.January, 1, 12, 1, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 59, 999999999, time.UTC),
			expected:      true,
		},
		{
			name:          "5 minute frequency - within same 5 minute window",
			frequency:     ProbeFrequency5Min,
			now:           time.Date(2024, time.January, 1, 12, 4, 59, 999999999, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "5 minute frequency - crossed 5 minute boundary",
			frequency:     ProbeFrequency5Min,
			now:           time.Date(2024, time.January, 1, 12, 5, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 4, 59, 999999999, time.UTC),
			expected:      true,
		},
		{
			name:          "30 minute frequency - within same 30 minute window",
			frequency:     ProbeFrequency30Min,
			now:           time.Date(2024, time.January, 1, 12, 29, 59, 999999999, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "30 minute frequency - crossed 30 minute boundary",
			frequency:     ProbeFrequency30Min,
			now:           time.Date(2024, time.January, 1, 12, 30, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 29, 59, 999999999, time.UTC),
			expected:      true,
		},
		{
			name:          "1 hour frequency - within same hour",
			frequency:     ProbeFrequency1Hour,
			now:           time.Date(2024, time.January, 1, 12, 59, 59, 999999999, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected:      false,
		},
		{
			name:          "1 hour frequency - crossed hour boundary",
			frequency:     ProbeFrequency1Hour,
			now:           time.Date(2024, time.January, 1, 13, 0, 0, 0, time.UTC),
			lastExecution: time.Date(2024, time.January, 1, 12, 59, 59, 999999999, time.UTC),
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldRunProbe(tt.frequency, tt.now, tt.lastExecution)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetIntervalMinutesFromFrequency(t *testing.T) {
	tests := []struct {
		name      string
		frequency ProbeFrequency
		expected  int
	}{
		{
			name:      "1 minute frequency",
			frequency: ProbeFrequency1Min,
			expected:  1,
		},
		{
			name:      "5 minute frequency",
			frequency: ProbeFrequency5Min,
			expected:  5,
		},
		{
			name:      "30 minute frequency",
			frequency: ProbeFrequency30Min,
			expected:  30,
		},
		{
			name:      "1 hour frequency",
			frequency: ProbeFrequency1Hour,
			expected:  60,
		},
		{
			name:      "unknown frequency - defaults to 1 minute",
			frequency: ProbeFrequency("unknown"),
			expected:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIntervalMinutesFromFrequency(tt.frequency)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateTimestamps(t *testing.T) {
	tests := []struct {
		name          string
		setting       ChannelProbeSetting
		currentTime   time.Time
		expectedCount int
		expectedFirst int64
		expectedLast  int64
		expectedStep  int64
	}{
		{
			name: "1 minute frequency - 10 minute range",
			setting: ChannelProbeSetting{
				Enabled:   true,
				Frequency: ProbeFrequency1Min,
			},
			currentTime:   time.Date(2024, time.January, 1, 12, 5, 30, 0, time.UTC),
			expectedCount: 11,
			expectedFirst: time.Date(2024, time.January, 1, 11, 55, 0, 0, time.UTC).Unix(),
			expectedLast:  time.Date(2024, time.January, 1, 12, 5, 0, 0, time.UTC).Unix(),
			expectedStep:  60,
		},
		{
			name: "5 minute frequency - 60 minute range",
			setting: ChannelProbeSetting{
				Enabled:   true,
				Frequency: ProbeFrequency5Min,
			},
			currentTime:   time.Date(2024, time.January, 1, 12, 15, 0, 0, time.UTC),
			expectedCount: 13,
			expectedFirst: time.Date(2024, time.January, 1, 11, 15, 0, 0, time.UTC).Unix(),
			expectedLast:  time.Date(2024, time.January, 1, 12, 15, 0, 0, time.UTC).Unix(),
			expectedStep:  300,
		},
		{
			name: "30 minute frequency - 12 hour range",
			setting: ChannelProbeSetting{
				Enabled:   true,
				Frequency: ProbeFrequency30Min,
			},
			currentTime:   time.Date(2024, time.January, 1, 12, 30, 0, 0, time.UTC),
			expectedCount: 25,
			expectedFirst: time.Date(2024, time.January, 1, 0, 30, 0, 0, time.UTC).Unix(),
			expectedLast:  time.Date(2024, time.January, 1, 12, 30, 0, 0, time.UTC).Unix(),
			expectedStep:  1800,
		},
		{
			name: "1 hour frequency - 24 hour range",
			setting: ChannelProbeSetting{
				Enabled:   true,
				Frequency: ProbeFrequency1Hour,
			},
			currentTime:   time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expectedCount: 25,
			expectedFirst: time.Date(2023, time.December, 31, 12, 0, 0, 0, time.UTC).Unix(),
			expectedLast:  time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC).Unix(),
			expectedStep:  3600,
		},
		{
			name: "1 minute frequency - at exact minute boundary",
			setting: ChannelProbeSetting{
				Enabled:   true,
				Frequency: ProbeFrequency1Min,
			},
			currentTime:   time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expectedCount: 11,
			expectedFirst: time.Date(2024, time.January, 1, 11, 50, 0, 0, time.UTC).Unix(),
			expectedLast:  time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC).Unix(),
			expectedStep:  60,
		},
		{
			name: "unknown frequency - defaults to 1 minute",
			setting: ChannelProbeSetting{
				Enabled:   true,
				Frequency: ProbeFrequency("unknown"),
			},
			currentTime:   time.Date(2024, time.January, 1, 12, 5, 30, 0, time.UTC),
			expectedCount: 11,
			expectedFirst: time.Date(2024, time.January, 1, 11, 55, 0, 0, time.UTC).Unix(),
			expectedLast:  time.Date(2024, time.January, 1, 12, 5, 0, 0, time.UTC).Unix(),
			expectedStep:  60,
		},
		{
			name: "User example - 00:44 at 5 minute frequency",
			setting: ChannelProbeSetting{
				Enabled:   true,
				Frequency: ProbeFrequency5Min,
			},
			currentTime:   time.Date(2024, time.January, 1, 0, 44, 0, 0, time.UTC),
			expectedCount: 13,
			expectedFirst: time.Date(2023, time.December, 31, 23, 40, 0, 0, time.UTC).Unix(),
			expectedLast:  time.Date(2024, time.January, 1, 0, 40, 0, 0, time.UTC).Unix(),
			expectedStep:  300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateTimestamps(tt.setting, tt.currentTime)
			require.Equal(t, tt.expectedCount, len(result))

			if len(result) > 0 {
				require.Equal(t, tt.expectedFirst, result[0])
				require.Equal(t, tt.expectedLast, result[len(result)-1])
			}

			for i := 0; i < len(result)-1; i++ {
				step := result[i+1] - result[i]
				require.Equal(t, tt.expectedStep, step)
			}
		})
	}
}
