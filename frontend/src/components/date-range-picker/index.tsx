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
  const [pendingValue, setPendingValue] = React.useState<DateTimeRangeValue | undefined>(
    normalizedValue
  )

  React.useEffect(() => {
    if (!isControlled) return
    setInternalValue(normalizedValue)
  }, [isControlled, normalizedValue])

  React.useEffect(() => {
    if (open) {
      const currentValue = isControlled ? normalizedValue : internalValue
      setPendingValue(currentValue)
    }
  }, [open, isControlled, normalizedValue, internalValue])

  const handlePendingChange = React.useCallback(
    (next: DateTimeRangeValue | undefined) => {
      const nextValue = next ? normalizeDateTimeRangeValue(next) : undefined
      setPendingValue(nextValue)
    },
    []
  )

  const handleConfirm = React.useCallback(
    (next: DateTimeRangeValue) => {
      const nextValue = normalizeDateTimeRangeValue(next)
      const hasRange = !!nextValue.from || !!nextValue.to
      if (!hasRange) {
        if (!isControlled) setInternalValue(undefined)
        onChange?.(undefined)
        setOpen(false)
        return
      }
      if (!isControlled) setInternalValue(nextValue)
      onChange?.(nextValue)
      onConfirm?.(nextValue)
      setOpen(false)
    },
    [isControlled, onChange, onConfirm]
  )

  const handleCancel = React.useCallback(() => {
    const currentValue = isControlled ? normalizedValue : internalValue
    setPendingValue(currentValue)
    setOpen(false)
    onCancel?.()
  }, [isControlled, normalizedValue, internalValue, onCancel])

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
            value={pendingValue}
            onChange={handlePendingChange}
            onCancel={handleCancel}
            onConfirm={handleConfirm}
          />
        </PopoverContent>
      </Popover>
    </div>
  )
}
