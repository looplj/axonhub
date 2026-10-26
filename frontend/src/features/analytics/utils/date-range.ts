import {
  buildDateRangeWhereClause,
  DEFAULT_END_TIME,
  DEFAULT_START_TIME,
  type DateTimeRangeValue,
  type TimeValue,
} from '@/utils/date-range';

function toTimeValue(date: Date): TimeValue {
  const pad = (value: number) => String(value).padStart(2, '0');
  return { hh: pad(date.getHours()), mm: pad(date.getMinutes()), ss: pad(date.getSeconds()) };
}

function parseTimestamp(value?: string | null): Date | undefined {
  if (!value) return undefined;
  const date = new Date(value);

  return Number.isNaN(date.getTime()) ? undefined : date;
}

/** Local midnight of the given day, used by the quick range presets. */
export function startOfLocalDay(date: Date): Date {
  const day = new Date(date);
  day.setHours(0, 0, 0, 0);

  return day;
}

/**
 * Converts the filter timestamps into the value shape used by the shared
 * date range picker, so the selected range can be edited with second precision.
 */
export function timestampsToDateTimeRange(startTime?: string | null, endTime?: string | null): DateTimeRangeValue | undefined {
  const start = parseTimestamp(startTime);
  const end = parseTimestamp(endTime);

  if (!start && !end) return undefined;

  // Keep each side independent: filling a missing bound with the other one
  // would turn an open-ended filter into a single-day range on the next confirm.
  return {
    from: start ? startOfLocalDay(start) : undefined,
    to: end ? startOfLocalDay(end) : undefined,
    startTime: start ? toTimeValue(start) : DEFAULT_START_TIME,
    endTime: end ? toTimeValue(end) : DEFAULT_END_TIME,
  };
}

/**
 * Converts a picked date range into RFC3339 timestamps for the API.
 * The start is inclusive at the first millisecond of the selected second and
 * the end is inclusive at its last millisecond.
 */
export function dateTimeRangeToTimestamps(range?: DateTimeRangeValue): {
  startTime: string | null;
  endTime: string | null;
} {
  const { createdAtGTE, createdAtLTE } = buildDateRangeWhereClause(range);

  return { startTime: createdAtGTE ?? null, endTime: createdAtLTE ?? null };
}
