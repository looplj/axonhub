'use client'

import { format } from 'date-fns'
import { ColumnDef } from '@tanstack/react-table'
import { zhCN } from 'date-fns/locale'
import { Eye, MoreHorizontal, FileText } from 'lucide-react'
import { extractNumberID } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useRequestsContext } from '../context'
import { Request, RequestStatus } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { getStatusColor } from './help'

const statusColors: Record<RequestStatus, string> = {
  pending:
    'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300',
  processing: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
  completed:
    'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300',
}

const statusLabels: Record<RequestStatus, string> = {
  pending: '等待中',
  processing: '处理中',
  completed: '已完成',
  failed: '失败',
}

export const requestsColumns: ColumnDef<Request>[] = [
  {
    accessorKey: 'id',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='ID' />
    ),
    cell: ({ row }) => (
      <div className='font-mono text-xs'>
        #{extractNumberID(row.getValue('id'))}
      </div>
    ),
    enableSorting: true,
    enableHiding: false,
  },
  {
    id: 'modelId',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='模型ID' />
    ),
    enableSorting: false,
    cell: ({ row }) => {
      const request = row.original
      // 从 executions 中获取第一个 execution 的 modelID
      const modelId = request.executions?.edges?.[0]?.node?.modelID
      return <div className='font-mono text-xs'>{modelId || '未知'}</div>
    },
  },
  {
    accessorKey: 'status',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='状态' />
    ),
    enableSorting: false,
    cell: ({ row }) => {
      const status = row.getValue('status') as string
      return <Badge className={getStatusColor(status)}>{status}</Badge>
    },
    filterFn: (row, id, value) => {
      return value.includes(row.getValue(id))
    },
  },
  {
    accessorKey: 'user.name',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='用户' />
    ),
    enableSorting: false,
    cell: ({ row }) => (
      <div className='font-mono text-xs'>
        {row.original.user?.name || '未知'}
      </div>
    ),
  },
  {
    accessorKey: 'apiKey',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='API密钥' />
    ),
    enableSorting: false,
    cell: ({ row }) => (
      <div className='font-mono text-xs'>
        {row.original.apiKey?.name || '未知'}
      </div>
    ),
  },
  {
    id: 'requestBody',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='请求体' />
    ),
    cell: ({ row }) => {
      const { setJsonViewerOpen, setJsonViewerData } = useRequestsContext()

      const handleViewRequestBody = () => {
        setJsonViewerData({
          title: '请求体',
          data: row.original.requestBody,
        })
        setJsonViewerOpen(true)
      }

      return (
        <Button variant='outline' size='sm' onClick={handleViewRequestBody}>
          <FileText className='mr-2 h-4 w-4' />
          查看请求
        </Button>
      )
    },
  },
  {
    id: 'responseBody',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='响应体' />
    ),
    cell: ({ row }) => {
      const { setJsonViewerOpen, setJsonViewerData } = useRequestsContext()

      const handleViewResponseBody = () => {
        setJsonViewerData({
          title: '响应体',
          data: row.original.responseBody,
        })
        setJsonViewerOpen(true)
      }

      return (
        <Button
          variant='outline'
          size='sm'
          onClick={handleViewResponseBody}
          disabled={!row.original.responseBody}
        >
          <FileText className='mr-2 h-4 w-4' />
          查看响应
        </Button>
      )
    },
  },
  {
    accessorKey: 'createdAt',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='创建时间' />
    ),
    cell: ({ row }) => {
      const date = new Date(row.getValue('createdAt'))
      return (
        <div className='text-xs'>
          {format(date, 'yyyy-MM-dd HH:mm:ss', { locale: zhCN })}
        </div>
      )
    },
  },
  {
    id: 'actions',
    cell: ({ row }) => {
      const request = row.original
      const {
        setCurrentRequest,
        setCurrentExecution,
        setExecutionDetailOpen,
        setExecutionsDrawerOpen,
      } = useRequestsContext()

      const handleViewDetails = () => {
        setCurrentRequest(request)

        // 获取 executions
        const executions =
          request.executions?.edges?.map((edge) => edge.node) || []

        if (executions.length === 1) {
          // 如果只有一个 execution，直接打开详情弹窗
          setCurrentExecution(executions[0])
          setExecutionDetailOpen(true)
        } else if (executions.length > 1) {
          // 如果有多个 executions，打开抽屉
          setExecutionsDrawerOpen(true)
        } else {
          // 如果没有 executions，可以显示一个提示或者什么都不做
          console.log('No executions found for this request')
        }
      }

      return (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant='ghost' className='h-8 w-8 p-0'>
              <span className='sr-only'>打开菜单</span>
              <MoreHorizontal className='h-4 w-4' />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align='end'>
            <DropdownMenuItem onClick={handleViewDetails}>
              <Eye className='mr-2 h-4 w-4' />
              查看详情
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )
    },
  },
]
