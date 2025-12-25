import { Cross2Icon } from '@radix-ui/react-icons'
import { RefreshCw, X } from 'lucide-react'
import { Table } from '@tanstack/react-table'
import { DateRange } from 'react-day-picker'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DateRangePicker } from '@/components/date-range-picker'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
  dateRange?: DateRange
  onDateRangeChange?: (range: DateRange | undefined) => void
  threadIdFilter: string
  onThreadIdFilterChange: (threadId: string) => void
  onRefresh?: () => void
  showRefresh?: boolean
}

export function ThreadsTableToolbar<TData>({
  table,
  dateRange,
  onDateRangeChange,
  threadIdFilter,
  onThreadIdFilterChange,
  onRefresh,
  showRefresh = false,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation()
  const isFiltered = table.getState().columnFilters.length > 0 || !!dateRange || !!threadIdFilter.trim()

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 items-center space-x-2'>
        <Input
          placeholder={t('threads.filters.filterThreadId')}
          value={threadIdFilter}
          onChange={(event) => onThreadIdFilterChange(event.target.value)}
          className='h-8 w-[150px] lg:w-[250px]'
        />
        <DateRangePicker value={dateRange} onChange={onDateRangeChange} />
        {dateRange && (
          <Button 
            variant='ghost' 
            onClick={() => onDateRangeChange?.(undefined)} 
            className='h-8 px-2'
            size='sm'
          >
            <X className='h-4 w-4' />
          </Button>
        )}
        {isFiltered && (
          <Button
            variant='ghost'
            onClick={() => {
              table.resetColumnFilters()
              onDateRangeChange?.(undefined)
              onThreadIdFilterChange('')
            }}
            className='h-8 px-2 lg:px-3'
          >
            {t('common.filters.reset')}
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
      </div>
      <div className='flex items-center space-x-2'>
        {showRefresh && onRefresh && (
          <Button variant='outline' size='sm' onClick={onRefresh}>
            <RefreshCw className='mr-2 h-4 w-4' />
            {t('common.refresh')}
          </Button>
        )}
      </div>
    </div>
  )
}
