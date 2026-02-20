import { useEffect, useMemo, useState } from 'react';
import { SortingState } from '@tanstack/react-table';
import { IconPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Button } from '@/components/ui/button';
import { PermissionGuard } from '@/components/permission-guard';
import AgentsProvider, { useAgents } from './context/agents-context';
import { useQueryAgents } from './data/agents';
import { createColumns } from './components/agents-columns';
import { AgentsTable } from './components/agents-table';
import { AgentsDialogs } from './components/agents-dialogs';

function AgentsContent() {
  const { t } = useTranslation();
  const { hasScope } = usePermissions();
  const { setOpen } = useAgents();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'agents-table-page-size',
  });

  const [nameFilter, setNameFilter] = useState('');
  const [sorting, setSorting] = useState<SortingState>(() => {
    const stored = localStorage.getItem('agents-table-sorting');
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
    localStorage.setItem('agents-table-sorting', JSON.stringify(sorting));
  }, [sorting]);

  const debouncedNameFilter = useDebounce(nameFilter, 300);

  const whereClause = useMemo(() => {
    if (debouncedNameFilter) return { nameContainsFold: debouncedNameFilter };
    return undefined;
  }, [debouncedNameFilter]);

  const currentOrderBy = useMemo(() => {
    if (sorting.length === 0) return { field: 'CREATED_AT', direction: 'DESC' } as const;
    const [primary] = sorting;
    switch (primary.id) {
      case 'createdAt':
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      default:
        return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
  }, [sorting]);

  const { data, isLoading } = useQueryAgents({
    ...paginationArgs,
    where: whereClause,
    orderBy: currentOrderBy,
  });

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

  const handleNameFilterChange = (filter: string) => {
    setNameFilter(filter);
    resetCursor();
  };

  const canWrite = hasScope('write_agents');
  const columns = useMemo(() => createColumns(t, canWrite), [t, canWrite]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <AgentsTable
        data={data?.edges?.map((edge) => edge.node) || []}
        columns={columns}
        loading={isLoading}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        nameFilter={nameFilter}
        sorting={sorting}
        onSortingChange={setSorting}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onNameFilterChange={handleNameFilterChange}
      />

      <AgentsDialogs />

      <PermissionGuard requiredScope='write_agents'>
        <div className='fixed bottom-6 right-6'>
          <Button onClick={() => setOpen('create')} className='shadow-lg'>
            <IconPlus className='mr-2 h-4 w-4' />
            {t('agents.actions.create')}
          </Button>
        </div>
      </PermissionGuard>
    </div>
  );
}

export default function AgentsManagement() {
  const { t } = useTranslation();

  return (
    <AgentsProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('agents.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('agents.description')}</p>
          </div>
        </div>
      </Header>

      <Main fixed>
        <AgentsContent />
      </Main>
    </AgentsProvider>
  );
}

