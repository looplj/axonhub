import * as React from 'react'
import { Calendar, ChevronLeft, ChevronRight } from 'lucide-react'
import { DayPicker, type DateRange } from 'react-day-picker'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useClickOutside } from '@/hooks/use-click-outside'
import { buttonVariants } from '@/components/ui/button'
import type { DateTimeRangeValue } from '@/utils/date-range'
import {
  DEFAULT_END_TIME,
  DEFAULT_START_TIME,
  defaultDateTimeRangeValue,
  isSameTime,
  normalizeDateTimeRangeValue,
} from '@/utils/date-range'
import { dayPickerClassNames, dayPickerComponents, dayPickerFormatters } from './day-picker-config'
import { TimeField } from './time-field'
import { addMonthsSafe, formatRange } from './utils'

export interface DateTimeRangePickerProps {
  value?: DateTimeRangeValue
  onChange?: (next: DateTimeRangeValue | undefined) => void
  onCancel?: () => void
  onConfirm?: (next: DateTimeRangeValue) => void
  className?: string
}

export function DateTimeRangePicker(props: DateTimeRangePickerProps) {
  const { value, onChange, onCancel, onConfirm, className } = props
  const { t } = useTranslation()
  const isControlled = Object.prototype.hasOwnProperty.call(props, 'value')
  const normalizedValue = React.useMemo(() => normalizeDateTimeRangeValue(value), [value])
  const [internal, setInternal] = React.useState<DateTimeRangeValue>(() => normalizedValue)

  React.useEffect(() => {
    if (!isControlled) return
    setInternal(normalizedValue)
  }, [isControlled, normalizedValue])

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

  const [openPanel, setOpenPanel] = React.useState<'start' | 'end' | null>(null)
  const startOpen = openPanel === 'start'
  const endOpen = openPanel === 'end'
  const startWrapRef = React.useRef<HTMLDivElement>(null)
  const endWrapRef = React.useRef<HTMLDivElement>(null)
  const closePanel = React.useCallback(() => setOpenPanel(null), [])
  useClickOutside(startWrapRef, closePanel, startOpen)
  useClickOutside(endWrapRef, closePanel, endOpen)

  const toggleStart = React.useCallback(() => {
    setOpenPanel((current) => (current === 'start' ? null : 'start'))
  }, [])

  const toggleEnd = React.useCallback(() => {
    setOpenPanel((current) => (current === 'end' ? null : 'end'))
  }, [])

  const headerText = React.useMemo(
    () => formatRange(internal.from, internal.to, t('common.filters.dateRange')),
    [internal.from, internal.to, t]
  )

  const startActive = startOpen || !isSameTime(internal.startTime, DEFAULT_START_TIME)
  const endActive = endOpen || !isSameTime(internal.endTime, DEFAULT_END_TIME)

  return (
    <div
      className={cn(
        'w-full max-w-[720px] overflow-visible rounded-[24px] border border-gray-200 bg-white shadow-2xl dark:border-white/5 dark:bg-[#121214]',
        className
      )}
    >
      <div className='flex items-center justify-between border-b border-gray-100 bg-white p-4 dark:border-white/5 dark:bg-[#0a0a0b]/50'>
        <div
          className={cn(
            buttonVariants({ variant: 'outline', size: 'sm' }),
            'h-8 cursor-default border-solid'
          )}
        >
          <Calendar className='h-4 w-4' />
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

      <div className='flex flex-col gap-8 bg-white p-6 dark:bg-[#0a0a0b] md:flex-row'>
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
            classNames={dayPickerClassNames}
            formatters={dayPickerFormatters}
            components={dayPickerComponents}
          />
        </div>
      </div>

      <div className='border-t border-gray-100 bg-gray-50 px-6 py-6 dark:border-white/5 dark:bg-[#0a0a0b]/80'>
        <div className='flex flex-col gap-6 md:flex-row'>
          <TimeField
            label={t('common.filters.startTime')}
            value={internal.startTime}
            active={startActive}
            open={startOpen}
            onToggle={toggleStart}
            onChange={(next) => emit({ ...internal, startTime: next })}
            onClose={closePanel}
            closeLabel={t('common.close')}
            wrapperRef={startWrapRef}
          />

          <TimeField
            label={t('common.filters.endTime')}
            value={internal.endTime}
            active={endActive}
            open={endOpen}
            onToggle={toggleEnd}
            onChange={(next) => emit({ ...internal, endTime: next })}
            onClose={closePanel}
            closeLabel={t('common.close')}
            wrapperRef={endWrapRef}
          />
        </div>
      </div>

      <div className='flex items-center justify-between border-t border-gray-100 bg-white px-6 py-4 dark:border-white/5 dark:bg-[#0a0a0b]'>
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
            className='h-10 min-w-24 rounded-md px-6 text-sm font-semibold text-gray-600 transition-all hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-white/5'
            onClick={onCancel}
          >
            {t('common.buttons.cancel')}
          </button>
          <button
            type='button'
            className='h-10 min-w-24 rounded-md bg-primary px-6 text-sm font-semibold text-white shadow-xl shadow-primary/20 transition-all active:scale-[0.98]'
            onClick={() => onConfirm?.(internal)}
          >
            {t('common.buttons.confirm')}
          </button>
        </div>
      </div>
    </div>
  )
}
