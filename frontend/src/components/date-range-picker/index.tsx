import * as React from 'react';
import { Calendar } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { normalizeDateTimeRangeValue, type DateTimeRangeValue } from '@/utils/date-range';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { DateTimeRangePicker } from './date-time-range-picker';
import { formatRange } from './utils';


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
  const normalizedValue = React.useMemo(
    () => (value ? normalizeDateTimeRangeValue(value) : undefined),
    [value]
  )
  const [internalValue, setInternalValue] = React.useState<DateTimeRangeValue | undefined>(
    normalizedValue
  )

  React.useEffect(() => {
    if (!isControlled) return
    setInternalValue(normalizedValue)
  }, [isControlled, normalizedValue])

  const handleChange = React.useCallback(
    (next: DateTimeRangeValue | undefined) => {
      const nextValue = next ? normalizeDateTimeRangeValue(next) : undefined
      if (!isControlled) setInternalValue(nextValue)
      onChange?.(nextValue)
    },
    [isControlled, onChange]
  )

  const currentValue = isControlled ? normalizedValue : internalValue
  const label = formatRange(currentValue?.from, currentValue?.to, t('common.filters.dateRange'))

  return (
    <div className={cn('grid gap-2', className)}>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            id='date'
            variant='outline'
            size='sm'
            className={cn(
              'h-8 border-solid',
              !currentValue?.from && !currentValue?.to && 'text-muted-foreground'
            )}
          >
            <Calendar className='h-4 w-4' />
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
