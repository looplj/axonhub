import { useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { DateRange } from 'react-day-picker'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ThreadsTable } from './components/threads-table'
import { useThreads } from './data/threads'
import type { Thread } from './data/schema'
import { usePaginationSearch } from '@/hooks/use-pagination-search'
import { useDebounce } from '@/hooks/use-debounce'
import useInterval from '@/hooks/useInterval'
import { buildDateRangeWhereClause } from '@/utils/date-range'

function ThreadsContent() {
  const {
    pageSize,
    setCursors,
    setPageSize,
    resetCursor,
    paginationArgs,
    cursorHistory,
  } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'threads-table-page-size',
  })
  const [dateRange, setDateRange] = useState<DateRange | undefined>()
  const [threadIdFilter, setThreadIdFilter] = useState<string>('')
  const [autoRefresh, setAutoRefresh] = useState(false)
  const debouncedThreadIdFilter = useDebounce(threadIdFilter, 300)

  const whereClause = (() => {
    const where: { [key: string]: any } = {
      ...buildDateRangeWhereClause(dateRange),
    }
    
    if (debouncedThreadIdFilter.trim()) {
      where.threadIDContains = debouncedThreadIdFilter.trim()
    }
    
    return Object.keys(where).length > 0 ? where : undefined
  })()

  const { data, isLoading, refetch } = useThreads({
    ...paginationArgs,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  })

  const threads: Thread[] = data ? data.edges.map(({ node }) => node) : []
  const pageInfo = data?.pageInfo
  const isFirstPage = !paginationArgs.after && cursorHistory.length === 0

  useInterval(
    () => {
      refetch()
    },
    autoRefresh && isFirstPage ? 30000 : null
  )

  const handleNextPage = () => {
    if (pageInfo?.hasNextPage && pageInfo.endCursor) {
      setCursors(pageInfo.startCursor ?? undefined, pageInfo.endCursor ?? undefined, 'after')
    }
  }

  const handlePreviousPage = () => {
    if (pageInfo?.hasPreviousPage) {
      setCursors(pageInfo.startCursor ?? undefined, pageInfo.endCursor ?? undefined, 'before')
    }
  }

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize)
    resetCursor()
  }

  const handleDateRangeChange = useCallback(
    (range: DateRange | undefined) => {
      setDateRange(range)
      resetCursor()
    },
    [resetCursor]
  )

  const handleThreadIdFilterChange = useCallback(
    (threadId: string) => {
      setThreadIdFilter(threadId)
      resetCursor()
    },
    [resetCursor]
  )

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <ThreadsTable
        data={threads}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        dateRange={dateRange}
        threadIdFilter={threadIdFilter}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onDateRangeChange={handleDateRangeChange}
        onThreadIdFilterChange={handleThreadIdFilterChange}
        onRefresh={refetch}
        showRefresh={isFirstPage}
        autoRefresh={autoRefresh}
        onAutoRefreshChange={setAutoRefresh}
      />
    </div>
  )
}

export default function ThreadsManagement() {
  const { t } = useTranslation()

  return (
    <>
      <Header fixed></Header>

      <Main fixed>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>{t('threads.title')}</h2>
            <p className='text-muted-foreground'>{t('threads.description')}</p>
          </div>
        </div>
        <ThreadsContent />
      </Main>
    </>
  )
}
