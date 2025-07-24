import { useState } from 'react'
import { useRequests } from './data'
import { 
  RequestsTable, 
  RequestDetailDialog, 
  JsonViewerDialog,
  ExecutionDetailDialog,
  ExecutionsDrawer
} from './components'
import { requestsColumns } from './components/requests-columns'
import { RequestsProvider, useRequestsContext } from './context'
import { Button } from '@/components/ui/button'
import { RefreshCw } from 'lucide-react'

function RequestsContent() {
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  
  const { data, isLoading, refetch } = useRequests({
    first: pageSize,
    after: page > 1 ? btoa(`cursor:${(page - 1) * pageSize}`) : undefined,
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

  const handleExecutionSelect = (execution: any) => {
    setCurrentExecution(execution)
    setExecutionsDrawerOpen(false)
    setExecutionDetailOpen(true)
  }

  return (
    <>
      <div className='flex h-full flex-col'>
        <div className='flex items-center justify-between border-b px-6 py-4'>
          <div>
            <h1 className='text-2xl font-semibold'>请求日志</h1>
            <p className='text-muted-foreground text-sm'>
              查看和管理 LLM 调用请求日志
            </p>
          </div>
          <div className='flex items-center space-x-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => refetch()}
              disabled={isLoading}
            >
              <RefreshCw className='mr-2 h-4 w-4' />
              刷新
            </Button>
          </div>
        </div>
        <div className='flex-1 p-6'>
          <RequestsTable
            columns={requestsColumns}
            data={requests}
            isLoading={isLoading}
          />
        </div>
      </div>

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

export default function RequestsManagement() {
  return (
    <RequestsProvider>
      <RequestsContent />
    </RequestsProvider>
  )
}