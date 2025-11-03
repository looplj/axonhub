import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { TracesTable } from './components'
import { TracesProvider } from './context'
import { useTraces } from './data'
import { usePaginationSearch } from '@/hooks/use-pagination-search'

function TracesContent() {
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
  })

  // Build where clause with filters
  const whereClause = undefined

  const { data, isLoading } = useTraces({
    ...paginationArgs,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  })

  const traces = data?.edges?.map((edge) => edge.node) || []
  const pageInfo = data?.pageInfo

  const handleNextPage = () => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'after')
    }
  }

  const handlePreviousPage = () => {
    if (data?.pageInfo?.hasPreviousPage) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'before')
    }
  }

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize)
    resetCursor()
  }

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <TracesTable
        data={traces}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
      />
    </div>
  )
}

function TracesPrimaryButtons() {
  const { t } = useTranslation()
  const { refetch } = useTraces({
    first: 20,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  })

  return (
    <div className='flex items-center space-x-2'>
      <Button variant='outline' size='sm' onClick={() => refetch()}>
        <RefreshCw className='mr-2 h-4 w-4' />
        {t('traces.refresh')}
      </Button>
    </div>
  )
}

export default function TracesManagement() {
  const { t } = useTranslation()

  return (
    <TracesProvider>
      <Header fixed>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>{t('traces.title')}</h2>
            <p className='text-muted-foreground'>{t('traces.description')}</p>
          </div>
          <TracesPrimaryButtons />
        </div>
        <TracesContent />
      </Main>
    </TracesProvider>
  )
}
