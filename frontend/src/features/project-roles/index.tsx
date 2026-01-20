'use client';

import React, { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { RolesDialogs } from './components/roles-action-dialog';
import { createColumns } from './components/roles-columns';
import { RolesPrimaryButtons } from './components/roles-primary-buttons';
import { RolesTable } from './components/roles-table';
import RolesProvider from './context/roles-context';
import { useRoles } from './data/roles';

function RolesContent() {
  const { t } = useTranslation();
  const { rolePermissions } = usePermissions();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'project-roles-table-page-size',
  });

  // Filter states - search by name
  const [searchFilter, setSearchFilter] = useState<string>('');

  const debouncedSearchFilter = useDebounce(searchFilter, 300);

  // Memoize columns to prevent infinite re-renders
  const columns = useMemo(() => createColumns(t, rolePermissions.canWrite), [t, rolePermissions.canWrite]);

  // Build where clause for API filtering with OR logic
  const whereClause = (() => {
    if (!debouncedSearchFilter) {
      return undefined;
    }

    // Search by name
    return {
      nameContainsFold: debouncedSearchFilter,
    };
  })();

  const {
    data,
    isLoading,
    error: _error,
  } = useRoles({
    ...paginationArgs,
    where: whereClause,
  });

  // Reset cursor when filters change
  React.useEffect(() => {
    resetCursor();
  }, [debouncedSearchFilter, resetCursor]);

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
      <RolesTable
        columns={columns}
        data={data?.edges?.map((edge) => edge.node) || []}
        loading={isLoading}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        searchFilter={searchFilter}
        onSearchFilterChange={setSearchFilter}
      />
    </div>
  );
}

export default function RolesPage() {
  const { t } = useTranslation();

  return (
    <RolesProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('projectRoles.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('projectRoles.description')}</p>
          </div>
          <RolesPrimaryButtons />
        </div>
      </Header>

      <Main fixed>
        <RolesContent />
      </Main>
      <RolesDialogs />
    </RolesProvider>
  );
}
