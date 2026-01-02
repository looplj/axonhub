import { useState, useMemo, useCallback, useEffect } from 'react';
import { SortingState } from '@tanstack/react-table';
import { IconPlus, IconSettings, IconAlertCircle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Button } from '@/components/ui/button';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { PermissionGuard } from '@/components/permission-guard';
import { useOnboardingInfo } from '@/features/system/data/system';
import { createColumns } from './components/models-columns';
import { ModelsDialogs } from './components/models-dialogs';
import { ModelsOnboardingFlow } from './components/models-onboarding-flow';
import { ModelsTable } from './components/models-table';
import ModelsProvider, { useModels } from './context/models-context';
import { useQueryModels } from './data/models';

function ModelsContent() {
  const { t } = useTranslation();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'models-table-page-size',
  });
  const [nameFilter, setNameFilter] = useState<string>('');
  const [sorting, setSorting] = useState<SortingState>(() => {
    const stored = localStorage.getItem('models-table-sorting');
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
    localStorage.setItem('models-table-sorting', JSON.stringify(sorting));
  }, [sorting]);

  const debouncedNameFilter = useDebounce(nameFilter, 300);

  const whereClause = (() => {
    const where: Record<string, string | string[]> = {};
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter;
    }
    return Object.keys(where).length > 0 ? where : undefined;
  })();

  const currentOrderBy = (() => {
    if (sorting.length === 0) {
      return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
    const [primary] = sorting;
    switch (primary.id) {
      case 'name':
        return { field: 'NAME', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'modelId':
        return { field: 'MODEL_ID', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'createdAt':
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      default:
        return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
  })();

  const { data } = useQueryModels({
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
    [resetCursor, setNameFilter]
  );

  const columns = useMemo(() => createColumns(t), [t]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <ModelsTable
        data={data?.edges?.map((edge) => edge.node) || []}
        columns={columns}
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
    </div>
  );
}

function CreateButton() {
  const { t } = useTranslation();
  const { setOpen } = useModels();

  return (
    <Button onClick={() => setOpen('create')}>
      <IconPlus className='mr-2 h-4 w-4' />
      {t('models.actions.create')}
    </Button>
  );
}

function BulkAddButton() {
  const { t } = useTranslation();
  const { setOpen } = useModels();

  return (
    <Button variant='outline' onClick={() => setOpen('batchCreate')}>
      <IconPlus className='mr-2 h-4 w-4' />
      {t('models.actions.bulkAdd')}
    </Button>
  );
}

function SettingsButton() {
  const { t } = useTranslation();
  const { setOpen } = useModels();

  return (
    <Button variant='outline' onClick={() => setOpen('settings')} data-settings-button>
      <IconSettings className='mr-2 h-4 w-4' />
      {t('models.actions.settings')}
    </Button>
  );
}

function DetectUnassociatedButton() {
  const { t } = useTranslation();
  const { setOpen } = useModels();

  return (
    <Button variant='outline' onClick={() => setOpen('unassociated')}>
      <IconAlertCircle className='mr-2 h-4 w-4' />
      {t('models.actions.detectUnassociated')}
    </Button>
  );
}

function ActionButtons() {
  return (
    <div className='flex gap-2'>
      <PermissionGuard requiredScope='write_channels'>
        <DetectUnassociatedButton />
      </PermissionGuard>
      <PermissionGuard requiredScope='write_channels'>
        <SettingsButton />
      </PermissionGuard>
      <PermissionGuard requiredScope='write_channels'>
        <BulkAddButton />
      </PermissionGuard>
      <PermissionGuard requiredScope='write_channels'>
        <CreateButton />
      </PermissionGuard>
    </div>
  );
}

export default function ModelsManagement() {
  const { t } = useTranslation();
  const { data: onboardingInfo } = useOnboardingInfo();
  const [showOnboarding, setShowOnboarding] = useState(false);

  const shouldShowOnboarding = onboardingInfo && !onboardingInfo.systemModelSetting?.onboarded;

  useEffect(() => {
    if (shouldShowOnboarding) {
      setShowOnboarding(true);
    }
  }, [shouldShowOnboarding]);

  const handleOnboardingComplete = useCallback(() => {
    setShowOnboarding(false);
  }, []);

  return (
    <ModelsProvider>
      <Header fixed />

      <Main fixed>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>{t('models.title')}</h2>
            <p className='text-muted-foreground'>{t('models.description')}</p>
          </div>
          <ActionButtons />
        </div>
        <ModelsContent />
      </Main>
      <ModelsDialogs />
      {showOnboarding && <ModelsOnboardingFlow onComplete={handleOnboardingComplete} />}
    </ModelsProvider>
  );
}
