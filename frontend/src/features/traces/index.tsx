import { useState, useCallback, useEffect, useMemo } from 'react';
import type { SortingState } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import {
  buildDateRangeWhereClause,
  buildLastRequestAtDateRangeWhereClause,
  type DateRangeField,
  type DateTimeRangeValue,
} from '@/utils/date-range';
import { useAutoRefreshInterval } from '@/hooks/use-auto-refresh-interval';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import useInterval from '@/hooks/useInterval';
import { Main } from '@/components/layout/main';
import { PageHeader } from '@/components/layout/page-header';
import { TracesTable } from './components';
import { TracesProvider } from './context';
import { useTraces } from './data';

const DEFAULT_SORTING: SortingState = [{ id: 'lastRequestAt', desc: true }];
const SORTABLE_COLUMN_IDS = new Set(['id', 'createdAt', 'lastRequestAt']);
const SORTING_STORAGE_KEY = 'traces-table-sorting';

function loadSorting(): SortingState {
  const fallback = () => DEFAULT_SORTING.map((item) => ({ ...item }));

  try {
    const stored = localStorage.getItem(SORTING_STORAGE_KEY);
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

function TracesContent() {
  const { startCursor, endCursor, pageSize, setCursors, setPageSize, resetCursor, paginationArgs, cursorHistory } =
    usePaginationSearch({
      defaultPageSize: 20,
      pageSizeStorageKey: 'traces-table-page-size',
    });
  const [dateRange, setDateRange] = useState<DateTimeRangeValue | undefined>();
  const [dateRangeField, setDateRangeField] = useState<DateRangeField>('lastRequestAt');
  const [traceIdFilter, setTraceIdFilter] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [sorting, setSorting] = useState<SortingState>(loadSorting);
  const [sortingCursorResetPending, setSortingCursorResetPending] = useState(false);
  const [autoRefreshInterval, setAutoRefreshInterval] = useAutoRefreshInterval('traces-auto-refresh-interval-ms');
  const debouncedTraceIdFilter = useDebounce(traceIdFilter, 300);

  useEffect(() => {
    try {
      localStorage.setItem(SORTING_STORAGE_KEY, JSON.stringify(sorting));
    } catch {
      // Storage may be unavailable; sorting still works for the current session.
    }
  }, [sorting]);

  const hasPaginationCursor = Boolean(startCursor || endCursor || cursorHistory.length > 0);

  useEffect(() => {
    if (sortingCursorResetPending && !hasPaginationCursor) {
      setSortingCursorResetPending(false);
    }
  }, [hasPaginationCursor, sortingCursorResetPending]);

  // Build where clause with filters
  const whereClause = (() => {
    const where: { [key: string]: any } = {
      ...(dateRangeField === 'createdAt'
        ? buildDateRangeWhereClause(dateRange)
        : buildLastRequestAtDateRangeWhereClause(dateRange)),
    };

    if (debouncedTraceIdFilter.trim()) {
      where.traceIDContains = debouncedTraceIdFilter.trim();
    }

    // Status filter: if specific statuses selected, use statusIn; otherwise default to non-archived
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter;
    } else {
      where.statusNEQ = 'archived';
    }

    return where;
  })();

  const currentOrderBy = useMemo(() => {
    if (sorting.length === 0) {
      return { field: 'LAST_REQUEST_AT', direction: 'DESC' } as const;
    }

    const [primary] = sorting;
    // TraceOrder has no ID field; creation order is the closest the server can
    // express for the id column.
    const field = primary.id === 'lastRequestAt' ? 'LAST_REQUEST_AT' : 'CREATED_AT';

    return { field, direction: primary.desc ? 'DESC' : 'ASC' } as const;
  }, [sorting]);

  const { data, isLoading, refetch } = useTraces({
    ...(sortingCursorResetPending ? { first: pageSize } : paginationArgs),
    where: whereClause,
    orderBy: currentOrderBy,
  });

  const traces = data?.edges?.map((edge) => edge.node) || [];
  const pageInfo = data?.pageInfo;
  const isFirstPage = !paginationArgs.after && cursorHistory.length === 0;

  const autoRefreshResumeKey = useInterval(
    () => {
      refetch();
    },
    isFirstPage ? autoRefreshInterval : null,
    { refreshOnResume: true }
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

  const handleDateRangeFieldChange = useCallback(
    (field: DateRangeField) => {
      setDateRangeField(field);
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

  const handleTraceIdFilterChange = useCallback(
    (traceId: string) => {
      setTraceIdFilter(traceId);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleStatusFilterChange = useCallback(
    (statuses: string[]) => {
      setStatusFilter(statuses);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const handleSortingChange = (updater: SortingState | ((previous: SortingState) => SortingState)) => {
    if (hasPaginationCursor) {
      setSortingCursorResetPending(true);
    }
    setSorting((previous) => (typeof updater === 'function' ? updater(previous) : updater));
    resetCursor();
  };

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <TracesTable
        data={traces}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        dateRange={dateRange}
        dateRangeField={dateRangeField}
        traceIdFilter={traceIdFilter}
        statusFilter={statusFilter}
        sorting={sorting}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onDateRangeChange={handleDateRangeChange}
        onDateRangeFieldChange={handleDateRangeFieldChange}
        onTraceIdFilterChange={handleTraceIdFilterChange}
        onStatusFilterChange={handleStatusFilterChange}
        onSortingChange={handleSortingChange}
        onRefresh={refetch}
        showRefresh={isFirstPage}
        autoRefreshInterval={autoRefreshInterval}
        autoRefreshResumeKey={autoRefreshResumeKey}
        onAutoRefreshIntervalChange={setAutoRefreshInterval}
      />
    </div>
  );
}

export default function TracesManagement() {
  const { t } = useTranslation();

  return (
    <TracesProvider>
      <PageHeader title={t('traces.title')} description={t('traces.description')} />

      <Main fixed>
        <TracesContent />
      </Main>
    </TracesProvider>
  );
}
