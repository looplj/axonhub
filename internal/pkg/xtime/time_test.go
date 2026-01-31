package xtime

import (
	"testing"
	"time"
)

func TestGetCalendarPeriods(t *testing.T) {
	// Helper function to create time in specific location
	makeTime := func(year, month, day, hour, min, sec int, loc *time.Location) time.Time {
		return time.Date(year, time.Month(month), day, hour, min, sec, 0, loc)
	}

	tests := []struct {
		name              string
		mockNow           time.Time
		location          *time.Location
		wantTodayStart    time.Time
		wantTodayEnd      time.Time
		wantWeekStart     time.Time
		wantWeekEnd       time.Time
		wantLastWeekStart time.Time
		wantLastWeekEnd   time.Time
		wantMonthStart    time.Time
		wantMonthEnd      time.Time
	}{
		{
			name:     "Wednesday in UTC",
			mockNow:  time.Date(2024, 1, 17, 14, 30, 0, 0, time.UTC), // Wednesday
			location: time.UTC,
			// Today: 2024-01-17 00:00:00 to 2024-01-18 00:00:00
			wantTodayStart: time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC),
			wantTodayEnd:   time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC),
			// This week: Monday 2024-01-15 to Monday 2024-01-22
			wantWeekStart: makeTime(2024, 1, 15, 0, 0, 0, time.UTC),
			wantWeekEnd:   makeTime(2024, 1, 22, 0, 0, 0, time.UTC),
			// Last week: Monday 2024-01-08 to Monday 2024-01-15
			wantLastWeekStart: makeTime(2024, 1, 8, 0, 0, 0, time.UTC),
			wantLastWeekEnd:   makeTime(2024, 1, 15, 0, 0, 0, time.UTC),
			// This month: Jan 1 to Feb 1
			wantMonthStart: makeTime(2024, 1, 1, 0, 0, 0, time.UTC),
			wantMonthEnd:   makeTime(2024, 2, 1, 0, 0, 0, time.UTC),
		},
		{
			name:              "Monday (start of week) in UTC",
			mockNow:           time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // Monday
			location:          time.UTC,
			wantTodayStart:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			wantTodayEnd:      time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
			wantWeekStart:     makeTime(2024, 1, 15, 0, 0, 0, time.UTC),
			wantWeekEnd:       makeTime(2024, 1, 22, 0, 0, 0, time.UTC),
			wantLastWeekStart: makeTime(2024, 1, 8, 0, 0, 0, time.UTC),
			wantLastWeekEnd:   makeTime(2024, 1, 15, 0, 0, 0, time.UTC),
			wantMonthStart:    makeTime(2024, 1, 1, 0, 0, 0, time.UTC),
			wantMonthEnd:      makeTime(2024, 2, 1, 0, 0, 0, time.UTC),
		},
		{
			name:              "Sunday (end of week) in UTC",
			mockNow:           time.Date(2024, 1, 21, 23, 59, 59, 0, time.UTC), // Sunday
			location:          time.UTC,
			wantTodayStart:    time.Date(2024, 1, 21, 0, 0, 0, 0, time.UTC),
			wantTodayEnd:      time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC),
			wantWeekStart:     makeTime(2024, 1, 15, 0, 0, 0, time.UTC),
			wantWeekEnd:       makeTime(2024, 1, 22, 0, 0, 0, time.UTC),
			wantLastWeekStart: makeTime(2024, 1, 8, 0, 0, 0, time.UTC),
			wantLastWeekEnd:   makeTime(2024, 1, 15, 0, 0, 0, time.UTC),
			wantMonthStart:    makeTime(2024, 1, 1, 0, 0, 0, time.UTC),
			wantMonthEnd:      makeTime(2024, 2, 1, 0, 0, 0, time.UTC),
		},
		{
			name:              "First day of month",
			mockNow:           time.Date(2024, 3, 1, 8, 0, 0, 0, time.UTC),
			location:          time.UTC,
			wantTodayStart:    time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			wantTodayEnd:      time.Date(2024, 3, 2, 0, 0, 0, 0, time.UTC),
			wantWeekStart:     makeTime(2024, 2, 26, 0, 0, 0, time.UTC), // Monday
			wantWeekEnd:       makeTime(2024, 3, 4, 0, 0, 0, time.UTC),
			wantLastWeekStart: makeTime(2024, 2, 19, 0, 0, 0, time.UTC),
			wantLastWeekEnd:   makeTime(2024, 2, 26, 0, 0, 0, time.UTC),
			wantMonthStart:    makeTime(2024, 3, 1, 0, 0, 0, time.UTC),
			wantMonthEnd:      makeTime(2024, 4, 1, 0, 0, 0, time.UTC),
		},
		{
			name:              "Last day of month",
			mockNow:           time.Date(2024, 2, 29, 20, 0, 0, 0, time.UTC), // Leap year
			location:          time.UTC,
			wantTodayStart:    time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			wantTodayEnd:      time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			wantWeekStart:     makeTime(2024, 2, 26, 0, 0, 0, time.UTC),
			wantWeekEnd:       makeTime(2024, 3, 4, 0, 0, 0, time.UTC),
			wantLastWeekStart: makeTime(2024, 2, 19, 0, 0, 0, time.UTC),
			wantLastWeekEnd:   makeTime(2024, 2, 26, 0, 0, 0, time.UTC),
			wantMonthStart:    makeTime(2024, 2, 1, 0, 0, 0, time.UTC),
			wantMonthEnd:      makeTime(2024, 3, 1, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the current time
			setUTCNowFunc(func() time.Time {
				return tt.mockNow
			})

			defer resetUTCNowFunc()

			got := GetCalendarPeriods(tt.location)

			// Check Today period
			if !got.Today.Start.Equal(tt.wantTodayStart) {
				t.Errorf("Today.Start = %v, want %v", got.Today.Start, tt.wantTodayStart)
			}

			if !got.Today.End.Equal(tt.wantTodayEnd) {
				t.Errorf("Today.End = %v, want %v", got.Today.End, tt.wantTodayEnd)
			}

			// Check ThisWeek period
			if !got.ThisWeek.Start.Equal(tt.wantWeekStart) {
				t.Errorf("ThisWeek.Start = %v, want %v", got.ThisWeek.Start, tt.wantWeekStart)
			}

			if !got.ThisWeek.End.Equal(tt.wantWeekEnd) {
				t.Errorf("ThisWeek.End = %v, want %v", got.ThisWeek.End, tt.wantWeekEnd)
			}

			// Check LastWeek period
			if !got.LastWeek.Start.Equal(tt.wantLastWeekStart) {
				t.Errorf("LastWeek.Start = %v, want %v", got.LastWeek.Start, tt.wantLastWeekStart)
			}

			if !got.LastWeek.End.Equal(tt.wantLastWeekEnd) {
				t.Errorf("LastWeek.End = %v, want %v", got.LastWeek.End, tt.wantLastWeekEnd)
			}

			// Check ThisMonth period
			if !got.ThisMonth.Start.Equal(tt.wantMonthStart) {
				t.Errorf("ThisMonth.Start = %v, want %v", got.ThisMonth.Start, tt.wantMonthStart)
			}

			if !got.ThisMonth.End.Equal(tt.wantMonthEnd) {
				t.Errorf("ThisMonth.End = %v, want %v", got.ThisMonth.End, tt.wantMonthEnd)
			}
		})
	}
}

func TestGetCalendarPeriodsWithLocation(t *testing.T) {
	// Test with different time zones
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("Cannot load Asia/Shanghai timezone: %v", err)
	}

	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("Cannot load America/New_York timezone: %v", err)
	}

	// Mock time: 2024-01-17 14:30:00 UTC (Wednesday)
	// In Shanghai: 2024-01-17 22:30:00 (same day)
	// In New York: 2024-01-17 09:30:00 (same day)
	mockNow := time.Date(2024, 1, 17, 14, 30, 0, 0, time.UTC)

	setUTCNowFunc(func() time.Time {
		return mockNow
	})

	defer resetUTCNowFunc()

	// Test Shanghai
	periodsShanghai := GetCalendarPeriods(shanghai)

	// In Shanghai, it should still be 2024-01-17
	wantTodayStartShanghai := time.Date(2024, 1, 17, 0, 0, 0, 0, shanghai).UTC()
	if !periodsShanghai.Today.Start.Equal(wantTodayStartShanghai) {
		t.Errorf("Shanghai Today.Start = %v, want %v", periodsShanghai.Today.Start, wantTodayStartShanghai)
	}

	// Test New York
	periodsNY := GetCalendarPeriods(newYork)

	// In New York, it should still be 2024-01-17
	wantTodayStartNY := time.Date(2024, 1, 17, 0, 0, 0, 0, newYork).UTC()
	if !periodsNY.Today.Start.Equal(wantTodayStartNY) {
		t.Errorf("NY Today.Start = %v, want %v", periodsNY.Today.Start, wantTodayStartNY)
	}
}

func TestPeriodHalfOpenInterval(t *testing.T) {
	// Test that periods are half-open intervals [Start, End)
	mockNow := time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC)

	setUTCNowFunc(func() time.Time {
		return mockNow
	})

	defer resetUTCNowFunc()

	periods := GetCalendarPeriods(time.UTC)

	// Today should include the exact start time but not the end time
	if !periods.Today.Start.Equal(mockNow) {
		t.Errorf("Today should start at exactly 00:00:00")
	}

	// The period should contain the start but not the end
	testTime := periods.Today.Start
	if testTime.Before(periods.Today.Start) || !testTime.Before(periods.Today.End) {
		t.Errorf("Start time should be within the period")
	}

	if !periods.Today.End.After(periods.Today.Start) {
		t.Errorf("End should be after Start")
	}
}
