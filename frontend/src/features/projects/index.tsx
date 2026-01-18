'use client';

import React, { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { ProjectsDialogs } from './components/projects-action-dialog';
import { createColumns } from './components/projects-columns';
import { ProjectsPrimaryButtons } from './components/projects-primary-buttons';
import { ProjectsTable } from './components/projects-table';
import ProjectsProvider from './context/projects-context';
import { useProjects } from './data/projects';

function ProjectsContent() {
  const { t } = useTranslation();
  const { projectPermissions } = usePermissions();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'projects-table-page-size',
  });

  // Filter states - combined search for name or slug
  const [searchFilter, setSearchFilter] = useState<string>('');

  const debouncedSearchFilter = useDebounce(searchFilter, 300);

  // Memoize columns to prevent infinite re-renders
  const columns = useMemo(() => createColumns(t, projectPermissions.canWrite), [t, projectPermissions.canWrite]);

  // Build where clause for API filtering with OR logic
  const whereClause = (() => {
    if (!debouncedSearchFilter) {
      return undefined;
    }

    // Use OR logic to search in both name and slug fields
    return {
      or: [{ nameContainsFold: debouncedSearchFilter }],
    };
  })();

  const {
    data,
    isLoading,
    error: _error,
  } = useProjects({
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
      <ProjectsTable
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

export default function ProjectsPage() {
  const { t } = useTranslation();

  return (
    <ProjectsProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('projects.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('projects.description')}</p>
          </div>
          <ProjectsPrimaryButtons />
        </div>
      </Header>

      <Main fixed>
        <ProjectsContent />
      </Main>
      <ProjectsDialogs />
    </ProjectsProvider>
  );
}
