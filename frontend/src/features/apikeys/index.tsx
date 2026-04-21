import React, { useState } from 'react';
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

function ApiKeysContent() {
  const { t } = useTranslation();
  const { apiKeyPermissions } = usePermissions();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'apikeys-table-page-size',
  });

  const [activeTab, setActiveTab] = useState<ApiKeyTabKey>('all');

  // Filter states - following the same pattern as roles and users
  const [searchFilter, setSearchFilter] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [userFilter, setUserFilter] = useState<string[]>([]);
  const [dateRange, setDateRange] = useState<DateTimeRangeValue | undefined>();

  const debouncedSearchFilter = useDebounce(searchFilter, 300);

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

  const { data, isLoading } = useApiKeys({
    ...paginationArgs,
    where: whereClause,
    orderBy: { field: 'CREATED_AT', direction: 'DESC' },
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

  const handleResetFilters = () => {
    setSearchFilter('');
    setStatusFilter([]);
    setUserFilter([]);
    setDateRange(undefined);
    resetCursor();
  };

  const columns = React.useMemo(() => createColumns(t, apiKeyPermissions.canWrite), [t, apiKeyPermissions.canWrite]);

  return (
    <div className='flex flex-1 flex-col'>
      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as ApiKeyTabKey)} className='w-full'>
        <TabsList className='shadow-soft border-border bg-background grid w-full grid-cols-3 rounded-2xl border'>
          <TabsTrigger value='all' data-value='all'>
            {t('apikeys.tabs.all')}
          </TabsTrigger>
          <TabsTrigger value='user' data-value='user'>
            {t('apikeys.type.user')}
          </TabsTrigger>
          <TabsTrigger value='service_account' data-value='service_account'>
            {t('apikeys.type.service_account')}
          </TabsTrigger>
        </TabsList>
      </Tabs>
      <div className='mt-6 flex-1'>
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
          onNextPage={handleNextPage}
          onPreviousPage={handlePreviousPage}
          onPageSizeChange={handlePageSizeChange}
          onSearchFilterChange={setSearchFilter}
          onStatusFilterChange={setStatusFilter}
          onUserFilterChange={setUserFilter}
          onDateRangeChange={setDateRange}
          onResetFilters={handleResetFilters}
          canWrite={apiKeyPermissions.canWrite}
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

      <Main>
        <ApiKeysContent />
      </Main>
      <ApiKeysDialogs />
    </ApiKeysProvider>
  );
}
