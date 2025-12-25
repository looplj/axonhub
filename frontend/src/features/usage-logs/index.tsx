import { useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { DateRange } from 'react-day-picker'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { useUsageLogs } from './data'
import { 
  UsageLogsTable, 
  UsageDetailDialog,
} from './components'
import { UsageLogsProvider, useUsageLogsContext } from './context'
import { usePaginationSearch } from '@/hooks/use-pagination-search'
import { buildDateRangeWhereClause } from '@/utils/date-range'

function UsageLogsContent() {
  const { t } = useTranslation()
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs, cursorHistory } = usePaginationSearch({
    defaultPageSize: 20,
  })
  const [sourceFilter, setSourceFilter] = useState<string[]>([])
  const [channelFilter, setChannelFilter] = useState<string[]>([])
  const [dateRange, setDateRange] = useState<DateRange | undefined>()
  
  // Build where clause with filters
  const whereClause = (() => {
    const where: { [key: string]: any } = {
      ...buildDateRangeWhereClause(dateRange),
    }
    
    if (sourceFilter.length > 0) {
      where.sourceIn = sourceFilter
    }
    
    if (channelFilter.length > 0) {
      where.channelIDIn = channelFilter
    }
    
    return Object.keys(where).length > 0 ? where : undefined
  })()

  const { 
    data, 
    isLoading, 
    error,
    refetch,
  } = useUsageLogs({
    ...paginationArgs,
    orderBy: { field: 'CREATED_AT', direction: 'DESC' },
    where: whereClause,
  })

  const usageLogs = data?.edges?.map(edge => edge.node) || []
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

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize)
  }

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

  const handleDateRangeChange = useCallback(
    (range: DateRange | undefined) => {
      setDateRange(range)
      resetCursor()
    },
    [resetCursor]
  )

  if (error) {
    return (
      <div className='flex h-64 items-center justify-center'>
        <p className='text-destructive'>
          {t('common.loadError')} {error.message}
        </p>
      </div>
    )
  }

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <UsageLogsTable
        data={usageLogs}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        sourceFilter={sourceFilter}
        channelFilter={channelFilter}
        dateRange={dateRange}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onSourceFilterChange={handleSourceFilterChange}
        onChannelFilterChange={handleChannelFilterChange}
        onDateRangeChange={handleDateRangeChange}
        onRefresh={refetch}
        showRefresh={isFirstPage}
      />
    </div>
  )
}

function UsageLogsDialogs() {
  const { 
    detailDialogOpen, 
    setDetailDialogOpen, 
    currentUsageLog: selectedUsageLog,
  } = useUsageLogsContext()

  return (
    <>
      {/* Usage detail dialog */}
      <UsageDetailDialog
        open={detailDialogOpen}
        onOpenChange={setDetailDialogOpen}
        usageLogId={selectedUsageLog?.id}
      />
    </>
  )
}

export default function UsageLogsManagement() {
  const { t } = useTranslation()
  
  return (
    <UsageLogsProvider>
      <Header fixed>
      </Header>

      <Main fixed>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>{t('usageLogs.title')}</h2>
            <p className='text-muted-foreground'>
              {t('usageLogs.description')}
            </p>
          </div>
        </div>
        <UsageLogsContent />
      </Main>
      <UsageLogsDialogs />
    </UsageLogsProvider>
  )
}