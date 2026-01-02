import { useState, useMemo, useCallback, useEffect } from 'react';
import { SortingState } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { createColumns } from './components/channels-columns';
import { ChannelsDialogs } from './components/channels-dialogs';
import { ChannelsErrorBanner } from './components/channels-error-banner';
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons';
import { ChannelsTable } from './components/channels-table';
import { ChannelsTypeTabs } from './components/channels-type-tabs';
import ChannelsProvider from './context/channels-context';
import { useQueryChannels, useChannelTypes, useErrorChannelsCount } from './data/channels';

function ChannelsContent() {
  const { t } = useTranslation();
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

  // Build where clause with filters
  const whereClause = (() => {
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
  })();

  const currentOrderBy = (() => {
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
      case 'createdAt':
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'updatedAt':
        return { field: 'UPDATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      default:
        return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
  })();

  const {
    data,
    isLoading: _isLoading,
    error: _error,
  } = useQueryChannels({
    ...paginationArgs,
    where: whereClause,
    orderBy: currentOrderBy,
    hasTag: tagFilter || undefined,
    model: modelFilter || undefined,
  });

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
    [resetCursor, setNameFilter]
  );

  const handleTypeFilterChange = useCallback(
    (filters: string[]) => {
      setTypeFilter(filters);
      resetCursor();
    },
    [resetCursor, setTypeFilter]
  );

  const handleTabChange = useCallback(
    (tab: string) => {
      setSelectedTypeTab(tab);
      // Clear manual type filter when switching tabs
      setTypeFilter([]);
      resetCursor();
    },
    [resetCursor, setSelectedTypeTab]
  );

  const handleStatusFilterChange = useCallback(
    (filters: string[]) => {
      setStatusFilter(filters);
      resetCursor();
    },
    [resetCursor, setStatusFilter]
  );

  const handleTagFilterChange = useCallback(
    (filter: string) => {
      setTagFilter(filter);
      resetCursor();
    },
    [resetCursor, setTagFilter]
  );

  const handleModelFilterChange = useCallback(
    (filter: string) => {
      setModelFilter(filter);
      resetCursor();
    },
    [resetCursor, setModelFilter]
  );

  const handleFilterErrorChannels = useCallback(() => {
    setShowErrorOnly(true);
    resetCursor();
  }, [resetCursor]);

  const handleExitErrorOnlyMode = useCallback(() => {
    setShowErrorOnly(false);
    resetCursor();
  }, [resetCursor]);

  const columns = useMemo(() => createColumns(t), [t]);

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
        // loading={isLoading}
        data={data?.edges?.map((edge) => edge.node) || []}
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
      />
    </div>
  );
}

export default function ChannelsManagement() {
  const { t } = useTranslation();

  return (
    <ChannelsProvider>
      <Header fixed>{/* <Search /> */}</Header>

      <Main fixed>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>{t('channels.title')}</h2>
            <p className='text-muted-foreground'>{t('channels.description')}</p>
          </div>
          <ChannelsPrimaryButtons />
        </div>
        <ChannelsContent />
      </Main>
      <ChannelsDialogs />
    </ChannelsProvider>
  );
}
