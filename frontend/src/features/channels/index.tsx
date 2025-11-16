import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useDebounce } from '@/hooks/use-debounce'
import { usePaginationSearch } from '@/hooks/use-pagination-search'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { createColumns } from './components/channels-columns'
import { ChannelsDialogs } from './components/channels-dialogs'
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons'
import { ChannelsTable } from './components/channels-table'
import { ChannelsTypeTabs } from './components/channels-type-tabs'
import ChannelsProvider from './context/channels-context'
import { useChannels, useChannelTypes } from './data/channels'

function ChannelsContent() {
  const { t } = useTranslation()
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
  })
  const [nameFilter, setNameFilter] = useState<string>('')
  const [typeFilter, setTypeFilter] = useState<string[]>([])
  const [statusFilter, setStatusFilter] = useState<string[]>([])
  const [selectedTypeTab, setSelectedTypeTab] = useState<string>('all')
  
  // Fetch channel types for tabs
  const { data: channelTypeCounts = [] } = useChannelTypes()
  
  // Debounce the name filter to avoid excessive API calls
  const debouncedNameFilter = useDebounce(nameFilter, 300)
  
  // Get types for the selected tab
  const tabFilteredTypes = useMemo(() => {
    if (selectedTypeTab === 'all') {
      return []
    }
    // Filter types that start with the selected prefix
    return channelTypeCounts
      .filter(({ type }) => type.startsWith(selectedTypeTab))
      .map(({ type }) => type)
  }, [selectedTypeTab, channelTypeCounts])
  
  // Build where clause with filters
  const whereClause = (() => {
    const where: Record<string, string | string[]> = {}
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter
    }
    // Combine tab filter with manual type filter
    const combinedTypeFilter = [...typeFilter]
    if (tabFilteredTypes.length > 0) {
      // If tab is selected, use tab types
      combinedTypeFilter.push(...tabFilteredTypes)
    }
    if (combinedTypeFilter.length > 0) {
      where.typeIn = Array.from(new Set(combinedTypeFilter)) // Remove duplicates
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
  
  const handleTabChange = (tab: string) => {
    setSelectedTypeTab(tab)
    // Clear manual type filter when switching tabs
    setTypeFilter([])
    resetCursor()
  }

  const handleStatusFilterChange = (filters: string[]) => {
    setStatusFilter(filters)
    resetCursor()
  }

  const columns = createColumns(t)

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
        <ChannelsTypeTabs 
          typeCounts={channelTypeCounts}
          selectedTab={selectedTypeTab}
          onTabChange={handleTabChange}
        />
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
          selectedTypeTab={selectedTypeTab}
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