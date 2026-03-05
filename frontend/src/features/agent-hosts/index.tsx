import { useState, useMemo, useCallback, useEffect, lazy, Suspense } from 'react';
import { SortingState } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { createColumns } from './components/agent-hosts-columns';
import { AgentHostsPrimaryButtons } from './components/agent-hosts-primary-buttons';
import { AgentHostsTable } from './components/agent-hosts-table';
import AgentHostsProvider from './context/agent-hosts-context';
import { useQueryAgentHosts } from './data/agent-hosts';

const AgentHostsDialogs = lazy(() =>
  import('./components/agent-hosts-dialogs').then((m) => ({ default: m.AgentHostsDialogs }))
);

function AgentHostsContent() {
  const { t } = useTranslation();
  const { agentHostsPermissions } = usePermissions();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'agent-hosts-table-page-size',
  });
  const [nameFilter, setNameFilter] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string[]>([]);
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [sorting, setSorting] = useState<SortingState>(() => {
    const stored = localStorage.getItem('agent-hosts-table-sorting');
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
    localStorage.setItem('agent-hosts-table-sorting', JSON.stringify(sorting));
  }, [sorting]);

  // Debounce the name filter to avoid excessive API calls
  const debouncedNameFilter = useDebounce(nameFilter, 300);

  // Build where clause with filters using useMemo
  const whereClause = useMemo(() => {
    const where: Record<string, string | string[]> = {};
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter;
    }
    if (typeFilter.length > 0) {
      where.typeIn = typeFilter;
    }
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter;
    }
    return Object.keys(where).length > 0 ? where : undefined;
  }, [debouncedNameFilter, typeFilter, statusFilter]);

  const currentOrderBy = useMemo(() => {
    if (sorting.length === 0) {
      return { field: 'CREATED_AT' as const, direction: 'DESC' as const };
    }
    const [primary] = sorting;
    switch (primary.id) {
      case 'createdAt':
        return { field: 'CREATED_AT' as const, direction: primary.desc ? 'DESC' : 'ASC' };
      case 'updatedAt':
        return { field: 'UPDATED_AT' as const, direction: primary.desc ? 'DESC' : 'ASC' };
      default:
        return { field: 'CREATED_AT' as const, direction: 'DESC' };
    }
  }, [sorting]);

  const {
    data,
    isLoading,
    error: _error,
  } = useQueryAgentHosts({
    ...paginationArgs,
    where: whereClause,
    orderBy: currentOrderBy,
  });

  const agentHosts = useMemo(() => {
    return data?.edges?.map((edge) => edge.node) || [];
  }, [data?.edges]);

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

  const handleStatusFilterChange = useCallback(
    (filters: string[]) => {
      setStatusFilter(filters);
      resetCursor();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  const columns = useMemo(
    () => createColumns(t, agentHostsPermissions.canWrite),
    [t, agentHostsPermissions.canWrite]
  );

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <AgentHostsTable
        loading={isLoading}
        data={agentHosts}
        columns={columns}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        nameFilter={nameFilter}
        typeFilter={typeFilter}
        statusFilter={statusFilter}
        sorting={sorting}
        onSortingChange={setSorting}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onResetCursor={resetCursor}
        onNameFilterChange={handleNameFilterChange}
        onTypeFilterChange={handleTypeFilterChange}
        onStatusFilterChange={handleStatusFilterChange}
        canWrite={agentHostsPermissions.canWrite}
      />
    </div>
  );
}

export default function AgentHostsManagement() {
  const { t } = useTranslation();

  return (
    <AgentHostsProvider>
      <Header fixed>
        <div className="flex w-full flex-1 flex-col gap-2 md:flex-row md:items-center md:justify-between md:gap-0">
          <div className="min-w-0">
            <h2 className="text-xl font-bold tracking-tight">{t('agentHosts.title')}</h2>
            <p className="text-sm text-muted-foreground">{t('agentHosts.description')}</p>
          </div>
          <AgentHostsPrimaryButtons />
        </div>
      </Header>

      <Main fixed>
        <AgentHostsContent />
      </Main>
      <Suspense fallback={null}>
        <AgentHostsDialogs />
      </Suspense>
    </AgentHostsProvider>
  );
}
