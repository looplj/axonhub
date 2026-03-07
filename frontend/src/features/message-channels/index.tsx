import { useCallback, useEffect, useMemo, useState } from 'react';
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
import MessageChannelsProvider from './context/message-channels-context';
import { useQueryMessageChannels } from './data/message-channels';
import { createColumns } from './components/message-channels-columns';
import { MessageChannelsTable } from './components/message-channels-table';
import { MessageChannelsDialogs } from './components/message-channels-dialogs';
import { useMessageChannels } from './context/message-channels-context';

function MessageChannelsContent() {
  const { t } = useTranslation();
  const { hasScope } = usePermissions();
  const { setOpen } = useMessageChannels();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'message-channels-table-page-size',
  });

  const [nameFilter, setNameFilter] = useState('');
  const [sorting, setSorting] = useState<SortingState>(() => {
    const stored = localStorage.getItem('message-channels-table-sorting');
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
    localStorage.setItem('message-channels-table-sorting', JSON.stringify(sorting));
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
      case 'type':
        return { field: 'TYPE', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'status':
        return { field: 'STATUS', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      default:
        return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
  }, [sorting]);

  const { data, isLoading } = useQueryMessageChannels({
    ...paginationArgs,
    where: whereClause,
    orderBy: currentOrderBy,
  });

  const messageChannels = useMemo(() => data?.edges?.map((edge) => edge.node) || [], [data?.edges]);

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

  const canWrite = hasScope('write_agents');
  const columns = useMemo(() => createColumns(t, canWrite), [t, canWrite]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <MessageChannelsTable
        data={messageChannels}
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

      <MessageChannelsDialogs />
    </div>
  );
}

function HeaderContent() {
  const { t } = useTranslation();
  const { setOpen } = useMessageChannels();

  return (
    <div className='flex w-full flex-1 flex-col gap-2 md:flex-row md:items-center md:justify-between md:gap-0'>
      <div className='min-w-0'>
        <h2 className='text-xl font-bold tracking-tight'>{t('messageChannels.title')}</h2>
        <p className='text-sm text-muted-foreground'>{t('messageChannels.description')}</p>
      </div>
      <PermissionGuard requiredScope='write_agents'>
        <Button className='shrink-0 space-x-1' onClick={() => setOpen('create')}>
          <span>{t('messageChannels.actions.create')}</span> <IconPlus size={18} />
        </Button>
      </PermissionGuard>
    </div>
  );
}

export default function MessageChannelsManagement() {
  return (
    <MessageChannelsProvider>
      <Header fixed>
        <HeaderContent />
      </Header>

      <Main fixed>
        <MessageChannelsContent />
      </Main>
    </MessageChannelsProvider>
  );
}
