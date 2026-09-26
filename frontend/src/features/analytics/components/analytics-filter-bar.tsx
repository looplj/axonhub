import { useState, useCallback, useMemo } from 'react';
import { IconX, IconFilter } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useAnalyticsFilterStore } from '@/stores/analyticsStore';
import { useDashboardTimeStore } from '@/stores/dashboardStore';
import { formatUserName } from '@/lib/utils';
import { useDebounce } from '@/hooks/use-debounce';
import { Button } from '@/components/ui/button';
import { TimeRangeFilter } from '@/components/time-range-filter';
import { useApiKeyOptions, useApiKeyOptionsByIDs } from '@/features/apikeys/data/apikeys';
import { useAllChannelSummarys } from '@/features/channels/data/channels';
import { useProjects } from '@/features/projects/data/projects';
import { useUsers } from '@/features/users/data/users';
import { usePermissions } from '@/hooks/usePermissions';
import { AnalyticsFacetedFilter } from './analytics-faceted-filter';

interface AnalyticsFilterBarProps {
  earliestDate?: string | null;
}

/** Analytics filter bar: the shared time range filter plus faceted dimension selectors. */
export function AnalyticsFilterBar({ earliestDate }: AnalyticsFilterBarProps) {
  const { t } = useTranslation();
  const { startTime, endTime, setRange } = useDashboardTimeStore();
  const dimensions = useAnalyticsFilterStore((state) => state.dimensions);
  const { setProjectIDs, setChannelIDs, setModelIDs, setAPIKeyIDs, setUserIDs, resetDimensionFilters } =
    useAnalyticsFilterStore();
  const [apiKeySearch, setApiKeySearch] = useState('');
  const debouncedApiKeySearch = useDebounce(apiKeySearch, 300);
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
  const { data: selectedApiKeysData } = useApiKeyOptionsByIDs(dimensions.apiKeyIDs, {
    enabled: !!dimensions.apiKeyIDs?.length,
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

  const hasDimensionFilters =
    dimensions.projectIDs || dimensions.channelIDs || dimensions.modelIDs || dimensions.apiKeyIDs || dimensions.userIDs;

  return (
    <div className='bg-card space-y-3 rounded-lg border p-4'>
      <TimeRangeFilter value={{ startTime, endTime }} onChange={setRange} earliestDate={earliestDate} />

      <div className='flex flex-wrap items-center gap-2'>
        <div className='flex items-center gap-1.5 text-sm font-medium'>
          <IconFilter className='text-muted-foreground h-4 w-4' />
          {t('analytics.filter.dimensions')}
        </div>

        <AnalyticsFacetedFilter
          title={t('analytics.filter.project')}
          options={projectOptions}
          selectedValues={dimensions.projectIDs || []}
          onSelectedValuesChange={setProjectIDs}
          isLoading={isLoadingProjects}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.channel')}
          options={channelOptions}
          selectedValues={dimensions.channelIDs || []}
          onSelectedValuesChange={setChannelIDs}
          isLoading={isLoadingChannels}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.model')}
          options={modelOptions}
          selectedValues={dimensions.modelIDs || []}
          onSelectedValuesChange={setModelIDs}
          isLoading={isLoadingChannels}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.apiKey')}
          options={apiKeyOptions}
          selectedValues={dimensions.apiKeyIDs || []}
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
            selectedValues={dimensions.userIDs || []}
            onSelectedValuesChange={setUserIDs}
            isLoading={isLoadingUsers}
          />
        )}

        {hasDimensionFilters && (
          <Button variant='ghost' size='sm' className='text-muted-foreground h-8 text-xs' onClick={resetDimensionFilters}>
            <IconX className='mr-1 h-3 w-3' />
            {t('analytics.filter.reset')}
          </Button>
        )}
      </div>
    </div>
  );
}
