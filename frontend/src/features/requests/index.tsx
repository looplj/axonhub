import { useState, useCallback } from 'react';
import { DateRange } from 'react-day-picker';
import { useTranslation } from 'react-i18next';
import { buildDateRangeWhereClause } from '@/utils/date-range';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import useInterval from '@/hooks/useInterval';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { RequestsTable } from './components';
import { RequestsProvider } from './context';
import { useRequests } from './data';
import { useRequestsSSE } from './hooks/useRequestsSSE';
import { useSelectedProjectId } from '@/stores/projectStore';

function RequestsContent() {
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs, cursorHistory } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'requests-table-page-size',
  });
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [sourceFilter, setSourceFilter] = useState<string[]>([]);
  const [channelFilter, setChannelFilter] = useState<string[]>([]);
  const [apiKeyFilter, setApiKeyFilter] = useState<string[]>([]);
  const [dateRange, setDateRange] = useState<DateRange | undefined>();
  const [autoRefresh, setAutoRefresh] = useState(false);
  const selectedProjectId = useSelectedProjectId();

  // Build where clause with filters
  const whereClause = (() => {
    const where: { [key: string]: any } = {
      ...buildDateRangeWhereClause(dateRange),
    };
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter;
    }
    if (sourceFilter.length > 0) {
      where.sourceIn = sourceFilter;
    }
    if (channelFilter.length > 0) {
      where.channelIDIn = channelFilter;
    }
    if (apiKeyFilter.length > 0) {
      where.apiKeyIDIn = apiKeyFilter;
    }
    return Object.keys(where).length > 0 ? where : undefined;
  })();

  const { data, isLoading, refetch } = useRequests({
    ...paginationArgs,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  });

  const requests = data?.edges?.map((edge) => edge.node) || [];
  const pageInfo = data?.pageInfo;

  const isFirstPage = !paginationArgs.after && cursorHistory.length === 0;

  // Use SSE for real-time updates when enabled and on first page
  const projectId = selectedProjectId ? parseInt(selectedProjectId, 10) : 0;
  
  const sseState = useRequestsSSE({
    enabled: autoRefresh && isFirstPage && projectId > 0,
    projectId,
  });

  // Fallback polling only if SSE is not connected
  // This provides graceful degradation if SSE fails
  const shouldPoll = autoRefresh && isFirstPage && !sseState.isConnected;
  
  useInterval(
    () => {
      refetch();
    },
    shouldPoll ? 10000 : null
  );

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
    resetCursor();
  };

  const handleStatusFilterChange = useCallback(
    (filters: string[]) => {
      setStatusFilter(filters);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleSourceFilterChange = useCallback(
    (filters: string[]) => {
      setSourceFilter(filters);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleChannelFilterChange = useCallback(
    (filters: string[]) => {
      setChannelFilter(filters);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleApiKeyFilterChange = useCallback(
    (filters: string[]) => {
      setApiKeyFilter(filters);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleDateRangeChange = useCallback(
    (range: DateRange | undefined) => {
      setDateRange(range);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <RequestsTable
        data={requests}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        statusFilter={statusFilter}
        sourceFilter={sourceFilter}
        channelFilter={channelFilter}
        apiKeyFilter={apiKeyFilter}
        dateRange={dateRange}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onStatusFilterChange={handleStatusFilterChange}
        onSourceFilterChange={handleSourceFilterChange}
        onChannelFilterChange={handleChannelFilterChange}
        onApiKeyFilterChange={handleApiKeyFilterChange}
        onDateRangeChange={handleDateRangeChange}
        onRefresh={refetch}
        showRefresh={isFirstPage}
        autoRefresh={autoRefresh}
        onAutoRefreshChange={setAutoRefresh}
      />
    </div>
  );
}

export default function RequestsManagement() {
  const { t } = useTranslation();

  return (
    <RequestsProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('requests.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('requests.description')}</p>
          </div>
        </div>
      </Header>

      <Main fixed>
        <RequestsContent />
      </Main>
    </RequestsProvider>
  );
}
