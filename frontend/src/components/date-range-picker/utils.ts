import { format } from 'date-fns'
import type { TimeValue } from '@/utils/date-range'

export function formatRange(from?: Date, to?: Date, placeholder = '') {
  if (!from && !to) return placeholder
  if (from && !to) return `${format(from, 'MMM d, yyyy')} - ...`
  return `${format(from!, 'MMM d, yyyy')} - ${format(to!, 'MMM d, yyyy')}`
}

export function timeToString(value: TimeValue) {
  return `${value.hh}:${value.mm}:${value.ss}`
}

export function addMonthsSafe(date: Date, months: number) {
  const next = new Date(date)
  next.setMonth(next.getMonth() + months)
  return next
}
