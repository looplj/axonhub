import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { extractNumberID } from '@/lib/utils'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { LanguageSwitch } from '@/components/language-switch'
import { useUsageLogs } from './data'
import { 
  UsageLogsTable, 
  UsageDetailDialog,
} from './components'
import { UsageLogsProvider, useUsageLogsContext } from './context'
import { Button } from '@/components/ui/button'
import { RefreshCw } from 'lucide-react'

function UsageLogsContent() {
  const { t } = useTranslation()
  const [pageSize, setPageSize] = useState(20)
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  const [userFilter, setUserFilter] = useState<string>('')
  const [sourceFilter, setSourceFilter] = useState<string[]>([])
  const [channelFilter, setChannelFilter] = useState<string[]>([])
  
  // Build where clause with filters
  const whereClause = (() => {
    const where: any = {}
    
    if (userFilter) {
        where.userID = parseInt(userFilter)
    }
    
    if (sourceFilter.length > 0) {
      where.sourceIn = sourceFilter
    }
    
    if (channelFilter.length > 0) {
      where.channelIDIn = channelFilter;
    }
    
    return Object.keys(where).length > 0 ? where : undefined
  })()

  const { 
    data, 
    isLoading, 
    error 
  } = useUsageLogs({
    first: pageSize,
    after: cursor,
    orderBy: { field: 'CREATED_AT', direction: 'DESC' },
    where: whereClause,
  })

  const usageLogs = data?.edges?.map(edge => edge.node) || []
  const pageInfo = data?.pageInfo

  const handleNextPage = () => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursor(data.pageInfo.endCursor)
    }
  }

  const handlePreviousPage = () => {
    if (data?.pageInfo?.hasPreviousPage && data?.pageInfo?.startCursor) {
      setCursor(data.pageInfo.startCursor)
    }
  }

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize)
    setCursor(undefined) // Reset to first page
  }

  const handleUserFilterChange = (filter: string) => {
    setUserFilter(filter)
    setCursor(undefined) // Reset to first page when filtering
  }

  const handleSourceFilterChange = (filters: string[]) => {
    setSourceFilter(filters)
    setCursor(undefined) // Reset to first page when filtering
  }

  const handleChannelFilterChange = (filters: string[]) => {
    setChannelFilter(filters)
    setCursor(undefined) // Reset to first page when filtering
  }

  if (error) {
    return (
      <div className='flex h-64 items-center justify-center'>
        <p className='text-destructive'>
          {t('usageLogs.loadError')} {error.message}
        </p>
      </div>
    )
  }

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <UsageLogsTable
        data={usageLogs}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        userFilter={userFilter}
        sourceFilter={sourceFilter}
        channelFilter={channelFilter}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onUserFilterChange={handleUserFilterChange}
        onSourceFilterChange={handleSourceFilterChange}
        onChannelFilterChange={handleChannelFilterChange}
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

function UsageLogsPrimaryButtons() {
  const { t } = useTranslation()
  const { refetch } = useUsageLogs({
    first: 20,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  })

  return (
    <div className='flex items-center space-x-2'>
      <Button
        variant='outline'
        size='sm'
        onClick={() => refetch()}
      >
        <RefreshCw className='mr-2 h-4 w-4' />
        {t('usageLogs.refresh')}
      </Button>
    </div>
  )
}

export default function UsageLogsManagement() {
  const { t } = useTranslation()
  
  return (
    <UsageLogsProvider>
      <Header fixed>
        <div className='ml-auto flex items-center space-x-4'>
          <LanguageSwitch />
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>{t('usageLogs.title')}</h2>
            <p className='text-muted-foreground'>
              {t('usageLogs.description')}
            </p>
          </div>
          <UsageLogsPrimaryButtons />
        </div>
        <UsageLogsContent />
      </Main>
      <UsageLogsDialogs />
    </UsageLogsProvider>
  )
}