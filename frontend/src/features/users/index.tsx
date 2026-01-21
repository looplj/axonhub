import React, { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { createColumns } from './components/users-columns';
import { UsersDialogs } from './components/users-dialogs';
import { UsersPrimaryButtons } from './components/users-primary-buttons';
import { UsersTable } from './components/users-table';
import UsersProvider from './context/users-context';
import { useUsers } from './data/users';

function UsersContent() {
  const { t } = useTranslation();
  const { userPermissions } = usePermissions();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'users-table-page-size',
  });

  // Filter states
  const [nameFilter, setNameFilter] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [roleFilter, setRoleFilter] = useState<string[]>([]);

  const debouncedNameFilter = useDebounce(nameFilter, 300);

  // Memoize columns to prevent infinite re-renders
  const columns = useMemo(() => createColumns(t, userPermissions.canWrite), [t, userPermissions.canWrite]);

  // Build where clause for API filtering
  const whereClause = (() => {
    const where: Record<string, string | string[]> = {};
    if (debouncedNameFilter) {
      where.firstNameContainsFold = debouncedNameFilter;
    }
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter;
    }
    if (roleFilter.length > 0) {
      // Note: This would need to be implemented based on the actual user role relationship
      // For now, we'll leave it as a placeholder
    }
    return Object.keys(where).length > 0 ? where : undefined;
  })();

  const {
    data,
    isLoading,
    error: _error,
  } = useUsers({
    ...paginationArgs,
    where: whereClause,
  });

  // Reset cursor when filters change
  React.useEffect(() => {
    resetCursor();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedNameFilter, statusFilter, roleFilter]);

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

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <UsersTable
        data={data?.edges?.map((edge) => edge.node) || []}
        columns={columns}
        loading={isLoading}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        nameFilter={nameFilter}
        statusFilter={statusFilter}
        roleFilter={roleFilter}
        onNameFilterChange={setNameFilter}
        onStatusFilterChange={setStatusFilter}
        onRoleFilterChange={setRoleFilter}
      />
    </div>
  );
}

export default function UsersManagement() {
  const { t } = useTranslation();

  return (
    <UsersProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('users.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('users.description')}</p>
          </div>
          <UsersPrimaryButtons />
        </div>
      </Header>

      <Main fixed>
        <UsersContent />
      </Main>
      <UsersDialogs />
    </UsersProvider>
  );
}
