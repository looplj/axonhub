import { format } from 'date-fns'
import type { DayPickerProps } from 'react-day-picker'
import { cn } from '@/lib/utils'

const WEEKDAYS = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']

export const dayPickerClassNames: DayPickerProps['classNames'] = {
  nav: 'hidden',
  months: 'flex gap-10',
  month: 'w-full',
  month_caption: 'mb-6 text-center text-sm font-semibold tracking-wide text-gray-900 dark:text-gray-100',
  month_grid: 'w-full border-collapse',
  weekdays: 'grid grid-cols-7 text-center',
  weekday: 'pb-4 text-[10px] font-bold uppercase tracking-[0.15em] text-gray-400 dark:text-gray-500',
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
}

export const dayPickerFormatters: DayPickerProps['formatters'] = {
  formatWeekdayName: (date) => WEEKDAYS[date.getDay()],
}

export const dayPickerComponents: DayPickerProps['components'] = {
  MonthCaption: ({ calendarMonth, className, displayIndex: _displayIndex, ...monthProps }) => (
    <div className={className} {...monthProps}>
      {format(calendarMonth.date, 'MMMM yyyy')}
    </div>
  ),
}
