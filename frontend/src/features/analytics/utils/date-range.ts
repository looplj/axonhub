import {
  buildDateRangeWhereClause,
  DEFAULT_END_TIME,
  DEFAULT_START_TIME,
  type DateTimeRangeValue,
  type TimeValue,
} from '@/utils/date-range';

interface RangeBounds {
  range: DateTimeRangeValue | undefined;
  /**
   * Original timestamps. The picker only edits local wall-clock fields, which
   * cannot distinguish the two instants of a fall-back DST hour and drops the
   * sub-second part, so an untouched bound is reused verbatim instead of being
   * rebuilt.
   */
  startTime?: string;
  endTime?: string;
}

function toTimeValue(date: Date): TimeValue {
  const pad = (value: number) => String(value).padStart(2, '0');
  return { hh: pad(date.getHours()), mm: pad(date.getMinutes()), ss: pad(date.getSeconds()) };
}

function parseTimestamp(value?: string | null): Date | undefined {
  if (!value) return undefined;
  const date = new Date(value);

  return Number.isNaN(date.getTime()) ? undefined : date;
}

function sameTimeValue(left: TimeValue, right: TimeValue): boolean {
  return left.hh === right.hh && left.mm === right.mm && left.ss === right.ss;
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
 * The sub-second component is returned alongside because the picker only edits
 * whole seconds; dropping it would silently widen the stored range on confirm.
 */
export function timestampsToDateTimeRange(startTime?: string | null, endTime?: string | null): RangeBounds {
  const start = parseTimestamp(startTime);
  const end = parseTimestamp(endTime);

  if (!start && !end) return { range: undefined };

  // Keep each side independent: filling a missing bound with the other one
  // would turn an open-ended filter into a single-day range on the next confirm.
  return {
    range: {
      from: start ? startOfLocalDay(start) : undefined,
      to: end ? startOfLocalDay(end) : undefined,
      startTime: start ? toTimeValue(start) : DEFAULT_START_TIME,
      endTime: end ? toTimeValue(end) : DEFAULT_END_TIME,
    },
    startTime: start?.toISOString(),
    endTime: end?.toISOString(),
  };
}

/**
 * Converts a picked date range into RFC3339 timestamps for the API.
 * The start is inclusive at the first millisecond of the selected second and
 * the end is inclusive at its last millisecond.
 *
 * When the picked range still matches the stored one, the stored sub-second
 * components are reused so confirming an untouched range is a no-op. A side
 * whose time was edited falls back to the `.000`/`.999` boundaries.
 */
export function dateTimeRangeToTimestamps(
  range: DateTimeRangeValue | undefined,
  previous?: RangeBounds
): {
  startTime: string | null;
  endTime: string | null;
} {
  const { createdAtGTE, createdAtLTE } = buildDateRangeWhereClause(range);

  const previousRange = previous?.range;

  // A side counts as untouched when its day and its whole-second time still
  // match; only then is its stored sub-second component carried over.
  const isUnchanged = (
    previousDay: Date | undefined,
    day: Date | undefined,
    previousTime: TimeValue | undefined,
    time: TimeValue | undefined
  ): boolean => !!previousDay && !!day && !!previousTime && !!time && previousDay.getTime() === day.getTime() && sameTimeValue(previousTime, time);

  const startUnchanged = isUnchanged(previousRange?.from, range?.from, previousRange?.startTime, range?.startTime);

  const endUnchanged = isUnchanged(previousRange?.to, range?.to, previousRange?.endTime, range?.endTime);

  return {
    startTime: startUnchanged ? (previous?.startTime ?? null) : (createdAtGTE ?? null),
    endTime: endUnchanged ? (previous?.endTime ?? null) : (createdAtLTE ?? null),
  };
}
