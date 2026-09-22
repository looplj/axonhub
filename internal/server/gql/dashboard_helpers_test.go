package gql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/server/gql/qb"
)

func TestResolutionForSpan(t *testing.T) {
	tests := []struct {
		name string
		days int
		want qb.DateResolution
	}{
		{name: "single day is hourly", days: 1, want: qb.ResolutionHour},
		{name: "one week is hourly", days: 7, want: qb.ResolutionHour},
		{name: "boundary is hourly", days: hourlyResolutionMaxDays, want: qb.ResolutionHour},
		{name: "past the boundary is daily", days: hourlyResolutionMaxDays + 1, want: qb.ResolutionDay},
		{name: "a month is daily", days: 30, want: qb.ResolutionDay},
		{name: "a year is daily", days: 365, want: qb.ResolutionDay},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolutionForSpan(tt.days))
		})
	}
}

func TestBucketSequence(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 9, 22, 0, 0, 0, 0, loc)
	end := time.Date(2026, 9, 24, 0, 0, 0, 0, loc)

	t.Run("daily spans are inclusive of start and exclusive of end", func(t *testing.T) {
		assert.Equal(t, []string{"2026-09-22", "2026-09-23"}, bucketSequence(start, end, qb.ResolutionDay))
	})

	t.Run("hourly spans label each clock hour", func(t *testing.T) {
		got := bucketSequence(start, start.Add(3*time.Hour), qb.ResolutionHour)
		assert.Equal(t, []string{"2026-09-22 00:00", "2026-09-22 01:00", "2026-09-22 02:00"}, got)
	})

	t.Run("hourly spans roll past midnight", func(t *testing.T) {
		got := bucketSequence(start.Add(23*time.Hour), start.Add(26*time.Hour), qb.ResolutionHour)
		assert.Equal(t, []string{"2026-09-22 23:00", "2026-09-23 00:00", "2026-09-23 01:00"}, got)
	})

	t.Run("hourly spans stay on wall clock across a DST change", func(t *testing.T) {
		// America/New_York springs forward on 2026-03-08 at 02:00.
		ny, err := time.LoadLocation("America/New_York")
		assert.NoError(t, err)

		dstStart := time.Date(2026, 3, 8, 0, 0, 0, 0, ny)
		got := bucketSequence(dstStart, dstStart.Add(5*time.Hour), qb.ResolutionHour)

		assert.Equal(t, []string{
			"2026-03-08 00:00",
			"2026-03-08 01:00",
			"2026-03-08 03:00", // 02:00 does not exist on this date
			"2026-03-08 04:00",
			"2026-03-08 05:00",
		}, got)
	})

	t.Run("an empty span yields nothing", func(t *testing.T) {
		assert.Empty(t, bucketSequence(start, start, qb.ResolutionDay))
		assert.Empty(t, bucketSequence(start, start, qb.ResolutionHour))
	})
}
