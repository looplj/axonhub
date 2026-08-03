import { useMemo, useState } from 'react';
import { Cross2Icon } from '@radix-ui/react-icons';
import { Table } from '@tanstack/react-table';
import { Filter, RefreshCw, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { useSelectedProjectId } from '@/stores/projectStore';
import type { DateTimeRangeValue } from '@/utils/date-range';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { Switch } from '@/components/ui/switch';
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter';
import { DateRangePicker } from '@/components/date-range-picker';
import { useApiKeys } from '@/features/apikeys/data';
import { useMe } from '@/features/auth/data/auth';
import { useAllChannelSummarys } from '@/features/channels/data/channels';
import { RequestStatus } from '../data/schema';
import { DataTableViewOptions } from './data-table-view-options';
import { MODEL_ID_COLUMN } from './requests-columns';

interface DataTableToolbarProps<TData> {
  table: Table<TData>;
  dateRange?: DateTimeRangeValue;
  onDateRangeChange?: (range: DateTimeRangeValue | undefined) => void;
  onResetFilters?: () => void;
  onRefresh?: () => void;
  showRefresh?: boolean;
  autoRefresh?: boolean;
  onAutoRefreshChange?: (enabled: boolean) => void;
}

interface RequestFilterControlsProps {
  table: Table<any>;
  dateRange?: DateTimeRangeValue;
  onDateRangeChange?: (range: DateTimeRangeValue | undefined) => void;
  onResetFilters?: () => void;
  isFiltered: boolean;
  hasDateRange: boolean;
  requestStatuses: { value: string; label: string }[];
  requestSources: { value: string; label: string }[];
  canViewChannels: boolean;
  channelOptions: { value: string; label: string }[];
  isFetchingChannels: boolean;
  showArchivedChannels: boolean;
  handleToggleShowArchivedChannels: (checked: boolean) => void;
  canViewApiKeys: boolean;
  apiKeyOptions: { value: string; label: string }[];
  isFetchingApiKeys: boolean;
  showArchivedApiKeys: boolean;
  handleToggleShowArchivedApiKeys: (checked: boolean) => void;
  isMobile?: boolean;
  onCloseAfterAction?: () => void;
}

function RequestFilterControls({
  table,
  dateRange,
  onDateRangeChange,
  onResetFilters,
  isFiltered,
  hasDateRange,
  requestStatuses,
  requestSources,
  canViewChannels,
  channelOptions,
  isFetchingChannels,
  showArchivedChannels,
  handleToggleShowArchivedChannels,
  canViewApiKeys,
  apiKeyOptions,
  isFetchingApiKeys,
  showArchivedApiKeys,
  handleToggleShowArchivedApiKeys,
  isMobile = false,
  onCloseAfterAction,
}: RequestFilterControlsProps) {
  const { t } = useTranslation();

  const stopPropagation = (e: React.SyntheticEvent) => e.stopPropagation();

  const channelCheckboxId = isMobile ? 'show-archived-channels-mobile' : 'show-archived-channels';
  const apiKeyCheckboxId = isMobile ? 'show-archived-api-keys-mobile' : 'show-archived-api-keys';
  const wrapperClass = isMobile ? 'block' : 'hidden flex-wrap items-center gap-2 sm:flex';

  // Footer for archived toggle in DataTableFacetedFilter
  const channelFooter = (
    <div className='flex items-center space-x-2 px-2 py-1.5' onPointerDown={stopPropagation} onClick={stopPropagation}>
      <Checkbox
        id={channelCheckboxId}
        checked={showArchivedChannels}
        onCheckedChange={(checked) => handleToggleShowArchivedChannels(checked === true)}
        onPointerDown={stopPropagation}
        onClick={stopPropagation}
      />
      <label htmlFor={channelCheckboxId} className='cursor-pointer text-sm' onClick={stopPropagation}>
        {t('common.showArchived')}
      </label>
    </div>
  );

  const apiKeyFooter = (
    <div className='flex items-center space-x-2 px-2 py-1.5' onPointerDown={stopPropagation} onClick={stopPropagation}>
      <Checkbox
        id={apiKeyCheckboxId}
        checked={showArchivedApiKeys}
        onCheckedChange={(checked) => handleToggleShowArchivedApiKeys(checked === true)}
        onPointerDown={stopPropagation}
        onClick={stopPropagation}
      />
      <label htmlFor={apiKeyCheckboxId} className='cursor-pointer text-sm' onClick={stopPropagation}>
        {t('common.showArchived')}
      </label>
    </div>
  );

  return (
    <div className={wrapperClass}>
      {table.getColumn('status') && (
        <DataTableFacetedFilter column={table.getColumn('status')} title={t('requests.filters.status')} options={requestStatuses} />
      )}
      {table.getColumn('source') && (
        <DataTableFacetedFilter column={table.getColumn('source')} title={t('requests.filters.source')} options={requestSources} />
      )}
      {canViewChannels && table.getColumn('channel') && (channelOptions.length > 0 || isFetchingChannels) && (
        <DataTableFacetedFilter
          column={table.getColumn('channel')}
          title={t('requests.filters.channel')}
          options={channelOptions}
          footer={channelFooter}
        />
      )}
      {canViewApiKeys && table.getColumn('apiKey') && (apiKeyOptions.length > 0 || isFetchingApiKeys) && (
        <DataTableFacetedFilter
          column={table.getColumn('apiKey')}
          title={t('requests.filters.apiKey')}
          options={apiKeyOptions}
          footer={apiKeyFooter}
        />
      )}
      <DateRangePicker
        value={dateRange}
        onChange={(range) => {
          onDateRangeChange?.(range);
          onCloseAfterAction?.();
        }}
        className={isMobile ? 'w-full' : 'max-w-[150px] min-w-0 sm:max-w-none'}
      />
      {hasDateRange && (
        <Button
          variant='ghost'
          onClick={() => {
            onDateRangeChange?.(undefined);
            onCloseAfterAction?.();
          }}
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
            onResetFilters?.();
            onCloseAfterAction?.();
          }}
          className='h-8 px-2 lg:px-3'
        >
          {t('common.filters.reset')}
          <Cross2Icon className='ml-2 h-4 w-4' />
        </Button>
      )}
    </div>
  );
}

export function DataTableToolbar<TData>({
  table,
  dateRange,
  onDateRangeChange,
  onResetFilters,
  onRefresh,
  showRefresh = false,
  autoRefresh = false,
  onAutoRefreshChange,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation();
  const [showArchivedApiKeys, setShowArchivedApiKeys] = useState(false);
  const [showArchivedChannels, setShowArchivedChannels] = useState(false);
  const [sheetOpen, setSheetOpen] = useState(false);
  const hasDateRange = !!dateRange?.from || !!dateRange?.to;
  const isFiltered = table.getState().columnFilters.length > 0 || hasDateRange;

  // Active filter count for mobile badge (excludes model ID filter)
  const columnFilters = table.getState().columnFilters;
  const activeFilterCount = useMemo(() => {
    let count = 0;
    for (const filter of columnFilters) {
      if (filter.id !== MODEL_ID_COLUMN && filter.value) count++;
    }
    if (hasDateRange) count++;
    return count;
  }, [columnFilters, hasDateRange]);

  // Handler to toggle show archived API keys and prune hidden IDs from filters
  const handleToggleShowArchivedApiKeys = (checked: boolean) => {
    setShowArchivedApiKeys(checked === true);

    if (checked === false) {
      // When turning off show archived, prune any archived IDs from the filter
      const currentFilter = table.getColumn('apiKey')?.getFilterValue() as string[] | undefined;
      if (currentFilter && currentFilter.length > 0) {
        // Compute visible IDs from raw data (filtering for non-archived status)
        const visibleIds = new Set(
          apiKeysData?.edges?.filter((edge) => edge.node.status !== 'archived')?.map((edge) => edge.node.id) ?? []
        );
        const prunedFilter = currentFilter.filter((id) => visibleIds.has(id));
        table.getColumn('apiKey')?.setFilterValue(prunedFilter.length > 0 ? prunedFilter : undefined);
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
          channelsData?.edges?.filter((edge) => edge.node.status !== 'archived')?.map((edge) => edge.node.id) ?? []
        );
        const prunedFilter = currentFilter.filter((id) => visibleIds.has(id));
        table.getColumn('channel')?.setFilterValue(prunedFilter.length > 0 ? prunedFilter : undefined);
      }
    }
  };

  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const user = meData || authUser;
  const userScopes = user?.scopes || [];
  const isOwner = user?.isOwner || false;
  const selectedProjectId = useSelectedProjectId();

  const canViewChannels = isOwner || userScopes.includes('*') || userScopes.includes('read_channels');
  const canViewApiKeys = isOwner || userScopes.includes('*') || userScopes.includes('read_api_keys');

  const { data: channelsData, isFetching: isFetchingChannels } = useAllChannelSummarys(selectedProjectId, {
    enabled: canViewChannels,
    includeArchived: showArchivedChannels,
  });

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
    {
      value: 'canceled' as RequestStatus,
      label: t('requests.status.canceled'),
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
  ];

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <Input
        placeholder={t('requests.filters.filterModelId')}
        value={(table.getColumn(MODEL_ID_COLUMN)?.getFilterValue() as string) ?? ''}
        onChange={(event) => table.getColumn(MODEL_ID_COLUMN)?.setFilterValue(event.target.value)}
        className='h-8 min-w-0 flex-1 sm:w-[150px] lg:w-[250px]'
      />

      {/* Mobile: Filters button opens bottom sheet */}
      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetTrigger asChild>
          <Button variant='outline' size='sm' className='h-8 gap-1 sm:hidden'>
            <Filter className='h-4 w-4' />
            {t('common.filters.title')}
            {activeFilterCount > 0 && (
              <Badge
                variant='secondary'
                className='ml-1 min-w-[1.25rem] rounded-full px-1.5 py-0'
                aria-label={t('common.filters.activeCount', { count: activeFilterCount })}
              >
                {activeFilterCount}
              </Badge>
            )}
          </Button>
        </SheetTrigger>
        <SheetContent side='bottom' className='h-auto max-h-[85vh] overflow-hidden'>
          <SheetHeader>
            <SheetTitle>{t('common.filters.title')}</SheetTitle>
          </SheetHeader>
          <div className='flex flex-col gap-4 overflow-y-auto py-4'>
            <RequestFilterControls
              table={table}
              dateRange={dateRange}
              onDateRangeChange={onDateRangeChange}
              onResetFilters={onResetFilters}
              isFiltered={isFiltered}
              hasDateRange={hasDateRange}
              requestStatuses={requestStatuses}
              requestSources={requestSources}
              canViewChannels={canViewChannels}
              channelOptions={channelOptions}
              isFetchingChannels={isFetchingChannels}
              showArchivedChannels={showArchivedChannels}
              handleToggleShowArchivedChannels={handleToggleShowArchivedChannels}
              canViewApiKeys={canViewApiKeys}
              apiKeyOptions={apiKeyOptions}
              isFetchingApiKeys={isFetchingApiKeys}
              showArchivedApiKeys={showArchivedApiKeys}
              handleToggleShowArchivedApiKeys={handleToggleShowArchivedApiKeys}
              isMobile
              onCloseAfterAction={() => setSheetOpen(false)}
            />
          </div>
        </SheetContent>
      </Sheet>

      {/* Desktop: inline filter controls */}
      <RequestFilterControls
        table={table}
        dateRange={dateRange}
        onDateRangeChange={onDateRangeChange}
        onResetFilters={onResetFilters}
        isFiltered={isFiltered}
        hasDateRange={hasDateRange}
        requestStatuses={requestStatuses}
        requestSources={requestSources}
        canViewChannels={canViewChannels}
        channelOptions={channelOptions}
        isFetchingChannels={isFetchingChannels}
        showArchivedChannels={showArchivedChannels}
        handleToggleShowArchivedChannels={handleToggleShowArchivedChannels}
        canViewApiKeys={canViewApiKeys}
        apiKeyOptions={apiKeyOptions}
        isFetchingApiKeys={isFetchingApiKeys}
        showArchivedApiKeys={showArchivedApiKeys}
        handleToggleShowArchivedApiKeys={handleToggleShowArchivedApiKeys}
      />
      <div className='hidden flex-1 sm:block' />
      <div className='flex shrink-0 flex-wrap items-center gap-2'>
        {showRefresh && onAutoRefreshChange && (
          <div className='flex shrink-0 items-center gap-2'>
            <Switch
              checked={autoRefresh}
              onCheckedChange={onAutoRefreshChange}
              id='auto-refresh-switch'
              aria-label={t('common.autoRefresh')}
            />
            <label htmlFor='auto-refresh-switch' className='text-muted-foreground cursor-pointer text-sm whitespace-nowrap'>
              {t('common.autoRefresh')}
            </label>
          </div>
        )}
        {showRefresh && onRefresh && (
          <Button variant='outline' size='sm' onClick={onRefresh} aria-label={t('common.refresh')} className='shrink-0'>
            <RefreshCw className={`h-4 w-4 ${autoRefresh ? 'animate-spin' : ''} sm:mr-2`} />
            <span className='hidden sm:inline'>{t('common.refresh')}</span>
          </Button>
        )}
        <DataTableViewOptions table={table} />
      </div>
    </div>
  );
}
