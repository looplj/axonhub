import * as React from 'react'
import { format } from 'date-fns'
import { Calendar, ChevronLeft, ChevronRight, Clock } from 'lucide-react'
import { DayPicker, type DateRange } from 'react-day-picker'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import type { DateTimeRangeValue } from '@/utils/date-range'
import { DEFAULT_END_TIME, DEFAULT_START_TIME } from '@/utils/date-range'
import { TimeDropdown } from './time-dropdown'
import {
  addMonthsSafe,
  defaultDateTimeRangeValue,
  formatRange,
  isSameTime,
  normalizeDateTimeRangeValue,
  timeToString,
} from './utils'

export interface DateTimeRangePickerProps {
  value?: DateTimeRangeValue
  onChange?: (next: DateTimeRangeValue | undefined) => void
  onCancel?: () => void
  onConfirm?: (next: DateTimeRangeValue) => void
  className?: string
}

const WEEKDAYS = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']

function useClickOutside(ref: React.RefObject<HTMLElement>, onOutside: () => void) {
  React.useEffect(() => {
    function handler(event: MouseEvent) {
      const target = event.target as Node
      if (!ref.current) return
      if (!ref.current.contains(target)) onOutside()
    }

    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [ref, onOutside])
}

export function DateTimeRangePicker(props: DateTimeRangePickerProps) {
  const { value, onChange, onCancel, onConfirm, className } = props
  const { t } = useTranslation()
  const isControlled = Object.prototype.hasOwnProperty.call(props, 'value')
  const [internal, setInternal] = React.useState<DateTimeRangeValue>(() => normalizeDateTimeRangeValue(value))

  React.useEffect(() => {
    if (!isControlled) return
    setInternal(normalizeDateTimeRangeValue(value))
  }, [isControlled, value])

  const emit = React.useCallback(
    (next: DateTimeRangeValue) => {
      if (!isControlled) setInternal(next)
      onChange?.(next)
    },
    [isControlled, onChange]
  )

  const handleReset = React.useCallback(() => {
    const next = defaultDateTimeRangeValue()
    if (!isControlled) setInternal(next)
    onChange?.(undefined)
  }, [isControlled, onChange])

  const range: DateRange | undefined =
    internal.from || internal.to ? { from: internal.from, to: internal.to } : undefined

  const [month, setMonth] = React.useState<Date>(() => internal.from ?? new Date())
  React.useEffect(() => {
    if (internal.from) setMonth(internal.from)
  }, [internal.from])

  const [startOpen, setStartOpen] = React.useState(false)
  const [endOpen, setEndOpen] = React.useState(false)
  const startWrapRef = React.useRef<HTMLDivElement>(null)
  const endWrapRef = React.useRef<HTMLDivElement>(null)
  const closeStart = React.useCallback(() => setStartOpen(false), [])
  const closeEnd = React.useCallback(() => setEndOpen(false), [])
  useClickOutside(startWrapRef, closeStart)
  useClickOutside(endWrapRef, closeEnd)

  const toggleStart = React.useCallback(() => {
    setStartOpen((open) => {
      const next = !open
      if (next) setEndOpen(false)
      return next
    })
  }, [])

  const toggleEnd = React.useCallback(() => {
    setEndOpen((open) => {
      const next = !open
      if (next) setStartOpen(false)
      return next
    })
  }, [])

  const headerText = React.useMemo(
    () => formatRange(internal.from, internal.to, t('common.filters.dateRange')),
    [internal.from, internal.to, t]
  )

  const startActive = startOpen || !isSameTime(internal.startTime, DEFAULT_START_TIME)
  const endActive = endOpen || !isSameTime(internal.endTime, DEFAULT_END_TIME)

  const timeInputClass = (active: boolean) =>
    cn(
      'w-full cursor-pointer border-none bg-transparent p-0 text-sm transition-colors focus:ring-0',
      active
        ? 'font-semibold text-gray-900 dark:text-gray-100'
        : 'font-medium text-gray-500 dark:text-gray-600'
    )

  const timeIconClass = (active: boolean) =>
    cn('ml-2 h-5 w-5 transition-colors', active ? 'text-gray-600 dark:text-gray-300' : 'text-gray-400 dark:text-gray-600')

  return (
    <div
      className={cn(
        'w-full max-w-[720px] overflow-visible rounded-[24px] border border-gray-200 bg-white shadow-2xl dark:border-white/5 dark:bg-[#121214]',
        className
      )}
    >
      <div className='flex items-center justify-between border-b border-gray-100 bg-white p-5 dark:border-white/5 dark:bg-[#0a0a0b]/50'>
        <div className='flex items-center gap-3 rounded-md border border-gray-200 bg-gray-50 px-4 py-2 text-sm font-medium text-gray-700 dark:border-white/10 dark:bg-white/5 dark:text-gray-300'>
          <Calendar className='h-4 w-4 opacity-70' />
          <span>{headerText}</span>
        </div>

        <div className='flex gap-1 text-gray-500'>
          <button
            type='button'
            className='rounded-full p-2 transition-all hover:bg-gray-100 dark:hover:bg-white/5'
            onClick={() => setMonth((m) => addMonthsSafe(m, -1))}
          >
            <ChevronLeft className='h-5 w-5' />
          </button>
          <button
            type='button'
            className='rounded-full p-2 transition-all hover:bg-gray-100 dark:hover:bg-white/5'
            onClick={() => setMonth((m) => addMonthsSafe(m, 1))}
          >
            <ChevronRight className='h-5 w-5' />
          </button>
        </div>
      </div>

      <div className='flex flex-col gap-10 bg-white p-8 dark:bg-[#0a0a0b] md:flex-row'>
        <div className='flex-1'>
          <DayPicker
            mode='range'
            selected={range}
            onSelect={(next) => {
              emit({
                ...internal,
                from: next?.from,
                to: next?.to,
              })
            }}
            month={month}
            onMonthChange={setMonth}
            numberOfMonths={2}
            showOutsideDays
            fixedWeeks
            weekStartsOn={0}
            classNames={{
              nav: 'hidden',
              months: 'flex gap-10',
              month: 'w-full',
              month_caption:
                'mb-6 text-center text-sm font-semibold tracking-wide text-gray-900 dark:text-gray-100',
              month_grid: 'w-full border-collapse',
              weekdays: 'grid grid-cols-7 text-center',
              weekday:
                'pb-4 text-[10px] font-bold uppercase tracking-[0.15em] text-gray-400 dark:text-gray-500',
              week: 'grid grid-cols-7 text-center',
              day: cn(
                'p-0 text-center',
                '[&.outside>button]:text-gray-300 dark:[&.outside>button]:text-gray-700',
                '[&.disabled>button]:cursor-not-allowed [&.disabled>button]:opacity-30',
                '[&.range_start>button]:bg-primary [&.range_start>button]:text-white',
                '[&.range_end>button]:bg-primary [&.range_end>button]:text-white',
                '[&.range_middle>button]:bg-primary/10 dark:[&.range_middle>button]:bg-primary/20',
                '[&.range_middle>button]:text-gray-700 dark:[&.range_middle>button]:text-gray-200',
                '[&.today>button]:ring-1 [&.today>button]:ring-primary/40'
              ),
              outside: 'outside',
              disabled: 'disabled',
              range_start: 'range_start',
              range_end: 'range_end',
              range_middle: 'range_middle',
              today: 'today',
              day_button: cn(
                'inline-flex h-8 w-8 items-center justify-center rounded-full text-sm transition-colors',
                'text-gray-700 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-white/5'
              ),
            }}
            formatters={{
              formatWeekdayName: (date) => WEEKDAYS[date.getDay()],
            }}
            components={{
              MonthCaption: ({ calendarMonth, className, displayIndex: _displayIndex, ...monthProps }) => (
                <div className={className} {...monthProps}>
                  {format(calendarMonth.date, 'MMMM yyyy')}
                </div>
              ),
            }}
          />
        </div>
      </div>

      <div className='border-t border-gray-100 bg-gray-50 px-8 py-10 dark:border-white/5 dark:bg-[#0a0a0b]/80'>
        <div className='flex flex-col gap-10 md:flex-row'>
          <div className='flex-1 space-y-3'>
            <label className='block text-[11px] font-bold uppercase tracking-[0.2em] text-gray-500'>
              {t('common.filters.startTime')}
            </label>

            <div className='relative' ref={startWrapRef}>
              <button
                type='button'
                className={cn(
                  'flex w-full items-center rounded-md border border-gray-200 bg-white px-4 py-3.5 transition-all hover:border-white/20',
                  'dark:border-white/10 dark:bg-[#121214]/60',
                  startOpen && 'active-glow'
                )}
                onClick={toggleStart}
              >
                <input
                  readOnly
                  className={timeInputClass(startActive)}
                  value={timeToString(internal.startTime)}
                  placeholder='HH:mm:ss'
                />
                <Clock className={timeIconClass(startActive)} />
              </button>

              {startOpen && (
                <TimeDropdown
                  value={internal.startTime}
                  onChange={(next) => emit({ ...internal, startTime: next })}
                  onClose={closeStart}
                  closeLabel={t('common.close')}
                />
              )}
            </div>
          </div>

          <div className='flex-1 space-y-3'>
            <label className='block text-[11px] font-bold uppercase tracking-[0.2em] text-gray-500'>
              {t('common.filters.endTime')}
            </label>

            <div className='relative' ref={endWrapRef}>
              <button
                type='button'
                className={cn(
                  'flex w-full items-center rounded-md border border-gray-200 bg-white px-4 py-3.5 transition-all hover:border-white/20',
                  'dark:border-white/10 dark:bg-[#121214]/60',
                  endOpen && 'active-glow'
                )}
                onClick={toggleEnd}
              >
                <input
                  readOnly
                  className={timeInputClass(endActive)}
                  value={timeToString(internal.endTime)}
                  placeholder='HH:mm:ss'
                />
                <Clock className={timeIconClass(endActive)} />
              </button>

              {endOpen && (
                <TimeDropdown
                  value={internal.endTime}
                  onChange={(next) => emit({ ...internal, endTime: next })}
                  onClose={closeEnd}
                  closeLabel={t('common.close')}
                />
              )}
            </div>
          </div>
        </div>
      </div>

      <div className='flex items-center justify-between border-t border-gray-100 bg-white px-8 py-6 dark:border-white/5 dark:bg-[#0a0a0b]'>
        <button
          type='button'
          className='rounded-md text-[11px] font-semibold uppercase tracking-widest text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-200'
          onClick={handleReset}
        >
          {t('common.filters.reset')}
        </button>

        <div className='flex gap-4'>
          <button
            type='button'
            className='rounded-md px-6 py-2.5 text-sm font-semibold text-gray-600 transition-all hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-white/5'
            onClick={onCancel}
          >
            {t('common.buttons.cancel')}
          </button>
          <button
            type='button'
            className='rounded-md bg-primary px-8 py-2.5 text-sm font-semibold text-white shadow-xl shadow-primary/20 transition-all active:scale-[0.98]'
            onClick={() => onConfirm?.(internal)}
          >
            {t('common.buttons.confirm')}
          </button>
        </div>
      </div>
    </div>
  )
}
