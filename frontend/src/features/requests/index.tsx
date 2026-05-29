import { useState, useCallback, useMemo } from 'react';
import { useNavigate, useRouterState } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import {
  DEFAULT_END_TIME,
  DEFAULT_START_TIME,
  buildDateRangeWhereClause,
  isSameTime,
  normalizeDateTimeRangeValue,
  type DateTimeRangeValue,
  type TimeValue,
} from '@/utils/date-range';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import useInterval from '@/hooks/useInterval';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { RequestsTable, type RequestTableFilters } from './components';
import { RequestsProvider } from './context';
import { useRequests } from './data';

const REQUEST_FILTER_SEARCH_KEYS = {
  status: 'status',
  source: 'source',
  channel: 'channel',
  apiKey: 'apiKey',
  modelID: 'modelID',
  createdAtFrom: 'createdAtFrom',
  createdAtTo: 'createdAtTo',
  createdAtStartTime: 'createdAtStartTime',
  createdAtEndTime: 'createdAtEndTime',
} as const;

const REQUEST_CURSOR_SEARCH_KEYS = ['startCursor', 'endCursor', 'cursorDirection', 'cursorHistory'] as const;

type RequestSearchFilters = RequestTableFilters & {
  dateRange?: DateTimeRangeValue;
};

function getSearchString(value: unknown): string {
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) {
    const first = value.find((item): item is string => typeof item === 'string');
    return first ?? '';
  }
  return '';
}

function getSearchStringArray(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.filter((item): item is string => typeof item === 'string' && item.length > 0);
  }

  if (typeof value !== 'string' || value.length === 0) {
    return [];
  }

  try {
    const parsed = JSON.parse(value);
    if (Array.isArray(parsed)) {
      return parsed.filter((item): item is string => typeof item === 'string' && item.length > 0);
    }
  } catch {
    // Fall through to comma-separated search values.
  }

  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function setSearchStringArray(draft: Record<string, unknown>, key: string, value: string[]) {
  if (value.length > 0) {
    draft[key] = value;
  } else {
    delete draft[key];
  }
}

function setSearchString(draft: Record<string, unknown>, key: string, value: string) {
  const normalized = value.trim();
  if (normalized) {
    draft[key] = normalized;
  } else {
    delete draft[key];
  }
}

function pad2(value: number) {
  return value.toString().padStart(2, '0');
}

function formatSearchDate(date: Date) {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`;
}

function parseSearchDate(value: unknown): Date | undefined {
  const raw = getSearchString(value);
  if (!raw) return undefined;

  const dateOnly = raw.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (dateOnly) {
    const year = Number.parseInt(dateOnly[1], 10);
    const month = Number.parseInt(dateOnly[2], 10);
    const day = Number.parseInt(dateOnly[3], 10);
    const date = new Date(year, month - 1, day);

    if (date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day) {
      return date;
    }
    return undefined;
  }

  const date = new Date(raw);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

function clampTimePart(value: string | undefined, max: number, fallback: string) {
  const parsed = Number.parseInt(value ?? '', 10);
  if (!Number.isFinite(parsed)) return fallback;
  return pad2(Math.min(Math.max(parsed, 0), max));
}

function formatSearchTime(time: TimeValue) {
  return `${time.hh}:${time.mm}:${time.ss}`;
}

function parseSearchTime(value: unknown, fallback: TimeValue): TimeValue {
  const raw = getSearchString(value);
  if (!raw) return fallback;

  const [hh, mm, ss] = raw.split(':');
  return {
    hh: clampTimePart(hh, 23, fallback.hh),
    mm: clampTimePart(mm, 59, fallback.mm),
    ss: clampTimePart(ss, 59, fallback.ss),
  };
}

function parseRequestSearchFilters(search: Record<string, unknown>): RequestSearchFilters {
  const from = parseSearchDate(search[REQUEST_FILTER_SEARCH_KEYS.createdAtFrom]);
  const to = parseSearchDate(search[REQUEST_FILTER_SEARCH_KEYS.createdAtTo]);
  const hasDateRange = !!from || !!to;

  return {
    statusFilter: getSearchStringArray(search[REQUEST_FILTER_SEARCH_KEYS.status]),
    sourceFilter: getSearchStringArray(search[REQUEST_FILTER_SEARCH_KEYS.source]),
    channelFilter: getSearchStringArray(search[REQUEST_FILTER_SEARCH_KEYS.channel]),
    apiKeyFilter: getSearchStringArray(search[REQUEST_FILTER_SEARCH_KEYS.apiKey]),
    modelIDFilter: getSearchString(search[REQUEST_FILTER_SEARCH_KEYS.modelID]),
    dateRange: hasDateRange
      ? normalizeDateTimeRangeValue({
          from,
          to,
          startTime: parseSearchTime(search[REQUEST_FILTER_SEARCH_KEYS.createdAtStartTime], DEFAULT_START_TIME),
          endTime: parseSearchTime(search[REQUEST_FILTER_SEARCH_KEYS.createdAtEndTime], DEFAULT_END_TIME),
        })
      : undefined,
  };
}

function clearRequestCursorSearch(draft: Record<string, unknown>) {
  REQUEST_CURSOR_SEARCH_KEYS.forEach((key) => {
    delete draft[key];
  });
}

function clearRequestFilterSearch(draft: Record<string, unknown>) {
  Object.values(REQUEST_FILTER_SEARCH_KEYS).forEach((key) => {
    delete draft[key];
  });
}

function RequestsContent() {
  const navigate = useNavigate();
  const currentSearch = useRouterState({
    select: (state) => (state.location.search ?? {}) as Record<string, unknown>,
  });
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs, cursorHistory } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'requests-table-page-size',
  });
  const { statusFilter, sourceFilter, channelFilter, apiKeyFilter, modelIDFilter, dateRange } = useMemo(
    () => parseRequestSearchFilters(currentSearch),
    [currentSearch]
  );
  const debouncedModelIDFilter = useDebounce(modelIDFilter, 300);
  const [autoRefresh, setAutoRefresh] = useState(false);

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
    if (debouncedModelIDFilter) {
      where.modelIDContainsFold = debouncedModelIDFilter;
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
    resetCursor();
  };

  const updateRequestSearch = useCallback(
    (apply: (draft: Record<string, unknown>) => void) => {
      navigate({
        search: (prev: Record<string, unknown> | undefined) => {
          const draft = { ...((prev ?? {}) as Record<string, unknown>) };
          apply(draft);
          clearRequestCursorSearch(draft);
          return draft;
        },
        replace: true,
      });
    },
    [navigate]
  );

  const handleFiltersChange = useCallback(
    (filters: RequestTableFilters) => {
      updateRequestSearch((draft) => {
        setSearchStringArray(draft, REQUEST_FILTER_SEARCH_KEYS.status, filters.statusFilter);
        setSearchStringArray(draft, REQUEST_FILTER_SEARCH_KEYS.source, filters.sourceFilter);
        setSearchStringArray(draft, REQUEST_FILTER_SEARCH_KEYS.channel, filters.channelFilter);
        setSearchStringArray(draft, REQUEST_FILTER_SEARCH_KEYS.apiKey, filters.apiKeyFilter);
        setSearchString(draft, REQUEST_FILTER_SEARCH_KEYS.modelID, filters.modelIDFilter);
      });
    },
    [updateRequestSearch]
  );

  const handleDateRangeChange = useCallback(
    (range: DateTimeRangeValue | undefined) => {
      updateRequestSearch((draft) => {
        const normalizedRange = range ? normalizeDateTimeRangeValue(range) : undefined;

        if (!normalizedRange || (!normalizedRange.from && !normalizedRange.to)) {
          delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtFrom];
          delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtTo];
          delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtStartTime];
          delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtEndTime];
          return;
        }

        if (normalizedRange.from) {
          draft[REQUEST_FILTER_SEARCH_KEYS.createdAtFrom] = formatSearchDate(normalizedRange.from);
          if (isSameTime(normalizedRange.startTime, DEFAULT_START_TIME)) {
            delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtStartTime];
          } else {
            draft[REQUEST_FILTER_SEARCH_KEYS.createdAtStartTime] = formatSearchTime(normalizedRange.startTime);
          }
        } else {
          delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtFrom];
          delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtStartTime];
        }

        if (normalizedRange.to) {
          draft[REQUEST_FILTER_SEARCH_KEYS.createdAtTo] = formatSearchDate(normalizedRange.to);
          if (isSameTime(normalizedRange.endTime, DEFAULT_END_TIME)) {
            delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtEndTime];
          } else {
            draft[REQUEST_FILTER_SEARCH_KEYS.createdAtEndTime] = formatSearchTime(normalizedRange.endTime);
          }
        } else {
          delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtTo];
          delete draft[REQUEST_FILTER_SEARCH_KEYS.createdAtEndTime];
        }
      });
    },
    [updateRequestSearch]
  );

  const handleResetFilters = useCallback(() => {
    updateRequestSearch(clearRequestFilterSearch);
  }, [updateRequestSearch]);

  const handleViewDetail = useCallback(
    (requestId: string) => {
      navigate({
        to: '/project/requests/$requestId',
        params: { requestId },
        search: currentSearch,
      });
    },
    [navigate, currentSearch]
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
        modelIDFilter={modelIDFilter}
        dateRange={dateRange}
        queryWhere={whereClause}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onFiltersChange={handleFiltersChange}
        onDateRangeChange={handleDateRangeChange}
        onResetFilters={handleResetFilters}
        onViewDetail={handleViewDetail}
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
            <p className='text-muted-foreground text-sm'>{t('requests.description')}</p>
          </div>
        </div>
      </Header>

      <Main fixed>
        <RequestsContent />
      </Main>
    </RequestsProvider>
  );
}
