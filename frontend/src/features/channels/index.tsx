import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useDebounce } from '@/hooks/use-debounce'
import { usePaginationSearch } from '@/hooks/use-pagination-search'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { createColumns } from './components/channels-columns'
import { ChannelsDialogs } from './components/channels-dialogs'
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons'
import { ChannelsTable } from './components/channels-table'
import ChannelsProvider from './context/channels-context'
import { useChannels } from './data/channels'

function ChannelsContent() {
  const { t } = useTranslation()
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
  })
  const [nameFilter, setNameFilter] = useState<string>('')
  const [typeFilter, setTypeFilter] = useState<string[]>([])
  const [statusFilter, setStatusFilter] = useState<string[]>([])
  
  // Debounce the name filter to avoid excessive API calls
  const debouncedNameFilter = useDebounce(nameFilter, 300)
  
  // Build where clause with filters
  const whereClause = (() => {
    const where: Record<string, string | string[]> = {}
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter
    }
    if (typeFilter.length > 0) {
      where.typeIn = typeFilter
    }
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter
    } else {
      // By default, exclude archived channels when no status filter is applied
      where.statusIn = ['enabled', 'disabled']
    }
    return Object.keys(where).length > 0 ? where : undefined
  })()
  
  const { data, isLoading: _isLoading, error: _error } = useChannels({
    ...paginationArgs,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  })

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
  }

  const handleNameFilterChange = (filter: string) => {
    setNameFilter(filter)
    resetCursor()
  }

  const handleTypeFilterChange = (filters: string[]) => {
    setTypeFilter(filters)
    resetCursor()
  }

  const handleStatusFilterChange = (filters: string[]) => {
    setStatusFilter(filters)
    resetCursor()
  }

  const columns = createColumns(t)

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
        <ChannelsTable 
          // loading={isLoading}
          data={data?.edges?.map(edge => edge.node) || []} 
          columns={columns}
          pageInfo={data?.pageInfo}
          pageSize={pageSize}
          totalCount={data?.totalCount}
          nameFilter={nameFilter}
          typeFilter={typeFilter}
          statusFilter={statusFilter}
          onNextPage={handleNextPage}
          onPreviousPage={handlePreviousPage}
          onPageSizeChange={handlePageSizeChange}
          onNameFilterChange={handleNameFilterChange}
          onTypeFilterChange={handleTypeFilterChange}
          onStatusFilterChange={handleStatusFilterChange}
        />
    </div>
  )
}

export default function ChannelsManagement() {
  const { t } = useTranslation()
  
  return (
    <ChannelsProvider>
      <Header fixed>
        {/* <Search /> */}
      </Header>

      <Main fixed>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>{t('channels.title')}</h2>
            <p className='text-muted-foreground'>
              {t('channels.description')}
            </p>
          </div>
          <ChannelsPrimaryButtons />
        </div>
        <ChannelsContent />
      </Main>
      <ChannelsDialogs />
    </ChannelsProvider>
  )
}