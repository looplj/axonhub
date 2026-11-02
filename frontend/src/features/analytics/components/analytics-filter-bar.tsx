import { useState, useCallback, useMemo } from 'react';
import { IconX, IconFilter } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useAnalyticsFilterStore } from '@/stores/analyticsStore';
import { formatUserName } from '@/lib/utils';
import { useDebounce } from '@/hooks/use-debounce';
import { Button } from '@/components/ui/button';
import { DateRangePicker } from '@/components/date-range-picker';
import type { DateTimeRangeValue } from '@/utils/date-range';
import { useApiKeyOptions, useApiKeyOptionsByIDs } from '@/features/apikeys/data/apikeys';
import { useAllChannelSummarys } from '@/features/channels/data/channels';
import { useProjects } from '@/features/projects/data/projects';
import { useUsers } from '@/features/users/data/users';
import { usePermissions } from '@/hooks/usePermissions';
import { dateTimeRangeToTimestamps, startOfLocalDay, timestampsToDateTimeRange } from '../utils/date-range';
import { AnalyticsFacetedFilter } from './analytics-faceted-filter';

// Formats the selected range with second precision for the trigger label.
function formatDateTimeLabel(range: DateTimeRangeValue | undefined, placeholder: string): string {
  if (!range?.from && !range?.to) return placeholder;

  const format = (day: Date, time: { hh: string; mm: string; ss: string }) => {
    const date = `${day.getFullYear()}-${String(day.getMonth() + 1).padStart(2, '0')}-${String(day.getDate()).padStart(2, '0')}`;

    return `${date} ${time.hh}:${time.mm}:${time.ss}`;
  };

  const from = range.from ? format(range.from, range.startTime) : '...';
  const to = range.to ? format(range.to, range.endTime) : '...';

  return `${from} ~ ${to}`;
}

interface AnalyticsFilterBarProps {
  earliestTime?: string | null;
}

export function AnalyticsFilterBar({ earliestTime }: AnalyticsFilterBarProps) {
  const { t } = useTranslation();
  const filter = useAnalyticsFilterStore((state) => state.filter);
  const [apiKeySearch, setApiKeySearch] = useState('');
  const debouncedApiKeySearch = useDebounce(apiKeySearch, 300);
  const { setStartTime, setEndTime, setProjectIDs, setChannelIDs, setModelIDs, setAPIKeyIDs, setUserIDs, resetFilter } =
    useAnalyticsFilterStore();
  const { userPermissions } = usePermissions();
  const canViewUsers = userPermissions.canRead;

  // Fetch real data for dropdowns
  const { data: channels, isLoading: isLoadingChannels } = useAllChannelSummarys();
  const {
    data: apiKeysData,
    isFetching: isFetchingApiKeys,
    fetchNextPage: fetchNextApiKeyPage,
    hasNextPage: hasNextApiKeyPage,
    isFetchingNextPage: isFetchingNextApiKeyPage,
  } = useApiKeyOptions({ search: debouncedApiKeySearch, includeArchived: true });
  const { data: selectedApiKeysData } = useApiKeyOptionsByIDs(filter.apiKeyIDs, {
    enabled: !!filter.apiKeyIDs?.length,
  });
  const { data: usersData, isLoading: isLoadingUsers } = useUsers({ first: 100 }, { disableAutoFetch: !canViewUsers });
  const { data: projectsData, isLoading: isLoadingProjects } = useProjects({ first: 100 });

  const channelOptions = useMemo(
    () =>
      (channels?.edges || []).map((edge) => ({
        label: edge.node.name,
        value: String(edge.node.id),
      })),
    [channels]
  );

  const modelOptions = useMemo(() => {
    const modelSet = new Set<string>();
    (channels?.edges || []).forEach((edge) => {
      (edge.node.allModelEntries || []).forEach((entry) => {
        if (entry.actualModel) modelSet.add(entry.actualModel);
      });
    });
    return Array.from(modelSet)
      .sort()
      .map((m) => ({ label: m, value: m }));
  }, [channels]);

  const apiKeyOptions = useMemo(() => {
    const options = new Map<string, { label: string; value: string }>();
    for (const edge of selectedApiKeysData?.edges ?? []) {
      options.set(edge.node.id, { label: edge.node.name, value: edge.node.id });
    }
    for (const page of apiKeysData?.pages ?? []) {
      for (const edge of page.edges) {
        options.set(edge.node.id, { label: edge.node.name, value: edge.node.id });
      }
    }
    return Array.from(options.values());
  }, [apiKeysData, selectedApiKeysData]);

  const userOptions = useMemo(
    () =>
      (usersData?.edges || []).map((edge) => ({
        label: formatUserName(edge.node.firstName, edge.node.lastName) || edge.node.email,
        value: String(edge.node.id),
      })),
    [usersData]
  );

  const projectOptions = useMemo(
    () =>
      (projectsData?.edges || []).map((edge) => ({
        label: edge.node.name,
        value: String(edge.node.id),
      })),
    [projectsData]
  );

  const bounds = useMemo(
    () => timestampsToDateTimeRange(filter.startTime, filter.endTime),
    [filter.startTime, filter.endTime]
  );

  const applyRange = useCallback(
    (start: Date | null, end: Date | null) => {
      setStartTime(start ? start.toISOString() : null);
      setEndTime(end ? end.toISOString() : null);
    },
    [setStartTime, setEndTime]
  );

  const handleRangeChange = useCallback(
    (range: DateTimeRangeValue | undefined) => {
      // Pass the stored bounds so an untouched range keeps its milliseconds.
      const { startTime, endTime } = dateTimeRangeToTimestamps(range, bounds);

      if (startTime && endTime && new Date(startTime) > new Date(endTime)) {
        toast.error(t('analytics.filter.startAfterEndError'));

        return;
      }

      setStartTime(startTime);
      setEndTime(endTime);
    },
    [bounds, setStartTime, setEndTime, t]
  );

  const setQuickRange = useCallback(
    (days: number) => {
      const now = new Date();
      const start = startOfLocalDay(now);
      start.setDate(start.getDate() - days + 1);

      applyRange(start, now);
    },
    [applyRange]
  );

  const setQuickMonth = useCallback(() => {
    const now = new Date();

    applyRange(new Date(now.getFullYear(), now.getMonth(), 1), now);
  }, [applyRange]);

  const setQuickYear = useCallback(() => {
    const now = new Date();

    applyRange(new Date(now.getFullYear(), 0, 1), now);
  }, [applyRange]);

  const setAllTime = useCallback(() => {
    if (!earliestTime) return;

    // The API returns an instant, so no client-side timezone reinterpretation.
    applyRange(new Date(earliestTime), new Date());
  }, [earliestTime, applyRange]);

  const hasFilters =
    filter.startTime || filter.endTime || filter.projectIDs || filter.channelIDs || filter.modelIDs || filter.apiKeyIDs || filter.userIDs;

  return (
    <div className='bg-card space-y-3 rounded-lg border p-4'>
      {/* Date Filters */}
      <div className='flex flex-wrap items-center gap-2'>
        <div className='flex items-center gap-1.5 text-sm font-medium'>
          <IconFilter className='text-muted-foreground h-4 w-4' />
          {t('analytics.filter.dateRange')}
        </div>

        {/* Date Range — precise to the second */}
        <DateRangePicker
          value={bounds.range}
          onChange={handleRangeChange}
          formatLabel={(range) => formatDateTimeLabel(range, t('analytics.filter.dateRangePlaceholder'))}
        />

        {/* Quick Range Buttons */}
        <div className='flex flex-wrap items-center gap-1'>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={() => setQuickRange(1)}>
            {t('analytics.filter.today')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={() => setQuickRange(7)}>
            {t('analytics.filter.last7Days')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={() => setQuickRange(30)}>
            {t('analytics.filter.last30Days')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={setQuickMonth}>
            {t('analytics.filter.thisMonth')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={setQuickYear}>
            {t('analytics.filter.thisYear')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={setAllTime}>
            {t('analytics.filter.allTime')}
          </Button>
        </div>
      </div>

      {/* Dimension Filters */}
      <div className='flex flex-wrap items-center gap-2'>
        <AnalyticsFacetedFilter
          title={t('analytics.filter.project')}
          options={projectOptions}
          selectedValues={filter.projectIDs || []}
          onSelectedValuesChange={setProjectIDs}
          isLoading={isLoadingProjects}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.channel')}
          options={channelOptions}
          selectedValues={filter.channelIDs || []}
          onSelectedValuesChange={setChannelIDs}
          isLoading={isLoadingChannels}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.model')}
          options={modelOptions}
          selectedValues={filter.modelIDs || []}
          onSelectedValuesChange={setModelIDs}
          isLoading={isLoadingChannels}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.apiKey')}
          options={apiKeyOptions}
          selectedValues={filter.apiKeyIDs || []}
          onSelectedValuesChange={setAPIKeyIDs}
          isLoading={isFetchingApiKeys && !apiKeysData}
          searchValue={apiKeySearch}
          onSearchValueChange={setApiKeySearch}
          hasMore={!!hasNextApiKeyPage}
          isLoadingMore={isFetchingNextApiKeyPage}
          onLoadMore={fetchNextApiKeyPage}
        />

        {canViewUsers && (
          <AnalyticsFacetedFilter
            title={t('analytics.filter.user')}
            options={userOptions}
            selectedValues={filter.userIDs || []}
            onSelectedValuesChange={setUserIDs}
            isLoading={isLoadingUsers}
          />
        )}

        {/* Reset Button */}
        {hasFilters && (
          <Button variant='ghost' size='sm' className='text-muted-foreground h-8 text-xs' onClick={resetFilter}>
            <IconX className='mr-1 h-3 w-3' />
            {t('analytics.filter.reset')}
          </Button>
        )}
      </div>
    </div>
  );
}
