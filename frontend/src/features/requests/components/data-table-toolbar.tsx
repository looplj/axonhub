import { useMemo } from 'react';
import { Cross2Icon } from '@radix-ui/react-icons';
import { Table } from '@tanstack/react-table';
import { RefreshCw, X } from 'lucide-react';
import { DateRange } from 'react-day-picker';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter';
import { DateRangePicker } from '@/components/date-range-picker';
import { useApiKeys } from '@/features/apikeys/data';
import { useMe } from '@/features/auth/data/auth';
import { useQueryChannels } from '@/features/channels/data/channels';
import { RequestStatus } from '../data/schema';

interface ApiKeyEdge {
  node: {
    id: string;
    name: string;
  };
  cursor: string;
}

interface DataTableToolbarProps<TData> {
  table: Table<TData>;
  dateRange?: DateRange;
  onDateRangeChange?: (range: DateRange | undefined) => void;
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
  const isFiltered = table.getState().columnFilters.length > 0 || !!dateRange;

  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const user = meData || authUser;
  const userScopes = user?.scopes || [];
  const isOwner = user?.isOwner || false;

  const canViewChannels = isOwner || userScopes.includes('*') || userScopes.includes('read_channels');
  const canViewApiKeys = isOwner || userScopes.includes('*') || userScopes.includes('read_api_keys');

  const { data: channelsData } = useQueryChannels(
    {
      first: 100,
      orderBy: { field: 'CREATED_AT', direction: 'DESC' },
    },
    {
      disableAutoFetch: !canViewChannels,
    }
  );

  const { data: apiKeysData } = useApiKeys(
    {
      first: 100,
      orderBy: { field: 'CREATED_AT', direction: 'DESC' },
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
        {canViewChannels && table.getColumn('channel') && channelOptions.length > 0 && (
          <DataTableFacetedFilter column={table.getColumn('channel')} title={t('requests.filters.channel')} options={channelOptions} />
        )}
        {canViewApiKeys && table.getColumn('apiKey') && apiKeyOptions.length > 0 && (
          <DataTableFacetedFilter column={table.getColumn('apiKey')} title={t('requests.filters.apiKey')} options={apiKeyOptions} />
        )}
        <DateRangePicker value={dateRange} onChange={onDateRangeChange} />
        {dateRange && (
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
