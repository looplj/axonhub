export type TimeValue = { hh: string; mm: string; ss: string }

export type DateTimeRangeValue = {
  from?: Date
  to?: Date
  startTime: TimeValue
  endTime: TimeValue
}

export const DEFAULT_START_TIME: TimeValue = { hh: '00', mm: '00', ss: '00' }
export const DEFAULT_END_TIME: TimeValue = { hh: '23', mm: '59', ss: '59' }

export function normalizeDateTimeRangeValue(value?: DateTimeRangeValue): DateTimeRangeValue {
  return {
    from: value?.from,
    to: value?.to,
    startTime: { ...DEFAULT_START_TIME, ...value?.startTime },
    endTime: { ...DEFAULT_END_TIME, ...value?.endTime },
  }
}

export function defaultDateTimeRangeValue() {
  return normalizeDateTimeRangeValue()
}

export function isSameTime(left: TimeValue, right: TimeValue) {
  return left.hh === right.hh && left.mm === right.mm && left.ss === right.ss
}

function clampTime(value: string, max: number, fallback: number) {
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed)) {
    return fallback
  }
  return Math.min(Math.max(parsed, 0), max)
}

export function buildDateRangeWhereClause(dateRange: DateTimeRangeValue | undefined) {
  const where: { createdAtGTE?: string; createdAtLTE?: string } = {}

  const normalized = normalizeDateTimeRangeValue(dateRange)

  if (normalized.from) {
    const startDate = new Date(normalized.from)
    const startTime = normalized.startTime ?? DEFAULT_START_TIME
    startDate.setHours(
      clampTime(startTime.hh, 23, 0),
      clampTime(startTime.mm, 59, 0),
      clampTime(startTime.ss, 59, 0),
      0
    )
    where.createdAtGTE = startDate.toISOString()
  }
  if (normalized.to) {
    const endDate = new Date(normalized.to)
    const endTime = normalized.endTime ?? DEFAULT_END_TIME
    endDate.setHours(
      clampTime(endTime.hh, 23, 23),
      clampTime(endTime.mm, 59, 59),
      clampTime(endTime.ss, 59, 59),
      999
    )
    where.createdAtLTE = endDate.toISOString()
  }

  return where
}
