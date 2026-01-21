import { useMemo } from 'react';
import { Cross2Icon } from '@radix-ui/react-icons';
import { Table } from '@tanstack/react-table';
import { RefreshCw, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter';
import { DateRangePicker } from '@/components/date-range-picker';
import { useUsageLogPermissions } from '../../../gql/useUsageLogPermissions';
import { useQueryChannels } from '../../channels/data';
import { UsageLogSource } from '../data/schema';
import { DataTableViewOptions } from './data-table-view-options';
import type { DateTimeRangeValue } from '@/utils/date-range';

interface DataTableToolbarProps<TData> {
  table: Table<TData>;
  dateRange?: DateTimeRangeValue;
  onDateRangeChange?: (range: DateTimeRangeValue | undefined) => void;
  onRefresh?: () => void;
  showRefresh?: boolean;
  autoRefresh?: boolean;
  onAutoRefreshChange?: (value: boolean) => void;
}

export function DataTableToolbar<TData>({
  table,
  dateRange,
  onDateRangeChange,
  onRefresh,
  showRefresh = false,
  autoRefresh = false,
  onAutoRefreshChange,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation();
  const permissions = useUsageLogPermissions();
  const { canViewChannels } = permissions;

  const hasDateRange = !!dateRange?.from || !!dateRange?.to;
  const isFiltered = table.getState().columnFilters.length > 0 || hasDateRange;

  // Fetch channels data if user has permission
  const { data: channelsData } = useQueryChannels(
    canViewChannels
      ? {
          first: 100,
          orderBy: { field: 'CREATED_AT', direction: 'DESC' },
        }
      : undefined
  );

  // Prepare channel options for filter
  const channelOptions = useMemo(() => {
    if (!canViewChannels || !channelsData?.edges) return [];

    return channelsData.edges.map((edge) => ({
      value: edge.node.id,
      label: edge.node.name || `Channel ${edge.node.id}`,
    }));
  }, [canViewChannels, channelsData]);

  const usageLogSources = [
    {
      value: 'api' as UsageLogSource,
      label: t('usageLogs.source.api'),
    },
    {
      value: 'playground' as UsageLogSource,
      label: t('usageLogs.source.playground'),
    },
    {
      value: 'test' as UsageLogSource,
      label: t('usageLogs.source.test'),
    },
  ];

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 items-center space-x-2'>
        <Input
          placeholder={t('usageLogs.filters.filterId')}
          value={(table.getColumn('id')?.getFilterValue() as string) ?? ''}
          onChange={(event) => table.getColumn('id')?.setFilterValue(event.target.value)}
          className='h-8 w-[150px] lg:w-[250px]'
        />
        {/* {table.getColumn('source') && (
          <DataTableFacetedFilter
            column={table.getColumn('source')}
            title={t('usageLogs.filters.source')}
            options={usageLogSources}
          />
        )} */}
        {canViewChannels && table.getColumn('channel') && channelOptions.length > 0 && channelsData?.edges && (
          <DataTableFacetedFilter column={table.getColumn('channel')} title={t('usageLogs.filters.channel')} options={channelOptions} />
        )}
        <DateRangePicker value={dateRange} onChange={onDateRangeChange} />
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
            }}
            className='h-8 px-2 lg:px-3'
          >
            {t('common.filters.reset')}
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
      </div>
      <div className='flex items-center space-x-2'>
        {showRefresh && onAutoRefreshChange && (
          <div className='flex items-center space-x-2'>
            <Switch checked={autoRefresh} onCheckedChange={onAutoRefreshChange} id='auto-refresh-switch' />
            <label htmlFor='auto-refresh-switch' className='text-muted-foreground cursor-pointer text-sm'>
              {t('common.autoRefresh')}
            </label>
          </div>
        )}
        {showRefresh && onRefresh && (
          <Button variant='outline' size='sm' onClick={onRefresh}>
            <RefreshCw className={`mr-2 h-4 w-4 ${autoRefresh ? 'animate-spin' : ''}`} />
            {t('common.refresh')}
          </Button>
        )}
        {/* <DataTableViewOptions table={table} /> */}
      </div>
    </div>
  );
}
