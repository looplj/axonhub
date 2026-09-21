export type CoarseTimeWindow = 'allTime' | 'month' | 'week' | 'day';

const LABEL_KEYS: Record<CoarseTimeWindow, string> = {
  day: 'timeRange.today',
  week: 'timeRange.last7Days',
  month: 'timeRange.last30Days',
  allTime: 'timeRange.allTime',
};

export function coarseWindowLabelKey(timeWindow: CoarseTimeWindow): string {
  return LABEL_KEYS[timeWindow];
}
