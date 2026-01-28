import { format } from 'date-fns'
import type { DayPickerProps } from 'react-day-picker'

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
  day: 'rdp-custom-day',
  outside: 'rdp-custom-outside',
  disabled: 'rdp-custom-disabled',
  range_start: 'rdp-custom-range-start',
  range_end: 'rdp-custom-range-end',
  range_middle: 'rdp-custom-range-middle',
  today: 'rdp-custom-today',
  selected: '',
  day_button: 'rdp-custom-day-button inline-flex h-8 w-8 items-center justify-center rounded-full text-sm transition-colors text-gray-700 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-white/5',
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
