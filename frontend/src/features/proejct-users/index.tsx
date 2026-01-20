import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
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
  const { userPermissions, rolePermissions } = usePermissions();
  const { pageSize, setCursors, setPageSize, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'project-users-table-page-size',
  });

  // Memoize columns to prevent infinite re-renders
  const columns = useMemo(
    () => createColumns(t, userPermissions.canWrite, rolePermissions.canRead),
    [t, userPermissions.canWrite, rolePermissions.canRead]
  );

  const {
    data,
    isLoading,
    error: _error,
  } = useUsers({
    ...paginationArgs,
  });

  const projectUsers = data?.edges?.map((edge) => edge.node) || [];
  const pageInfo = data?.pageInfo;

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

  const handlePageSizeChange = (nextPageSize: number) => {
    setPageSize(nextPageSize);
  };

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <UsersTable
        data={projectUsers}
        columns={columns}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        nameFilter={''}
        statusFilter={[]}
        roleFilter={[]}
        onNameFilterChange={() => {}}
        onStatusFilterChange={() => {}}
        onRoleFilterChange={() => {}}
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
            <h2 className='text-xl font-bold tracking-tight'>{t('projectUsers.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('projectUsers.description')}</p>
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
