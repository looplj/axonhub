export type TimeValue = { hh: string; mm: string; ss: string }

export type DateTimeRangeValue = {
  from?: Date
  to?: Date
  startTime: TimeValue
  endTime: TimeValue
}

export const DEFAULT_START_TIME: TimeValue = { hh: '00', mm: '00', ss: '00' }
export const DEFAULT_END_TIME: TimeValue = { hh: '23', mm: '59', ss: '59' }

function clampTime(value: string, max: number, fallback: number) {
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed)) {
    return fallback
  }
  return Math.min(Math.max(parsed, 0), max)
}

export function buildDateRangeWhereClause(dateRange: DateTimeRangeValue | undefined) {
  const where: { createdAtGTE?: string; createdAtLTE?: string } = {}

  if (dateRange?.from) {
    const startDate = new Date(dateRange.from)
    const startTime = dateRange.startTime ?? DEFAULT_START_TIME
    startDate.setHours(
      clampTime(startTime.hh, 23, 0),
      clampTime(startTime.mm, 59, 0),
      clampTime(startTime.ss, 59, 0),
      0
    )
    where.createdAtGTE = startDate.toISOString()
  }
  if (dateRange?.to) {
    const endDate = new Date(dateRange.to)
    const endTime = dateRange.endTime ?? DEFAULT_END_TIME
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
