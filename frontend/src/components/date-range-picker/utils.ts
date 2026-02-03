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

function clampTimeSegment(value: string | undefined, max: number, fallback: string) {
  if (value === undefined) return fallback
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed)) return fallback
  const clamped = Math.min(Math.max(parsed, 0), max)
  return String(clamped).padStart(2, '0')
}

export function parseTimeString(input: string, fallback: TimeValue): TimeValue | null {
  const trimmed = input.trim()
  if (!trimmed) return null
  const match = trimmed.match(/^(\d{1,2})(?::(\d{1,2}))?(?::(\d{1,2}))?$/)
  if (!match) return null
  return {
    hh: clampTimeSegment(match[1], 23, fallback.hh),
    mm: clampTimeSegment(match[2], 59, fallback.mm),
    ss: clampTimeSegment(match[3], 59, fallback.ss),
  }
}

export function addMonthsSafe(date: Date, months: number) {
  const next = new Date(date)
  const day = next.getDate()
  next.setDate(1)
  next.setMonth(next.getMonth() + months)
  const lastDay = new Date(next.getFullYear(), next.getMonth() + 1, 0).getDate()
  next.setDate(Math.min(day, lastDay))
  return next
}
