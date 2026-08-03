import React, { useState } from 'react';
import type { SortingState } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { type DateTimeRangeValue } from '@/utils/date-range';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { createColumns } from './components/apikeys-columns';
import { ApiKeysDialogs } from './components/apikeys-dialogs';
import { ApiKeysPrimaryButtons } from './components/apikeys-primary-buttons';
import { ApiKeysTable } from './components/apikeys-table';
import ApiKeysProvider from './context/apikeys-context';
import { useApiKeys } from './data/apikeys';
import { ApiKeyType } from './data/schema';

type ApiKeyTabKey = ApiKeyType | 'all';

const DEFAULT_SORTING: SortingState = [{ id: 'createdAt', desc: true }];
const SORTABLE_COLUMN_IDS = new Set(['name', 'createdAt', 'updatedAt']);

function loadSorting(): SortingState {
  const fallback = () => DEFAULT_SORTING.map((item) => ({ ...item }));
  try {
    const stored = localStorage.getItem('apikeys-table-sorting');
    if (!stored) return fallback();

    const parsed: unknown = JSON.parse(stored);
    if (!Array.isArray(parsed) || parsed.length !== 1) return fallback();

    const [primary] = parsed;
    if (
      typeof primary !== 'object' ||
      primary === null ||
      !('id' in primary) ||
      !('desc' in primary) ||
      typeof primary.id !== 'string' ||
      !SORTABLE_COLUMN_IDS.has(primary.id) ||
      typeof primary.desc !== 'boolean'
    ) {
      return fallback();
    }

    return [{ id: primary.id, desc: primary.desc }];
  } catch {
    return fallback();
  }
}

function ApiKeysContent() {
  const { t } = useTranslation();
  const { apiKeyPermissions, hasSystemScope } = usePermissions();
  const { startCursor, endCursor, cursorHistory, pageSize, setCursors, setPageSize, resetCursor, paginationArgs } =
    usePaginationSearch({
      defaultPageSize: 20,
      pageSizeStorageKey: 'apikeys-table-page-size',
    });

  const [activeTab, setActiveTab] = useState<ApiKeyTabKey>('all');
  const [sorting, setSorting] = useState<SortingState>(loadSorting);
  const [sortingCursorResetPending, setSortingCursorResetPending] = useState(false);

  // Filter states - following the same pattern as roles and users
  const [searchFilter, setSearchFilter] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [userFilter, setUserFilter] = useState<string[]>([]);
  const [dateRange, setDateRange] = useState<DateTimeRangeValue | undefined>();

  const debouncedSearchFilter = useDebounce(searchFilter, 300);

  React.useEffect(() => {
    try {
      localStorage.setItem('apikeys-table-sorting', JSON.stringify(sorting));
    } catch {
      // Storage may be unavailable; sorting still works for the current session.
    }
  }, [sorting]);

  const hasPaginationCursor = Boolean(startCursor || endCursor || cursorHistory.length > 0);

  React.useEffect(() => {
    if (sortingCursorResetPending && !hasPaginationCursor) {
      setSortingCursorResetPending(false);
    }
  }, [hasPaginationCursor, sortingCursorResetPending]);

  // Build where clause for API filtering
  const whereClause = (() => {
    const where: Record<string, unknown> = {};
    
    // Use OR condition for searching both name and key
    if (debouncedSearchFilter) {
      where.or = [
        { nameContainsFold: debouncedSearchFilter },
        { keyContainsFold: debouncedSearchFilter },
      ];
    }
    
    if (activeTab !== 'all') {
      where.typeIn = [activeTab];
    }
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter;
    } else {
      // By default, exclude archived API keys when no status filter is applied
      where.statusIn = ['enabled', 'disabled'];
    }
    if (userFilter.length > 0 && userFilter[0]) {
      where.userID = userFilter[0]; // API expects single userID
    }
    
    // Add AND condition to combine OR search with other filters
    if (where.or && (where.typeIn || where.statusIn || where.userID)) {
      const orCondition = where.or;
      delete where.or;
      return {
        and: [
          { or: orCondition },
          where,
        ],
      };
    }
    
    return Object.keys(where).length > 0 ? where : undefined;
  })();

  const currentOrderBy = React.useMemo(() => {
    if (sorting.length === 0) {
      return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }

    const [primary] = sorting;
    switch (primary.id) {
      case 'name':
        return { field: 'NAME', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'updatedAt':
        return { field: 'UPDATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'createdAt':
      default:
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
    }
  }, [sorting]);

  const { data, isLoading } = useApiKeys({
    ...(sortingCursorResetPending ? { first: pageSize } : paginationArgs),
    where: whereClause,
    orderBy: currentOrderBy,
  });

  const tableData = React.useMemo(
    () => (data?.edges?.map((edge) => edge.node) ?? []),
    [data?.edges]
  );

  // Reset cursor when filters change
  React.useEffect(() => {
    resetCursor();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearchFilter, activeTab, statusFilter, userFilter, dateRange]);

  const handleNextPage = () => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'after');
    }
  };

  const handlePreviousPage = () => {
    if (data?.pageInfo?.hasPreviousPage) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'before');
    }
  };

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize);
  };

  const handleSortingChange = (updater: SortingState | ((previous: SortingState) => SortingState)) => {
    if (hasPaginationCursor) {
      setSortingCursorResetPending(true);
    }
    setSorting((previous) => (typeof updater === 'function' ? updater(previous) : updater));
    resetCursor();
  };

  const handleResetFilters = () => {
    setSearchFilter('');
    setStatusFilter([]);
    setUserFilter([]);
    setDateRange(undefined);
    resetCursor();
  };

  const canViewCreators = hasSystemScope('read_users');

  const columns = React.useMemo(
    () => createColumns(t, apiKeyPermissions.canWrite, canViewCreators),
    [t, apiKeyPermissions.canWrite, canViewCreators]
  );

  return (
    <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as ApiKeyTabKey)} className='w-full'>
        <TabsList className='shadow-soft border-border bg-background grid w-full grid-cols-4 rounded-2xl border'>
          <TabsTrigger value='all' data-value='all'>
            {t('apikeys.tabs.all')}
          </TabsTrigger>
          <TabsTrigger value='user' data-value='user'>
            {t('apikeys.type.user')}
          </TabsTrigger>
          <TabsTrigger value='personal' data-value='personal'>
            {t('apikeys.type.personal')}
          </TabsTrigger>
          <TabsTrigger value='service_account' data-value='service_account'>
            {t('apikeys.type.service_account')}
          </TabsTrigger>
        </TabsList>
      </Tabs>
      <div className='mt-6 flex min-h-0 flex-1 flex-col overflow-hidden'>
        <ApiKeysTable
          data={tableData}
          loading={isLoading}
          columns={columns}
          pageInfo={data?.pageInfo}
          pageSize={pageSize}
          totalCount={data?.totalCount}
          searchFilter={searchFilter}
          statusFilter={statusFilter}
          userFilter={userFilter}
          dateRange={dateRange}
          sorting={sorting}
          onNextPage={handleNextPage}
          onPreviousPage={handlePreviousPage}
          onPageSizeChange={handlePageSizeChange}
          onSearchFilterChange={setSearchFilter}
          onStatusFilterChange={setStatusFilter}
          onUserFilterChange={setUserFilter}
          onDateRangeChange={setDateRange}
          onSortingChange={handleSortingChange}
          onResetFilters={handleResetFilters}
          canWrite={apiKeyPermissions.canWrite}
          canViewCreators={canViewCreators}
        />
      </div>
    </div>
  );
}

export default function ApiKeysManagement() {
  const { t } = useTranslation();

  return (
    <ApiKeysProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('apikeys.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('apikeys.description')}</p>
          </div>
          <ApiKeysPrimaryButtons />
        </div>
      </Header>

      <Main fixed>
        <ApiKeysContent />
      </Main>
      <ApiKeysDialogs />
    </ApiKeysProvider>
  );
}
