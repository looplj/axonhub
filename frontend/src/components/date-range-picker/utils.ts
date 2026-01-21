import { format } from 'date-fns'
import type { DateTimeRangeValue, TimeValue } from '@/utils/date-range'
import { DEFAULT_END_TIME, DEFAULT_START_TIME } from '@/utils/date-range'

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

export function formatRange(from?: Date, to?: Date, placeholder = '') {
  if (!from && !to) return placeholder
  if (from && !to) return `${format(from, 'MMM d, yyyy')} - ...`
  return `${format(from!, 'MMM d, yyyy')} - ${format(to!, 'MMM d, yyyy')}`
}

export function timeToString(value: TimeValue) {
  return `${value.hh}:${value.mm}:${value.ss}`
}

export function isSameTime(left: TimeValue, right: TimeValue) {
  return left.hh === right.hh && left.mm === right.mm && left.ss === right.ss
}

export function addMonthsSafe(date: Date, months: number) {
  const next = new Date(date)
  next.setMonth(next.getMonth() + months)
  return next
}
