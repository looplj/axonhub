/** Coarse time window, the string the dashboard stat queries accept for timeWindow. */
export type CoarseTimeWindow = 'allTime' | 'month' | 'week' | 'day' | 'last24Hours';

/** Window whose boundary the backend computes from the instant the query runs, so it
 * cannot be written as a calendar date pair. */
export type RelativeTimeWindow = 'last24Hours';

const LABEL_KEYS: Record<CoarseTimeWindow, string> = {
  day: 'timeRange.today',
  // week is ThisWeek.Start on the backend (Monday), not a rolling seven days, so the
  // badge has to say "this week" rather than repeat what the user picked.
  week: 'timeRange.thisWeek',
  month: 'timeRange.thisMonth',
  last24Hours: 'timeRange.last24Hours',
  allTime: 'timeRange.allTime',
};

/** Coarse window to i18n label key. */
export function coarseWindowLabelKey(timeWindow: CoarseTimeWindow): string {
  return LABEL_KEYS[timeWindow];
}

/** 'YYYY-MM-DD' to local midnight. Presets and the backend parseDateStr both take
 * boundaries by local calendar day, so this has to share that semantics or the day
 * count drifts across timezones. */
function parseLocalDate(dateStr: string): Date {
  const [y, m, d] = dateStr.split('-').map(Number);
  return new Date(y, m - 1, d);
}

/** Local midnight of the given day, dropping the time of day. */
function localMidnight(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

/** Calendar days spanned, both endpoints included. Timestamps cannot be subtracted
 * directly: a missing end date resolves to "now", so a range starting today counts
 * as 2 days past noon and picks the wrong window. */
export function inclusiveCalendarDays(startTime: string, endTime: string | null): number {
  const start = parseLocalDate(startTime);
  const end = endTime ? parseLocalDate(endTime) : localMidnight(new Date());
  return Math.round((end.getTime() - start.getTime()) / 86_400_000) + 1;
}

/** Closest coarse window supported by the dashboard stat queries, which accept
 * day/week/month/allTime and nothing longer than a month. */
export function coarseWindowFromRange(
  startTime: string | null,
  endTime: string | null,
  timeWindow?: RelativeTimeWindow
): CoarseTimeWindow {
  if (timeWindow) return timeWindow;

  if (!startTime) return 'allTime';

  const days = inclusiveCalendarDays(startTime, endTime);

  if (days <= 1) return 'day';
  if (days <= 7) return 'week';
  if (days <= 31) return 'month';
  return 'allTime';
}

/** Closest coarse window for the throughput queries, which only accept
 * day/week/month and treat anything else as day. */
export function performanceWindowFromRange(
  startTime: string | null,
  endTime: string | null,
  timeWindow?: RelativeTimeWindow
): CoarseTimeWindow {
  if (timeWindow) return timeWindow;

  if (!startTime) return 'month';

  const days = inclusiveCalendarDays(startTime, endTime);

  if (days <= 1) return 'day';
  if (days <= 7) return 'week';
  return 'month';
}
