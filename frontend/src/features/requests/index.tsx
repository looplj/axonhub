import { useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { LanguageSwitch } from '@/components/language-switch'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { RequestsTable } from './components'
import { RequestsProvider } from './context'
import { useRequests } from './data'

function RequestsContent() {
  const [pageSize, setPageSize] = useState(20)
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  const [userFilter, setUserFilter] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string[]>([])
  const [sourceFilter, setSourceFilter] = useState<string[]>([])
  const [channelFilter, setChannelFilter] = useState<string[]>([])

  // Build where clause with filters
  const whereClause = (() => {
    const where: any = {}
    if (userFilter) {
      where.userID = userFilter
    }
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
    first: pageSize,
    after: cursor,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  })

  const requests = data?.edges?.map((edge) => edge.node) || []
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

  const handleStatusFilterChange = (filters: string[]) => {
    setStatusFilter(filters)
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

  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <RequestsTable
        data={requests}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        userFilter={userFilter}
        statusFilter={statusFilter}
        sourceFilter={sourceFilter}
        channelFilter={channelFilter}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onUserFilterChange={handleUserFilterChange}
        onStatusFilterChange={handleStatusFilterChange}
        onSourceFilterChange={handleSourceFilterChange}
        onChannelFilterChange={handleChannelFilterChange}
      />
    </div>
  )
}

function RequestsPrimaryButtons() {
  const { t } = useTranslation()
  const { refetch } = useRequests({
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
        {t('requests.refresh')}
      </Button>
    </div>
  )
}

export default function RequestsManagement() {
  const { t } = useTranslation()

  return (
    <RequestsProvider>
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
            <h2 className='text-2xl font-bold tracking-tight'>{t('requests.title')}</h2>
            <p className='text-muted-foreground'>{t('requests.description')}</p>
          </div>
          <RequestsPrimaryButtons />
        </div>
        <RequestsContent />
      </Main>
    </RequestsProvider>
  )
}
