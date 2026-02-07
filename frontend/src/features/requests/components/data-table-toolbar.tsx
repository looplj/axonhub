import { useMemo, useState } from 'react';
import { Cross2Icon } from '@radix-ui/react-icons';
import { Table } from '@tanstack/react-table';
import { RefreshCw, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter';
import { DateRangePicker } from '@/components/date-range-picker';
import { DataTableViewOptions } from './data-table-view-options';
import { useApiKeys } from '@/features/apikeys/data';
import { useMe } from '@/features/auth/data/auth';
import { useQueryChannels } from '@/features/channels/data/channels';
import { RequestStatus } from '../data/schema';
import type { DateTimeRangeValue } from '@/utils/date-range';


interface DataTableToolbarProps<TData> {
  table: Table<TData>;
  dateRange?: DateTimeRangeValue;
  onDateRangeChange?: (range: DateTimeRangeValue | undefined) => void;
  onRefresh?: () => void;
  showRefresh?: boolean;
  apiKeyFilter?: string[];
  onApiKeyFilterChange?: (filters: string[]) => void;
  autoRefresh?: boolean;
  onAutoRefreshChange?: (enabled: boolean) => void;
}

export function DataTableToolbar<TData>({
  table,
  dateRange,
  onDateRangeChange,
  onRefresh,
  showRefresh = false,
  apiKeyFilter,
  onApiKeyFilterChange,
  autoRefresh = false,
  onAutoRefreshChange,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation();
  const [showArchivedApiKeys, setShowArchivedApiKeys] = useState(false);
  const [showArchivedChannels, setShowArchivedChannels] = useState(false);
  const hasDateRange = !!dateRange?.from || !!dateRange?.to;
  const isFiltered = table.getState().columnFilters.length > 0 || hasDateRange;

  // Handler to toggle show archived API keys and prune hidden IDs from filters
  const handleToggleShowArchivedApiKeys = (checked: boolean) => {
    setShowArchivedApiKeys(checked === true);

    if (checked === false) {
      // When turning off show archived, prune any archived IDs from the filter
      const currentFilter = table.getColumn('apiKey')?.getFilterValue() as string[] | undefined;
      if (currentFilter && currentFilter.length > 0) {
        // Compute visible IDs from raw data (filtering for non-archived status)
        const visibleIds = new Set(
          apiKeysData?.edges
            ?.filter((edge) => edge.node.status !== 'archived')
            ?.map((edge) => edge.node.id) ?? []
        );
        const prunedFilter = currentFilter.filter((id) => visibleIds.has(id));
        table
          .getColumn('apiKey')
          ?.setFilterValue(prunedFilter.length > 0 ? prunedFilter : undefined);
      }
    }
  };

  // Handler to toggle show archived channels and prune hidden IDs from filters
  const handleToggleShowArchivedChannels = (checked: boolean) => {
    setShowArchivedChannels(checked === true);

    if (checked === false) {
      // When turning off show archived, prune any archived IDs from the filter
      const currentFilter = table.getColumn('channel')?.getFilterValue() as string[] | undefined;
      if (currentFilter && currentFilter.length > 0) {
        // Compute visible IDs from raw data (filtering for non-archived status)
        const visibleIds = new Set(
          channelsData?.edges
            ?.filter((edge) => edge.node.status !== 'archived')
            ?.map((edge) => edge.node.id) ?? []
        );
        const prunedFilter = currentFilter.filter((id) => visibleIds.has(id));
        table
          .getColumn('channel')
          ?.setFilterValue(prunedFilter.length > 0 ? prunedFilter : undefined);
      }
    }
  };

  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const user = meData || authUser;
  const userScopes = user?.scopes || [];
  const isOwner = user?.isOwner || false;

  const canViewChannels = isOwner || userScopes.includes('*') || userScopes.includes('read_channels');
  const canViewApiKeys = isOwner || userScopes.includes('*') || userScopes.includes('read_api_keys');

   const { data: channelsData, isFetching: isFetchingChannels } = useQueryChannels(
     {
       first: 100,
       orderBy: { field: 'CREATED_AT', direction: 'DESC' },
       where: showArchivedChannels
         ? {
             statusIn: ['enabled', 'disabled', 'archived'],
           }
         : {
             statusIn: ['enabled', 'disabled'],
           },
     },
     {
       disableAutoFetch: !canViewChannels,
     }
   );

   const { data: apiKeysData, isFetching: isFetchingApiKeys } = useApiKeys(
     {
       first: 100,
       orderBy: { field: 'CREATED_AT', direction: 'DESC' },
       where: showArchivedApiKeys
         ? {
             statusIn: ['enabled', 'disabled', 'archived'],
           }
         : {
             statusIn: ['enabled', 'disabled'],
           },
     },
     {
       disableAutoFetch: !canViewApiKeys,
     }
   );

  const channelOptions = useMemo(() => {
    if (!canViewChannels || !channelsData?.edges) return [];

    return channelsData.edges.map((edge) => ({
      value: edge.node.id,
      label: edge.node.name,
    }));
  }, [canViewChannels, channelsData]);

  const apiKeyOptions = useMemo(() => {
    if (!canViewApiKeys || !apiKeysData?.edges) return [];

    return apiKeysData.edges.map((edge) => ({
      value: edge.node.id,
      label: edge.node.name,
    }));
  }, [canViewApiKeys, apiKeysData]);

  const requestStatuses = [
    {
      value: 'pending' as RequestStatus,
      label: t('requests.status.pending'),
    },
    {
      value: 'processing' as RequestStatus,
      label: t('requests.status.processing'),
    },
    {
      value: 'completed' as RequestStatus,
      label: t('requests.status.completed'),
    },
    {
      value: 'failed' as RequestStatus,
      label: t('requests.status.failed'),
    },
  ];

  const requestSources = [
    {
      value: 'api',
      label: t('requests.source.api'),
    },
    {
      value: 'playground',
      label: t('requests.source.playground'),
    },
    {
      value: 'test',
      label: t('requests.source.test'),
    },
  ];

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 items-center space-x-2'>
        <Input
          placeholder={t('requests.filters.filterId')}
          value={(table.getColumn('id')?.getFilterValue() as string) ?? ''}
          onChange={(event) => table.getColumn('id')?.setFilterValue(event.target.value)}
          className='h-8 w-[150px] lg:w-[250px]'
        />
        {table.getColumn('status') && (
          <DataTableFacetedFilter column={table.getColumn('status')} title={t('requests.filters.status')} options={requestStatuses} />
        )}
        {/* {table.getColumn('source') && (
          <DataTableFacetedFilter
            column={table.getColumn('source')}
            title={t('requests.filters.source')}
            options={requestSources}
          />
        )} */}
         {canViewChannels && table.getColumn('channel') && (channelOptions.length > 0 || isFetchingChannels) && (
          <DataTableFacetedFilter
            column={table.getColumn('channel')}
            title={t('requests.filters.channel')}
            options={channelOptions}
            footer={
              <div
                className='flex items-center space-x-2 px-2 py-1.5'
                onPointerDown={(e) => e.stopPropagation()}
                onClick={(e) => e.stopPropagation()}
              >
                <Checkbox
                  id='show-archived-channels'
                  checked={showArchivedChannels}
                  onCheckedChange={(checked) => handleToggleShowArchivedChannels(checked === true)}
                  onPointerDown={(e) => e.stopPropagation()}
                  onClick={(e) => e.stopPropagation()}
                />
                <label htmlFor='show-archived-channels' className='cursor-pointer text-sm' onClick={(e) => e.stopPropagation()}>
                  {t('common.showArchived')}
                </label>
              </div>
            }
          />
        )}
         {canViewApiKeys && table.getColumn('apiKey') && (apiKeyOptions.length > 0 || isFetchingApiKeys) && (
          <DataTableFacetedFilter
            column={table.getColumn('apiKey')}
            title={t('requests.filters.apiKey')}
            options={apiKeyOptions}
            footer={
              <div
                className='flex items-center space-x-2 px-2 py-1.5'
                onPointerDown={(e) => e.stopPropagation()}
                onClick={(e) => e.stopPropagation()}
              >
                <Checkbox
                  id='show-archived-api-keys'
                  checked={showArchivedApiKeys}
                  onCheckedChange={(checked) => handleToggleShowArchivedApiKeys(checked === true)}
                  onPointerDown={(e) => e.stopPropagation()}
                  onClick={(e) => e.stopPropagation()}
                />
                <label htmlFor='show-archived-api-keys' className='cursor-pointer text-sm' onClick={(e) => e.stopPropagation()}>
                  {t('common.showArchived')}
                </label>
              </div>
            }
          />
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
        <DataTableViewOptions table={table} />
      </div>
    </div>
  );
}
