import { useState, useCallback } from 'react';
import { DateRange } from 'react-day-picker';
import { useTranslation } from 'react-i18next';
import { buildDateRangeWhereClause } from '@/utils/date-range';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import useInterval from '@/hooks/useInterval';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { TracesTable } from './components';
import { TracesProvider } from './context';
import { useTraces } from './data';

function TracesContent() {
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs, cursorHistory } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'traces-table-page-size',
  });
  const [dateRange, setDateRange] = useState<DateRange | undefined>();
  const [traceIdFilter, setTraceIdFilter] = useState<string>('');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const debouncedTraceIdFilter = useDebounce(traceIdFilter, 300);

  // Build where clause with filters
  const whereClause = (() => {
    const where: { [key: string]: any } = {
      ...buildDateRangeWhereClause(dateRange),
    };

    if (debouncedTraceIdFilter.trim()) {
      where.traceIDContains = debouncedTraceIdFilter.trim();
    }

    return Object.keys(where).length > 0 ? where : undefined;
  })();

  const { data, isLoading, refetch } = useTraces({
    ...paginationArgs,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  });

  const traces = data?.edges?.map((edge) => edge.node) || [];
  const pageInfo = data?.pageInfo;
  const isFirstPage = !paginationArgs.after && cursorHistory.length === 0;

  useInterval(
    () => {
      refetch();
    },
    autoRefresh && isFirstPage ? 30000 : null
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

  const handleDateRangeChange = useCallback(
    (range: DateRange | undefined) => {
      setDateRange(range);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleTraceIdFilterChange = useCallback(
    (traceId: string) => {
      setTraceIdFilter(traceId);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <TracesTable
        data={traces}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        dateRange={dateRange}
        traceIdFilter={traceIdFilter}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onDateRangeChange={handleDateRangeChange}
        onTraceIdFilterChange={handleTraceIdFilterChange}
        onRefresh={refetch}
        showRefresh={isFirstPage}
        autoRefresh={autoRefresh}
        onAutoRefreshChange={setAutoRefresh}
      />
    </div>
  );
}

export default function TracesManagement() {
  const { t } = useTranslation();

  return (
    <TracesProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('traces.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('traces.description')}</p>
          </div>
        </div>
      </Header>

      <Main fixed>
        <TracesContent />
      </Main>
    </TracesProvider>
  );
}
