import { useState, useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { SortingState } from '@tanstack/react-table'
import { IconPlus } from '@tabler/icons-react'
import { useDebounce } from '@/hooks/use-debounce'
import { usePaginationSearch } from '@/hooks/use-pagination-search'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { createColumns } from './components/models-columns'
import { ModelsDialogs } from './components/models-dialogs'
import { ModelsTable } from './components/models-table'
import ModelsProvider, { useModels } from './context/models-context'
import { useQueryModels } from './data/models'

function ModelsContent() {
  const { t } = useTranslation()
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
  })
  const [nameFilter, setNameFilter] = useState<string>('')
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'createdAt', desc: true },
  ])

  const debouncedNameFilter = useDebounce(nameFilter, 300)

  const whereClause = (() => {
    const where: Record<string, string | string[]> = {}
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter
    }
    return Object.keys(where).length > 0 ? where : undefined
  })()

  const currentOrderBy = (() => {
    if (sorting.length === 0) {
      return { field: 'CREATED_AT', direction: 'DESC' } as const
    }
    const [primary] = sorting
    switch (primary.id) {
      case 'name':
        return { field: 'NAME', direction: primary.desc ? 'DESC' : 'ASC' } as const
      case 'modelId':
        return { field: 'MODEL_ID', direction: primary.desc ? 'DESC' : 'ASC' } as const
      case 'createdAt':
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const
      default:
        return { field: 'CREATED_AT', direction: 'DESC' } as const
    }
  })()

  const { data } = useQueryModels({
    ...paginationArgs,
    where: whereClause,
    orderBy: currentOrderBy,
  })

  const handleNextPage = useCallback(() => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'after')
    }
  }, [data?.pageInfo, setCursors])

  const handlePreviousPage = useCallback(() => {
    if (data?.pageInfo?.hasPreviousPage) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'before')
    }
  }, [data?.pageInfo, setCursors])

  const handlePageSizeChange = useCallback(
    (newPageSize: number) => {
      setPageSize(newPageSize)
    },
    [setPageSize]
  )

  const handleNameFilterChange = useCallback(
    (filter: string) => {
      setNameFilter(filter)
      resetCursor()
    },
    [resetCursor, setNameFilter]
  )

  const columns = useMemo(() => createColumns(t), [t])

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
  )
}

function CreateButton() {
  const { t } = useTranslation()
  const { setOpen } = useModels()
  
  return (
    <Button onClick={() => setOpen('create')}>
      <IconPlus className='mr-2 h-4 w-4' />
      {t('models.actions.create')}
    </Button>
  )
}

export default function ModelsManagement() {
  const { t } = useTranslation()

  return (
    <ModelsProvider>
      <Header fixed />

      <Main fixed>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>{t('models.title')}</h2>
            <p className='text-muted-foreground'>{t('models.description')}</p>
          </div>
          <CreateButton />
        </div>
        <ModelsContent />
      </Main>
      <ModelsDialogs />
    </ModelsProvider>
  )
}
