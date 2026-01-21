import { useState, useMemo, useCallback, useEffect } from 'react';
import { SortingState } from '@tanstack/react-table';
import { IconPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Button } from '@/components/ui/button';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { PermissionGuard } from '@/components/permission-guard';
import { createColumns } from './components/prompts-columns';
import { PromptsDialogs } from './components/prompts-dialogs';
import { PromptsTable } from './components/prompts-table';
import PromptsProvider, { usePrompts } from './context/prompts-context';
import { useQueryPrompts } from './data/prompts';

function PromptsContent() {
  const { t } = useTranslation();
  const { hasScope } = usePermissions();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'prompts-table-page-size',
  });
  const [nameFilter, setNameFilter] = useState<string>('');
  const [sorting, setSorting] = useState<SortingState>(() => {
    const stored = localStorage.getItem('prompts-table-sorting');
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
    localStorage.setItem('prompts-table-sorting', JSON.stringify(sorting));
  }, [sorting]);

  const debouncedNameFilter = useDebounce(nameFilter, 300);

  const whereClause = (() => {
    if (debouncedNameFilter) {
      return {
        nameContainsFold: debouncedNameFilter,
      };
    }
    return undefined;
  })();

  const currentOrderBy = (() => {
    if (sorting.length === 0) {
      return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
    const [primary] = sorting;
    switch (primary.id) {
      case 'name':
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'createdAt':
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      default:
        return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
  })();

  const { data, isLoading } = useQueryPrompts({
    ...paginationArgs,
    where: whereClause,
    orderBy: currentOrderBy,
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [setNameFilter]
  );

  const canWrite = hasScope('write_prompts');
  const columns = useMemo(() => createColumns(t, canWrite), [t, canWrite]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <PromptsTable
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
        canWrite={canWrite}
      />
    </div>
  );
}

function CreateButton() {
  const { t } = useTranslation();
  const { setOpen } = usePrompts();

  return (
    <Button onClick={() => setOpen('create')}>
      <IconPlus className='mr-2 h-4 w-4' />
      {t('prompts.actions.create')}
    </Button>
  );
}

function ActionButtons() {
  return (
    <div className='flex gap-2'>
      <PermissionGuard requiredScope='write_prompts'>
        <CreateButton />
      </PermissionGuard>
    </div>
  );
}

export default function PromptsManagement() {
  const { t } = useTranslation();

  return (
    <PromptsProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('prompts.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('prompts.description')}</p>
          </div>
          <ActionButtons />
        </div>
      </Header>

      <Main fixed>
        <PromptsContent />
      </Main>
      <PromptsDialogs />
    </PromptsProvider>
  );
}
