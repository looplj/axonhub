import { useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { buildDateRangeWhereClause, type DateTimeRangeValue } from '@/utils/date-range';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import useInterval from '@/hooks/useInterval';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { UsageLogsTable, UsageDetailDialog } from './components';
import { UsageLogsProvider, useUsageLogsContext } from './context';
import { useUsageLogs } from './data';

function UsageLogsContent() {
  const { t } = useTranslation();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs, cursorHistory } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'usage-logs-table-page-size',
  });
  const [sourceFilter, setSourceFilter] = useState<string[]>([]);
  const [channelFilter, setChannelFilter] = useState<string[]>([]);
  const [dateRange, setDateRange] = useState<DateTimeRangeValue | undefined>();
  const [autoRefresh, setAutoRefresh] = useState(false);

  // Build where clause with filters
  const whereClause = (() => {
    const where: { [key: string]: any } = {
      ...buildDateRangeWhereClause(dateRange),
    };

    if (sourceFilter.length > 0) {
      where.sourceIn = sourceFilter;
    }

    if (channelFilter.length > 0) {
      where.channelIDIn = channelFilter;
    }

    return Object.keys(where).length > 0 ? where : undefined;
  })();

  const { data, isLoading, error, refetch } = useUsageLogs({
    ...paginationArgs,
    orderBy: { field: 'CREATED_AT', direction: 'DESC' },
    where: whereClause,
  });

  const usageLogs = data?.edges?.map((edge) => edge.node) || [];
  const pageInfo = data?.pageInfo;
  const isFirstPage = !paginationArgs.after && cursorHistory.length === 0;

  useInterval(
    () => {
      refetch();
    },
    autoRefresh && isFirstPage ? 10000 : null
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
  };

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

  const handleDateRangeChange = useCallback(
    (range: DateTimeRangeValue | undefined) => {
      setDateRange(range);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  if (error) {
    return (
      <div className='flex h-64 items-center justify-center'>
        <p className='text-destructive'>
          {t('common.loadError')} {error.message}
        </p>
      </div>
    );
  }

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <UsageLogsTable
        data={usageLogs}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        sourceFilter={sourceFilter}
        channelFilter={channelFilter}
        dateRange={dateRange}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onSourceFilterChange={handleSourceFilterChange}
        onChannelFilterChange={handleChannelFilterChange}
        onDateRangeChange={handleDateRangeChange}
        onRefresh={refetch}
        showRefresh={isFirstPage}
        autoRefresh={autoRefresh}
        onAutoRefreshChange={setAutoRefresh}
      />
    </div>
  );
}

function UsageLogsDialogs() {
  const { detailDialogOpen, setDetailDialogOpen, currentUsageLog: selectedUsageLog } = useUsageLogsContext();

  return (
    <>
      {/* Usage detail dialog */}
      <UsageDetailDialog open={detailDialogOpen} onOpenChange={setDetailDialogOpen} usageLogId={selectedUsageLog?.id} />
    </>
  );
}

export default function UsageLogsManagement() {
  const { t } = useTranslation();

  return (
    <UsageLogsProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('usageLogs.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('usageLogs.description')}</p>
          </div>
        </div>
      </Header>

      <Main fixed>
        <UsageLogsContent />
      </Main>
      <UsageLogsDialogs />
    </UsageLogsProvider>
  );
}
