import * as React from 'react'
import { Calendar } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import type { DateTimeRangeValue, TimeValue } from '@/utils/date-range'
import { DateTimeRangePicker } from './date-time-range-picker'
import { formatRange, normalizeDateTimeRangeValue } from './utils'

export type { DateTimeRangeValue, TimeValue } from '@/utils/date-range'
export { DateTimeRangePicker } from './date-time-range-picker'

interface DateRangePickerProps {
  value?: DateTimeRangeValue
  onChange?: (range: DateTimeRangeValue | undefined) => void
  onCancel?: () => void
  onConfirm?: (range: DateTimeRangeValue) => void
  className?: string
}

export function DateRangePicker(props: DateRangePickerProps) {
  const { value, onChange, onCancel, onConfirm, className } = props
  const { t } = useTranslation()
  const isControlled = Object.prototype.hasOwnProperty.call(props, 'value')
  const [open, setOpen] = React.useState(false)
  const [internalValue, setInternalValue] = React.useState<DateTimeRangeValue | undefined>(
    value ? normalizeDateTimeRangeValue(value) : undefined
  )

  React.useEffect(() => {
    if (!isControlled) return
    setInternalValue(value ? normalizeDateTimeRangeValue(value) : undefined)
  }, [isControlled, value])

  const handleChange = React.useCallback(
    (next: DateTimeRangeValue | undefined) => {
      const nextValue = next ? normalizeDateTimeRangeValue(next) : undefined
      if (!isControlled) setInternalValue(nextValue)
      onChange?.(nextValue)
    },
    [isControlled, onChange]
  )

  const currentValue = isControlled ? (value ? normalizeDateTimeRangeValue(value) : undefined) : internalValue
  const label = formatRange(currentValue?.from, currentValue?.to, t('common.filters.dateRange'))

  return (
    <div className={cn('grid gap-2', className)}>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            id='date'
            variant='outline'
            className={cn(
              'h-8 justify-start text-left font-normal',
              !currentValue?.from && !currentValue?.to && 'text-muted-foreground'
            )}
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
