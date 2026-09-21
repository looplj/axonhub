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

// 'YYYY-MM-DD' → 本地午夜。预设和后端 parseDateStr 都按本地日历日取边界，
// 这里必须同一套语义，否则跨时区会算错天数。
function parseLocalDate(dateStr: string): Date {
  const [y, m, d] = dateStr.split('-').map(Number);
  return new Date(y, m - 1, d);
}

function localMidnight(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

// 含首含尾的日历天数。不能直接相减时间戳：结束日期缺省时取的是"当前时刻"，
// 过了中午后起始于今天的范围会被算成 2 天，从而选错窗口。
export function inclusiveCalendarDays(startTime: string, endTime: string | null): number {
  const start = parseLocalDate(startTime);
  const end = endTime ? parseLocalDate(endTime) : localMidnight(new Date());
  return Math.round((end.getTime() - start.getTime()) / 86_400_000) + 1;
}
