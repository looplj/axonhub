import * as React from 'react'
import { format } from 'date-fns'
import { Calendar, ChevronLeft, ChevronRight, Clock } from 'lucide-react'
import { DayPicker, type DateRange } from 'react-day-picker'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import type { DateTimeRangeValue, TimeValue } from '@/utils/date-range'

export type { DateTimeRangeValue, TimeValue } from '@/utils/date-range'

interface DateTimeRangePickerProps {
  value?: DateTimeRangeValue
  onChange?: (next: DateTimeRangeValue | undefined) => void
  onCancel?: () => void
  onConfirm?: (next: DateTimeRangeValue) => void
  className?: string
}

interface DateRangePickerProps {
  value?: DateTimeRangeValue
  onChange?: (range: DateTimeRangeValue | undefined) => void
  onCancel?: () => void
  onConfirm?: (range: DateTimeRangeValue) => void
  className?: string
}

const WEEKDAYS = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']
const DEFAULT_START_TIME: TimeValue = { hh: '09', mm: '30', ss: '00' }
const DEFAULT_END_TIME: TimeValue = { hh: '18', mm: '00', ss: '00' }

function buildNumberList(min: number, max: number) {
  return Array.from({ length: max - min + 1 }, (_, i) => min + i)
}

const DEFAULT_HOURS = buildNumberList(0, 23)
const DEFAULT_MINUTES = buildNumberList(0, 59)
const DEFAULT_SECONDS = buildNumberList(0, 59)

function pad2(n: number | string) {
  return String(n).padStart(2, '0')
}

function timeToString(t: TimeValue) {
  return `${t.hh}:${t.mm}:${t.ss}`
}

function defaultValue(): DateTimeRangeValue {
  return {
    from: undefined,
    to: undefined,
    startTime: { ...DEFAULT_START_TIME },
    endTime: { ...DEFAULT_END_TIME },
  }
}

function formatRange(from?: Date, to?: Date) {
  if (!from && !to) return 'Select range'
  if (from && !to) return `${format(from, 'MMM d, yyyy')} - ...`
  return `${format(from!, 'MMM d, yyyy')} - ${format(to!, 'MMM d, yyyy')}`
}

function addMonthsSafe(d: Date, months: number) {
  const next = new Date(d)
  next.setMonth(next.getMonth() + months)
  return next
}

function TimeDropdown({
  value,
  onChange,
  onClose,
  hours = DEFAULT_HOURS,
  minutes = DEFAULT_MINUTES,
  seconds = DEFAULT_SECONDS,
}: {
  value: TimeValue
  onChange: (next: TimeValue) => void
  onClose?: () => void
  hours?: number[]
  minutes?: number[]
  seconds?: number[]
}) {
  return (
    <div
      className={cn(
        'absolute left-0 top-[calc(100%+8px)] z-50 flex h-[220px] w-full overflow-hidden rounded-2xl',
        'border border-gray-200 bg-white shadow-2xl dark:border-white/10 dark:bg-[#121214]'
      )}
      role='dialog'
    >
      <TimeCol label='HH' items={hours} active={value.hh} onPick={(hh) => onChange({ ...value, hh })} />
      <div className='custom-scrollbar flex-1 overflow-y-auto border-x border-gray-100 p-1 text-center dark:border-white/5'>
        <TimeColInner label='MM' items={minutes} active={value.mm} onPick={(mm) => onChange({ ...value, mm })} />
      </div>
      <TimeCol label='SS' items={seconds} active={value.ss} onPick={(ss) => onChange({ ...value, ss })} />

      {onClose && (
        <button
          type='button'
          className='absolute -top-8 right-0 text-[11px] font-semibold uppercase tracking-widest text-gray-400 hover:text-gray-200'
          onClick={onClose}
        >
          Close
        </button>
      )}
    </div>
  )
}

function TimeCol({
  label,
  items,
  active,
  onPick,
}: {
  label: string
  items: number[]
  active: string
  onPick: (val: string) => void
}) {
  return (
    <div className='custom-scrollbar flex-1 overflow-y-auto p-1 text-center'>
      <TimeColInner label={label} items={items} active={active} onPick={onPick} />
    </div>
  )
}

function TimeColInner({
  label,
  items,
  active,
  onPick,
}: {
  label: string
  items: number[]
  active: string
  onPick: (val: string) => void
}) {
  return (
    <>
      <div className='sticky top-0 z-10 bg-white py-2 text-[10px] font-bold uppercase text-gray-400 dark:bg-[#121214] dark:text-gray-500'>
        {label}
      </div>
      {items.map((v) => {
        const txt = pad2(v)
        const isActive = txt === active

        return (
          <button
            key={txt}
            type='button'
            className={cn(
              'w-full rounded-lg py-2 text-sm',
              isActive
                ? 'glass-highlight border border-primary/20 font-bold text-primary'
                : 'text-gray-400 hover:bg-gray-100 dark:text-gray-500 dark:hover:bg-white/5'
            )}
            onClick={() => onPick(txt)}
          >
            {txt}
          </button>
        )
      })}
    </>
  )
}

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
  const isControlled = Object.prototype.hasOwnProperty.call(props, 'value')
  const [internal, setInternal] = React.useState<DateTimeRangeValue>(value ?? defaultValue())

  React.useEffect(() => {
    if (!isControlled) return
    setInternal(value ?? defaultValue())
  }, [isControlled, value])

  const emit = React.useCallback(
    (next: DateTimeRangeValue) => {
      if (!isControlled) setInternal(next)
      onChange?.(next)
    },
    [isControlled, onChange]
  )

  const handleClear = React.useCallback(() => {
    const next = defaultValue()
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

  const headerText = React.useMemo(() => formatRange(internal.from, internal.to), [internal.from, internal.to])

  return (
    <div
      className={cn(
        'w-full max-w-[720px] overflow-visible rounded-[24px] border border-gray-200 bg-white shadow-2xl dark:border-white/5 dark:bg-[#121214]',
        className
      )}
    >
      <div className='flex items-center justify-between border-b border-gray-100 bg-white p-5 dark:border-white/5 dark:bg-[#0a0a0b]/50'>
        <div className='flex items-center gap-3 rounded-xl border border-gray-200 bg-gray-50 px-4 py-2 text-sm font-medium text-gray-700 dark:border-white/10 dark:bg-white/5 dark:text-gray-300'>
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
              MonthCaption: ({ calendarMonth, className, displayIndex: _displayIndex, ...props }) => (
                <div className={className} {...props}>
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
              Start Time
            </label>

            <div className='relative' ref={startWrapRef}>
              <button
                type='button'
                className={cn(
                  'flex w-full items-center rounded-xl border border-gray-200 bg-white px-4 py-3.5 transition-all hover:border-white/20',
                  'dark:border-white/10 dark:bg-[#121214]/60',
                  startOpen && 'active-glow'
                )}
                onClick={toggleStart}
              >
                <input
                  readOnly
                  className='w-full cursor-pointer border-none bg-transparent p-0 text-sm font-semibold text-gray-900 placeholder-gray-500 focus:ring-0 dark:text-gray-100'
                  value={timeToString(internal.startTime)}
                  placeholder='HH:mm:ss'
                />
                <Clock className='ml-2 h-5 w-5 text-gray-400' />
              </button>

              {startOpen && (
                <TimeDropdown
                  value={internal.startTime}
                  onChange={(next) => emit({ ...internal, startTime: next })}
                  onClose={closeStart}
                />
              )}
            </div>
          </div>

          <div className='flex-1 space-y-3'>
            <label className='block text-[11px] font-bold uppercase tracking-[0.2em] text-gray-500'>
              End Time
            </label>

            <div className='relative' ref={endWrapRef}>
              <button
                type='button'
                className={cn(
                  'flex w-full items-center rounded-xl border border-gray-200 bg-white px-4 py-3.5 transition-all hover:border-white/20',
                  'dark:border-white/10 dark:bg-[#121214]/60',
                  endOpen && 'active-glow'
                )}
                onClick={toggleEnd}
              >
                <input
                  readOnly
                  className='w-full cursor-pointer border-none bg-transparent p-0 text-sm font-semibold text-gray-900 placeholder-gray-500 focus:ring-0 dark:text-gray-100'
                  value={timeToString(internal.endTime)}
                  placeholder='HH:mm:ss'
                />
                <Clock className='ml-2 h-5 w-5 text-gray-400' />
              </button>

              {endOpen && (
                <TimeDropdown
                  value={internal.endTime}
                  onChange={(next) => emit({ ...internal, endTime: next })}
                  onClose={closeEnd}
                />
              )}
            </div>
          </div>
        </div>
      </div>

      <div className='flex items-center justify-between border-t border-gray-100 bg-white px-8 py-6 dark:border-white/5 dark:bg-[#0a0a0b]'>
        <button
          type='button'
          className='text-[11px] font-semibold uppercase tracking-widest text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-200'
          onClick={handleClear}
        >
          Clear Selection
        </button>

        <div className='flex gap-4'>
          <button
            type='button'
            className='rounded-xl px-6 py-2.5 text-sm font-semibold text-gray-600 transition-all hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-white/5'
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            type='button'
            className='rounded-xl bg-primary px-8 py-2.5 text-sm font-bold text-white shadow-xl shadow-primary/20 transition-all active:scale-[0.98]'
            onClick={() => onConfirm?.(internal)}
          >
            Confirm Range
          </button>
        </div>
      </div>
    </div>
  )
}

export function DateRangePicker(props: DateRangePickerProps) {
  const { value, onChange, onCancel, onConfirm, className } = props
  const isControlled = Object.prototype.hasOwnProperty.call(props, 'value')
  const [open, setOpen] = React.useState(false)
  const [internalValue, setInternalValue] = React.useState<DateTimeRangeValue | undefined>(value)

  React.useEffect(() => {
    if (!isControlled) return
    setInternalValue(value)
  }, [isControlled, value])

  const handleChange = React.useCallback(
    (next: DateTimeRangeValue | undefined) => {
      if (!isControlled) setInternalValue(next)
      onChange?.(next)
    },
    [isControlled, onChange]
  )

  const currentValue = isControlled ? value : internalValue
  const label = formatRange(currentValue?.from, currentValue?.to)

  return (
    <div className={cn('grid gap-2', className)}>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            id='date'
            variant='outline'
            className={cn('h-8 justify-start text-left font-normal', !currentValue?.from && !currentValue?.to && 'text-muted-foreground')}
          >
            <Calendar className='mr-2 h-4 w-4' />
            <span>{label}</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent className='w-auto border-none bg-transparent p-0 shadow-none' align='start'>
          <DateTimeRangePicker
            value={currentValue}
            onChange={handleChange}
            onCancel={() => {
              setOpen(false)
              onCancel?.()
            }}
            onConfirm={(next) => {
              onConfirm?.(next)
              setOpen(false)
            }}
          />
        </PopoverContent>
      </Popover>
    </div>
  )
}
