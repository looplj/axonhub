/** Coarse time window, for backends that only accept day/week/month/allTime. */
export type CoarseTimeWindow = 'allTime' | 'month' | 'week' | 'day';

const LABEL_KEYS: Record<CoarseTimeWindow, string> = {
  day: 'timeRange.today',
  week: 'timeRange.last7Days',
  month: 'timeRange.last30Days',
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
export function coarseWindowFromRange(startTime: string | null, endTime: string | null): CoarseTimeWindow {
  if (!startTime) return 'allTime';

  const days = inclusiveCalendarDays(startTime, endTime);

  if (days <= 1) return 'day';
  if (days <= 7) return 'week';
  if (days <= 31) return 'month';
  return 'allTime';
}

/** Closest coarse window for the throughput queries, which only accept
 * day/week/month and treat anything else as day. */
export function performanceWindowFromRange(startTime: string | null, endTime: string | null): CoarseTimeWindow {
  if (!startTime) return 'month';

  const days = inclusiveCalendarDays(startTime, endTime);

  if (days <= 1) return 'day';
  if (days <= 7) return 'week';
  return 'month';
}
