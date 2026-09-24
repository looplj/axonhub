package gql

import (
	"testing"
	"time"

	"entgo.io/ent/dialect"
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

	t.Run("hourly spans collapse the hour a fall-back transition repeats", func(t *testing.T) {
		// Europe/Berlin falls back on 2026-10-25 at 03:00, so 02:00 happens twice.
		berlin, err := time.LoadLocation("Europe/Berlin")
		assert.NoError(t, err)

		dstStart := time.Date(2026, 10, 25, 0, 0, 0, 0, berlin)
		got := bucketSequence(dstStart, dstStart.Add(5*time.Hour), qb.ResolutionHour)

		assert.Equal(t, []string{
			"2026-10-25 00:00",
			"2026-10-25 01:00",
			"2026-10-25 02:00", // the repeated hour counts once
			"2026-10-25 03:00",
		}, got)
	})

	t.Run("an empty span yields nothing", func(t *testing.T) {
		assert.Empty(t, bucketSequence(start, start, qb.ResolutionDay))
		assert.Empty(t, bucketSequence(start, start, qb.ResolutionHour))
	})
}

func TestLocalHourFloorPreservesFallbackOffset(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	assert.NoError(t, err)

	firstOccurrence := time.Date(2026, 11, 1, 1, 30, 0, 0, newYork)
	secondOccurrence := firstOccurrence.Add(time.Hour)
	floored := localHourFloor(secondOccurrence)

	assert.Equal(t, secondOccurrence.Add(-30*time.Minute), floored)
	_, wantOffset := secondOccurrence.Zone()
	_, gotOffset := floored.Zone()
	assert.Equal(t, wantOffset, gotOffset)
}

// A relative window starts on a local wall-clock hour. Truncate(time.Hour) rounds the
// absolute duration since the zero time, so in a zone whose offset is not a whole hour it
// lands on :30 instead of :00 — which both mislabels the first bucket and shifts the
// sequence far enough to leave the current hour's usage out of the response.
func TestRelativeWindowStartsOnALocalHour(t *testing.T) {
	kolkata, err := time.LoadLocation("Asia/Kolkata") // UTC+05:30
	assert.NoError(t, err)

	now := time.Date(2026, 9, 23, 13, 15, 30, 0, kolkata)
	since := now.Add(-24 * time.Hour)

	assert.Equal(t, time.Date(2026, 9, 22, 13, 0, 0, 0, kolkata), localHourFloor(since))
	assert.Equal(t, 0, localHourFloor(since).Minute())

	labels := bucketSequence(localHourFloor(since), now, qb.ResolutionHour)
	assert.Equal(t, 25, len(labels))
	assert.Contains(t, labels, now.Format("2006-01-02 15:00"))

	t.Run("the whole-hour zones keep the same shape", func(t *testing.T) {
		// Fixed rather than wall-clock: at exactly the top of an hour the current bucket
		// has no data yet, so the exclusive upper bound legitimately drops it.
		utcNow := time.Date(2026, 9, 23, 13, 15, 30, 0, time.UTC)
		labels := bucketSequence(localHourFloor(utcNow.Add(-24*time.Hour)), utcNow, qb.ResolutionHour)
		assert.Equal(t, 25, len(labels))
		assert.Contains(t, labels, utcNow.Format("2006-01-02 15:00"))
	})
}

// The two builders serve different column types, and mixing them up is not a style issue:
// a native timestamp column through the epoch builder produces to_timestamp(timestamptz) on
// Postgres, which does not exist, and NULL on SQLite.
func TestDateExpressionBuildersServeDifferentColumnTypes(t *testing.T) {
	const epochCol = "channel_probes.timestamp"
	const timestampCol = "usage_logs.created_at"

	epoch := buildEpochDateExpression(dialect.Postgres, epochCol, 0, "UTC", qb.ResolutionDay)
	assert.Contains(t, epoch, "to_timestamp", "the epoch builder must convert from epoch seconds")

	native := qb.GetDateExpression(dialect.Postgres, timestampCol, "UTC", 0, qb.ResolutionDay)
	assert.Contains(t, native, "AT TIME ZONE")
	assert.NotContains(t, native, "to_timestamp", "a timestamptz column must not be wrapped in to_timestamp")

	for _, d := range []string{dialect.SQLite, dialect.MySQL, dialect.Postgres} {
		epoch := buildEpochDateExpression(d, epochCol, 0, "UTC", qb.ResolutionDay)
		native := qb.GetDateExpression(d, timestampCol, "UTC", 0, qb.ResolutionDay)
		assert.NotEqual(t, epoch, native, "%s: the two column types need different expressions", d)
	}
}
