import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { extractNumberID } from '@/lib/utils'
import { useDebounce } from '@/hooks/use-debounce'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { LanguageSwitch } from '@/components/language-switch'
import { useRequests } from './data'
import { 
  RequestsTable, 
  RequestDetailDialog, 
  JsonViewerDialog,
  ExecutionDetailDialog,
  ExecutionsDrawer
} from './components'
import { useRequestsColumns } from './components/requests-columns'
import { RequestsProvider, useRequestsContext } from './context'
import { Button } from '@/components/ui/button'
import { RefreshCw } from 'lucide-react'

function RequestsContent() {
  const { t } = useTranslation()
  const requestsColumns = useRequestsColumns()
  const [pageSize, setPageSize] = useState(20)
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  const [userFilter, setUserFilter] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string[]>([])
  const [sourceFilter, setSourceFilter] = useState<string[]>([])
  
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

  const { 
    detailDialogOpen, 
    setDetailDialogOpen, 
    currentRequest: selectedRequest,
    jsonViewerOpen,
    setJsonViewerOpen,
    jsonViewerData,
    executionDetailOpen,
    setExecutionDetailOpen,
    currentExecution,
    setCurrentExecution,
    executionsDrawerOpen,
    setExecutionsDrawerOpen,
    currentRequest
  } = useRequestsContext()

  const requests = data?.edges?.map(edge => edge.node) || []
  const pageInfo = data?.pageInfo

  const handleExecutionSelect = (execution: any) => {
    setCurrentExecution(execution)
    setExecutionsDrawerOpen(false)
    setExecutionDetailOpen(true)
  }

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
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onUserFilterChange={handleUserFilterChange}
        onStatusFilterChange={handleStatusFilterChange}
        onSourceFilterChange={handleSourceFilterChange}
      />
    </div>
  )
}

function RequestsDialogs() {
  const { 
    detailDialogOpen, 
    setDetailDialogOpen, 
    currentRequest: selectedRequest,
    jsonViewerOpen,
    setJsonViewerOpen,
    jsonViewerData,
    executionDetailOpen,
    setExecutionDetailOpen,
    currentExecution,
    setCurrentExecution,
    executionsDrawerOpen,
    setExecutionsDrawerOpen,
    currentRequest
  } = useRequestsContext()

  const handleExecutionSelect = (execution: any) => {
    setCurrentExecution(execution)
    setExecutionsDrawerOpen(false)
    setExecutionDetailOpen(true)
  }

  return (
    <>
      {/* JSON 查看器弹窗 */}
      <JsonViewerDialog
        open={jsonViewerOpen}
        onOpenChange={setJsonViewerOpen}
        title={jsonViewerData?.title || ''}
        jsonData={jsonViewerData?.data}
      />

      {/* 执行详情弹窗 */}
      <ExecutionDetailDialog
        open={executionDetailOpen}
        onOpenChange={setExecutionDetailOpen}
        execution={currentExecution}
      />

      {/* 执行列表抽屉 */}
      <ExecutionsDrawer
        open={executionsDrawerOpen}
        onOpenChange={setExecutionsDrawerOpen}
        executions={currentRequest?.executions?.edges?.map(edge => edge.node) || []}
        onExecutionSelect={handleExecutionSelect}
      />

      {/* 保留原有的详情弹窗（如果需要的话） */}
      <RequestDetailDialog
        open={detailDialogOpen}
        onOpenChange={setDetailDialogOpen}
        requestId={selectedRequest?.id}
      />
    </>
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
      <Button
        variant='outline'
        size='sm'
        onClick={() => refetch()}
      >
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
        {/* <Search /> */}
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
            <p className='text-muted-foreground'>
              {t('requests.description')}
            </p>
          </div>
          <RequestsPrimaryButtons />
        </div>
        <RequestsContent />
      </Main>
      <RequestsDialogs />
    </RequestsProvider>
  )
}