import { useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { usePaginationSearch } from '@/hooks/use-pagination-search'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { RequestsTable } from './components'
import { RequestsProvider } from './context'
import { useRequests } from './data'

function RequestsContent() {
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs, cursorHistory } = usePaginationSearch({
    defaultPageSize: 20,
  })
  const [statusFilter, setStatusFilter] = useState<string[]>([])
  const [sourceFilter, setSourceFilter] = useState<string[]>([])
  const [channelFilter, setChannelFilter] = useState<string[]>([])

  // Build where clause with filters
  const whereClause = (() => {
    const where: any = {}
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter
    }
    if (sourceFilter.length > 0) {
      where.sourceIn = sourceFilter
    }
    if (channelFilter.length > 0) {
      // Add channel filter - assuming the backend supports filtering by channel IDs
      // This might need to be adjusted based on the actual GraphQL schema
      where.channelIDIn = channelFilter
    }
    return Object.keys(where).length > 0 ? where : undefined
  })()

  const { data, isLoading, refetch } = useRequests({
    ...paginationArgs,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  })

  const requests = data?.edges?.map((edge) => edge.node) || []
  const pageInfo = data?.pageInfo

  const isFirstPage = !paginationArgs.after && cursorHistory.length === 0

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

  const handleStatusFilterChange = useCallback(
    (filters: string[]) => {
      setStatusFilter(filters)
      resetCursor()
    },
    [resetCursor]
  )

  const handleSourceFilterChange = useCallback(
    (filters: string[]) => {
      setSourceFilter(filters)
      resetCursor()
    },
    [resetCursor]
  )

  const handleChannelFilterChange = useCallback(
    (filters: string[]) => {
      setChannelFilter(filters)
      resetCursor()
    },
    [resetCursor]
  )

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <RequestsTable
        data={requests}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        statusFilter={statusFilter}
        sourceFilter={sourceFilter}
        channelFilter={channelFilter}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={setPageSize}
        onStatusFilterChange={handleStatusFilterChange}
        onSourceFilterChange={handleSourceFilterChange}
        onChannelFilterChange={handleChannelFilterChange}
        onRefresh={refetch}
        showRefresh={isFirstPage}
      />
    </div>
  )
}

export default function RequestsManagement() {
  const { t } = useTranslation()

  return (
    <RequestsProvider>
      {/* <Header fixed></Header> */}

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>{t('requests.title')}</h2>
            <p className='text-muted-foreground'>{t('requests.description')}</p>
          </div>
        </div>
        <RequestsContent />
      </Main>
    </RequestsProvider>
  )
}
