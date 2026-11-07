import { useMemo } from 'react';
import { CheckIcon, Cross2Icon, PlusCircledIcon } from '@radix-ui/react-icons';
import { Table } from '@tanstack/react-table';
import { X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import type { DateRangeField, DateTimeRangeValue } from '@/utils/date-range';
import type { AutoRefreshInterval } from '@/hooks/use-auto-refresh-interval';
import { useHorizontalScroll } from '@/hooks/use-horizontal-scroll';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandItem, CommandList, CommandSeparator } from '@/components/ui/command';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Separator } from '@/components/ui/separator';
import { AutoRefreshControl } from '@/components/auto-refresh-control';
import { DateRangePicker } from '@/components/date-range-picker';
import { DataTableViewOptions } from './data-table-view-options';

function DateRangeFieldToggle({ value, onChange }: { value: DateRangeField; onChange: (field: DateRangeField) => void }) {
  const { t } = useTranslation();
  const options: { value: DateRangeField; label: string }[] = [
    { value: 'lastRequestAt', label: t('traces.columns.lastRequestAt') },
    { value: 'createdAt', label: t('common.columns.createdAt') },
  ];

  return (
    <div className='flex items-center gap-2'>
      <span className='text-muted-foreground text-xs'>{t('traces.filters.dateField')}</span>
      <div className='inline-flex items-center rounded-md border border-gray-200 p-0.5 dark:border-white/10'>
        {options.map((option) => (
          <button
            key={option.value}
            type='button'
            onClick={() => onChange(option.value)}
            className={cn(
              'rounded px-2 py-1 text-xs transition-colors',
              option.value === value
                ? 'bg-gray-100 font-medium text-gray-900 dark:bg-white/10 dark:text-white'
                : 'text-gray-500 hover:text-gray-900 dark:hover:text-white'
            )}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

interface DataTableToolbarProps<TData> {
  table: Table<TData>;
  dateRange?: DateTimeRangeValue;
  onDateRangeChange?: (range: DateTimeRangeValue | undefined) => void;
  dateRangeField?: DateRangeField;
  onDateRangeFieldChange?: (field: DateRangeField) => void;
  traceIdFilter: string;
  onTraceIdFilterChange: (traceId: string) => void;
  statusFilter?: string[];
  onStatusFilterChange?: (statuses: string[]) => void;
  onRefresh?: () => void;
  showRefresh?: boolean;
  autoRefreshInterval?: AutoRefreshInterval;
  onAutoRefreshIntervalChange?: (interval: AutoRefreshInterval) => void;
}

export function DataTableToolbar<TData>({
  table,
  dateRange,
  onDateRangeChange,
  dateRangeField = 'lastRequestAt',
  onDateRangeFieldChange,
  traceIdFilter,
  onTraceIdFilterChange,
  statusFilter = [],
  onStatusFilterChange,
  onRefresh,
  showRefresh = false,
  autoRefreshInterval = null,
  onAutoRefreshIntervalChange,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation();
  const scrollRef = useHorizontalScroll<HTMLDivElement>();
  const hasDateRange = !!dateRange?.from || !!dateRange?.to;
  const isFiltered = table.getState().columnFilters.length > 0 || hasDateRange || !!traceIdFilter.trim() || statusFilter.length > 0;

  const statusOptions = useMemo(
    () => [
      { value: 'active', label: t('traces.status.active', 'Active') },
      { value: 'archived', label: t('traces.status.archived', 'Archived') },
      { value: 'retained', label: t('traces.status.retained', 'Retained') },
    ],
    [t]
  );

  return (
    <div ref={scrollRef} className='flex items-center justify-between gap-2 overflow-x-auto'>
      <div className='flex flex-1 shrink-0 items-center space-x-2'>
        <Input
          placeholder={t('traces.filters.filterTraceId')}
          value={traceIdFilter}
          onChange={(event) => onTraceIdFilterChange(event.target.value)}
          className='h-8 w-[150px] lg:w-[250px]'
        />
        <DateRangePicker
          value={dateRange}
          onChange={onDateRangeChange}
          headerExtra={
            onDateRangeFieldChange ? <DateRangeFieldToggle value={dateRangeField} onChange={onDateRangeFieldChange} /> : undefined
          }
        />
        {onStatusFilterChange && (
          <Popover>
            <PopoverTrigger asChild>
              <Button variant='outline' size='sm' className='h-8 border-dashed'>
                <PlusCircledIcon className='mr-1 h-4 w-4' />
                {t('common.columns.status')}
                {statusFilter.length > 0 && (
                  <>
                    <Separator orientation='vertical' className='mx-2 h-4' />
                    <Badge variant='secondary' className='rounded-sm px-1 font-normal lg:hidden'>
                      {statusFilter.length}
                    </Badge>
                    <div className='hidden space-x-1 lg:flex'>
                      {statusFilter.length > 2 ? (
                        <Badge variant='secondary' className='rounded-sm px-1 font-normal'>
                          {t('common.selectedItems', { count: statusFilter.length })}
                        </Badge>
                      ) : (
                        statusOptions
                          ?.filter((option) => statusFilter.includes(option.value))
                          .map((option) => (
                            <Badge variant='secondary' key={option.value} className='rounded-sm px-1 font-normal'>
                              {option.label}
                            </Badge>
                          ))
                      )}
                    </div>
                  </>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent className='w-[200px] p-0' align='start'>
              <Command>
                <CommandList>
                  <CommandEmpty>{t('common.noResultsFound')}</CommandEmpty>
                  <CommandGroup>
                    {statusOptions.map((option) => {
                      const isSelected = statusFilter.includes(option.value);
                      return (
                        <CommandItem
                          key={option.value}
                          onSelect={() => {
                            const newFilter = isSelected ? statusFilter.filter((s) => s !== option.value) : [...statusFilter, option.value];
                            onStatusFilterChange(newFilter.length > 0 ? newFilter : []);
                          }}
                        >
                          <div
                            className={cn(
                              'border-primary flex h-4 w-4 items-center justify-center rounded-sm border',
                              isSelected ? 'bg-primary text-primary-foreground' : 'opacity-50 [&_svg]:invisible'
                            )}
                          >
                            <CheckIcon className='h-4 w-4' />
                          </div>
                          <span>{option.label}</span>
                        </CommandItem>
                      );
                    })}
                  </CommandGroup>
                  {statusFilter.length > 0 && (
                    <>
                      <CommandSeparator />
                      <CommandGroup>
                        <CommandItem onSelect={() => onStatusFilterChange([])} className='justify-center text-center'>
                          {t('common.clearFilters')}
                        </CommandItem>
                      </CommandGroup>
                    </>
                  )}
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
        )}
        {hasDateRange && (
          <Button variant='ghost' onClick={() => onDateRangeChange?.(undefined)} className='h-8 px-2' size='sm'>
            <X className='h-4 w-4' />
          </Button>
        )}
        {isFiltered && (
          <Button
            variant='ghost'
            onClick={() => {
              table.resetColumnFilters();
              onDateRangeChange?.(undefined);
              onTraceIdFilterChange('');
              onStatusFilterChange?.([]);
            }}
            className='h-8 px-2 lg:px-3'
          >
            {t('common.filters.reset')}
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
      </div>
      <div className='flex shrink-0 items-center space-x-2'>
        {showRefresh && onRefresh && onAutoRefreshIntervalChange && (
          <AutoRefreshControl interval={autoRefreshInterval} onIntervalChange={onAutoRefreshIntervalChange} onRefresh={onRefresh} />
        )}
        <DataTableViewOptions table={table} />
      </div>
    </div>
  );
}
