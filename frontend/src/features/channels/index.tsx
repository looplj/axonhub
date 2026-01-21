import { useState, useMemo, useCallback, useEffect, lazy, Suspense } from 'react';
import { SortingState } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { createColumns } from './components/channels-columns';
import { ChannelsErrorBanner } from './components/channels-error-banner';
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons';
import { ChannelsTable } from './components/channels-table';
import { ChannelsTypeTabs } from './components/channels-type-tabs';
import ChannelsProvider from './context/channels-context';
import { useQueryChannels, useChannelTypes, useErrorChannelsCount, useChannelProbeData } from './data/channels';
import { useProvidersData } from '@/features/models/data/providers';

const ChannelsDialogs = lazy(() => import('./components/channels-dialogs').then((m) => ({ default: m.ChannelsDialogs })));

function ChannelsContent() {
  const { t } = useTranslation();
  useProvidersData();
  const { channelPermissions } = usePermissions();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'channels-table-page-size',
  });
  const [nameFilter, setNameFilter] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string[]>([]);
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [tagFilter, setTagFilter] = useState<string>('');
  const [modelFilter, setModelFilter] = useState<string>('');
  const [selectedTypeTab, setSelectedTypeTab] = useState<string>('all');
  const [showErrorOnly, setShowErrorOnly] = useState<boolean>(false);
  const [sorting, setSorting] = useState<SortingState>(() => {
    const stored = localStorage.getItem('channels-table-sorting');
    if (stored) {
      try {
        return JSON.parse(stored);
      } catch {
        return [{ id: 'createdAt', desc: true }];
      }
    }
    return [{ id: 'createdAt', desc: true }];
  });
  const [isHealthColumnVisible, setIsHealthColumnVisible] = useState<boolean>(() => {
    const stored = localStorage.getItem('channels-table-column-visibility');
    if (stored) {
      try {
        const visibility = JSON.parse(stored);
        return visibility.health !== false;
      } catch {
        return true;
      }
    }
    return true;
  });

  useEffect(() => {
    localStorage.setItem('channels-table-sorting', JSON.stringify(sorting));
  }, [sorting]);

  // Fetch channel types for tabs
  const { data: channelTypeCounts = [] } = useChannelTypes(statusFilter.length > 0 ? statusFilter : ['enabled', 'disabled']);

  // Fetch error channels count independently
  const { data: errorCount = 0 } = useErrorChannelsCount();

  // Debounce the name filter to avoid excessive API calls
  const debouncedNameFilter = useDebounce(nameFilter, 300);

  // Get types for the selected tab
  const tabFilteredTypes = useMemo(() => {
    if (selectedTypeTab === 'all') {
      return [];
    }
    // Filter types that start with the selected prefix
    return channelTypeCounts.filter(({ type }) => type.startsWith(selectedTypeTab)).map(({ type }) => type);
  }, [selectedTypeTab, channelTypeCounts]);

  // Build where clause with filters using useMemo
  const whereClause = useMemo(() => {
    const where: Record<string, string | string[] | boolean> = {};
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter;
    }
    // Combine tab filter with manual type filter
    const combinedTypeFilter = [...typeFilter];
    if (tabFilteredTypes.length > 0) {
      // If tab is selected, use tab types
      combinedTypeFilter.push(...tabFilteredTypes);
    }
    if (combinedTypeFilter.length > 0) {
      where.typeIn = Array.from(new Set(combinedTypeFilter)); // Remove duplicates
    }
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter;
    } else {
      // By default, exclude archived channels when no status filter is applied
      where.statusIn = ['enabled', 'disabled'];
    }
    if (showErrorOnly) {
      where.errorMessageNotNil = true;
    }
    return Object.keys(where).length > 0 ? where : undefined;
  }, [debouncedNameFilter, tabFilteredTypes, statusFilter, showErrorOnly]);

  const currentOrderBy = useMemo(() => {
    if (sorting.length === 0) {
      return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
    const [primary] = sorting;
    switch (primary.id) {
      case 'orderingWeight':
        return { field: 'ORDERING_WEIGHT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'name':
        return { field: 'NAME', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'status':
        return { field: 'STATUS', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'provider':
      case 'type':
        return { field: 'TYPE', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'createdAt':
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'updatedAt':
        return { field: 'UPDATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      default:
        return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
  }, [sorting]);

  const {
    data,
    isLoading,
    error: _error,
  } = useQueryChannels({
    ...paginationArgs,
    where: whereClause,
    orderBy: currentOrderBy,
    hasTag: tagFilter || undefined,
    model: modelFilter || undefined,
  });

  const channelIDs = useMemo(() => {
    return data?.edges?.map((edge) => edge.node.id) || [];
  }, [data?.edges]);

  const { data: probeData } = useChannelProbeData(channelIDs, { enabled: isHealthColumnVisible });

  const channelsWithProbeData = useMemo(() => {
    if (!data?.edges) return [];
    
    const probeMap = new Map(probeData?.map((probe) => [probe.channelID, probe.points]) || []);
    
    return data.edges.map((edge) => ({
      ...edge.node,
      probePoints: probeMap.get(edge.node.id) || [],
    }));
  }, [data?.edges, probeData]);

  const handleNextPage = useCallback(() => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'after');
    }
  }, [data?.pageInfo, setCursors]);

  const handlePreviousPage = useCallback(() => {
    if (data?.pageInfo?.hasPreviousPage) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'before');
    }
  }, [data?.pageInfo, setCursors]);

  const handlePageSizeChange = useCallback(
    (newPageSize: number) => {
      setPageSize(newPageSize);
    },
    [setPageSize]
  );

  const handleNameFilterChange = useCallback(
    (filter: string) => {
      setNameFilter(filter);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleTypeFilterChange = useCallback(
    (filters: string[]) => {
      setTypeFilter(filters);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleTabChange = useCallback(
    (tab: string) => {
      setSelectedTypeTab(tab);
      setTypeFilter([]);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [setSelectedTypeTab, setTypeFilter]
  );

  const handleStatusFilterChange = useCallback(
    (filters: string[]) => {
      setStatusFilter(filters);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleTagFilterChange = useCallback(
    (filter: string) => {
      setTagFilter(filter);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleModelFilterChange = useCallback(
    (filter: string) => {
      setModelFilter(filter);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleFilterErrorChannels = useCallback(() => {
    setShowErrorOnly(true);
    resetCursor();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleExitErrorOnlyMode = useCallback(() => {
    setShowErrorOnly(false);
    resetCursor();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const columns = useMemo(() => createColumns(t, channelPermissions.canWrite), [t, channelPermissions.canWrite]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <ChannelsErrorBanner
        errorCount={errorCount}
        onFilterErrorChannels={handleFilterErrorChannels}
        showErrorOnly={showErrorOnly}
        onExitErrorOnlyMode={handleExitErrorOnlyMode}
      />
      <ChannelsTypeTabs typeCounts={channelTypeCounts} selectedTab={selectedTypeTab} onTabChange={handleTabChange} />
      <ChannelsTable
        loading={isLoading}
        data={channelsWithProbeData}
        columns={columns}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        nameFilter={nameFilter}
        typeFilter={typeFilter}
        statusFilter={statusFilter}
        tagFilter={tagFilter}
        modelFilter={modelFilter}
        selectedTypeTab={selectedTypeTab}
        showErrorOnly={showErrorOnly}
        sorting={sorting}
        onSortingChange={setSorting}
        onExitErrorOnlyMode={handleExitErrorOnlyMode}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onResetCursor={resetCursor}
        onNameFilterChange={handleNameFilterChange}
        onTypeFilterChange={handleTypeFilterChange}
        onStatusFilterChange={handleStatusFilterChange}
        onTagFilterChange={handleTagFilterChange}
        onModelFilterChange={handleModelFilterChange}
        onHealthColumnVisibilityChange={setIsHealthColumnVisible}
        canWrite={channelPermissions.canWrite}
      />
    </div>
  );
}

export default function ChannelsManagement() {
  const { t } = useTranslation();

  return (
    <ChannelsProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('channels.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('channels.description')}</p>
          </div>
          <ChannelsPrimaryButtons />
        </div>
      </Header>

      <Main fixed>
        <ChannelsContent />
      </Main>
      <Suspense fallback={null}>
        <ChannelsDialogs />
      </Suspense>
    </ChannelsProvider>
  );
}
