package xtime

import "time"

func UTCNow() time.Time {
	return time.Now().UTC()
}

var utcNowFunc = UTCNow

// setUTCNowFunc sets the function used to get current UTC time.
// This is primarily used for testing to mock the current time.
func setUTCNowFunc(f func() time.Time) {
	utcNowFunc = f
}

// resetUTCNowFunc resets the UTC now function to the default implementation.
// This should be called in test cleanup to avoid affecting other tests.
func resetUTCNowFunc() {
	utcNowFunc = UTCNow
}

// Period represents a time period with Start (inclusive) and End (exclusive)
// The period is a half-open interval [Start, End).
type Period struct {
	Start time.Time
	End   time.Time
}

// CalendarPeriods represents calendar-based time periods for statistics.
type CalendarPeriods struct {
	// Today: [00:00:00 today, 00:00:00 tomorrow)
	Today Period

	// ThisWeek: [Monday 00:00:00 this week, Monday 00:00:00 next week)
	ThisWeek Period

	// LastWeek: [Monday 00:00:00 last week, Monday 00:00:00 this week)
	LastWeek Period

	// ThisMonth: [1st day 00:00:00 this month, 1st day 00:00:00 next month)
	ThisMonth Period
}

// GetCalendarPeriods returns calendar-based time periods for the given location
// This is useful for dashboard statistics that need calendar-aligned periods
// rather than rolling windows (e.g., "this week" = Monday to now, not last 7 days).
func GetCalendarPeriods(loc *time.Location) CalendarPeriods {
	nowLocal := utcNowFunc().In(loc)

	// Today: [00:00:00, 00:00:00 next day)
	todayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	todayEnd := todayStart.AddDate(0, 0, 1)

	// This week: [Monday 00:00:00, next Monday 00:00:00)
	weekday := int(nowLocal.Weekday())
	if weekday == 0 {
		weekday = 7 // Treat Sunday as 7 for Monday-based week
	}

	thisWeekStart := todayStart.AddDate(0, 0, -(weekday - 1))
	thisWeekEnd := thisWeekStart.AddDate(0, 0, 7)

	// Last week: [Monday 00:00:00, Monday 00:00:00 of this week)
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)
	lastWeekEnd := thisWeekStart

	// This month: [1st day 00:00:00, 1st day of next month 00:00:00)
	thisMonthStart := time.Date(nowLocal.Year(), nowLocal.Month(), 1, 0, 0, 0, 0, loc)
	thisMonthEnd := thisMonthStart.AddDate(0, 1, 0)

	return CalendarPeriods{
		Today: Period{
			Start: todayStart.UTC(),
			End:   todayEnd.UTC(),
		},
		ThisWeek: Period{
			Start: thisWeekStart.UTC(),
			End:   thisWeekEnd.UTC(),
		},
		LastWeek: Period{
			Start: lastWeekStart.UTC(),
			End:   lastWeekEnd.UTC(),
		},
		ThisMonth: Period{
			Start: thisMonthStart.UTC(),
			End:   thisMonthEnd.UTC(),
		},
	}
}
