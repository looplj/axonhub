import { useCallback, useEffect, useMemo, useState } from 'react';
import { SortingState } from '@tanstack/react-table';
import { IconPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Button } from '@/components/ui/button';
import { Main } from '@/components/layout/main';
import { PageHeader } from '@/components/layout/page-header';
import { PermissionGuard } from '@/components/permission-guard';
import PromptProtectionRulesProvider, { usePromptProtectionRules } from './context/rules-context';
import { createColumns } from './components/rules-columns';
import { RulesDialogs } from './components/rules-dialogs';
import { RulesTable } from './components/rules-table';
import { useQueryPromptProtectionRules } from './data/rules';

function RulesContent() {
  const { t } = useTranslation();
  const { modelPermissions } = usePermissions();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'prompt-protection-rules-table-page-size',
  });
  const [nameFilter, setNameFilter] = useState('');
  const [sorting, setSorting] = useState<SortingState>(() => {
    const stored = localStorage.getItem('prompt-protection-rules-table-sorting');
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
    localStorage.setItem('prompt-protection-rules-table-sorting', JSON.stringify(sorting));
  }, [sorting]);

  const debouncedNameFilter = useDebounce(nameFilter, 300);
  const whereClause = debouncedNameFilter
    ? {
        nameContainsFold: debouncedNameFilter,
      }
    : undefined;

  const currentOrderBy = (() => {
    if (sorting.length === 0) {
      return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
    const [primary] = sorting;
    switch (primary.id) {
      case 'name':
        return { field: 'NAME', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'createdAt':
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      default:
        return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
  })();

  const { data, isLoading } = useQueryPromptProtectionRules({
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
    [resetCursor]
  );

  const columns = useMemo(() => createColumns(t, modelPermissions.canWrite), [modelPermissions.canWrite, t]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <RulesTable
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
        canWrite={modelPermissions.canWrite}
      />
    </div>
  );
}

function CreateButton() {
  const { t } = useTranslation();
  const { setOpen, setCurrentRow } = usePromptProtectionRules();

  return (
    <Button
      onClick={() => {
        setCurrentRow(null);
        setOpen('create');
      }}
    >
      <IconPlus className='mr-2 h-4 w-4' />
      {t('promptProtectionRules.actions.create')}
    </Button>
  );
}

function ActionButtons() {
  return (
    <div className='flex min-w-0 max-w-full flex-wrap items-center gap-2'>
      <PermissionGuard requiredScope='write_channels'>
        <CreateButton />
      </PermissionGuard>
    </div>
  );
}

export default function PromptProtectionRulesManagement() {
  const { t } = useTranslation();

  return (
    <PromptProtectionRulesProvider>
      <PageHeader title={t('promptProtectionRules.title')} description={t('promptProtectionRules.description')} actions={<ActionButtons />} />

      <Main fixed>
        <RulesContent />
      </Main>
      <RulesDialogs />
    </PromptProtectionRulesProvider>
  );
}
