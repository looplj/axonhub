'use client'

import { format } from 'date-fns'
import { ColumnDef } from '@tanstack/react-table'
import { zhCN } from 'date-fns/locale'
import { Eye, MoreHorizontal } from 'lucide-react'
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
      <div className='font-mono text-xs'>{row.getValue('id')}</div>
    ),
    enableSorting: true,
    enableHiding: false,
  },
  {
    accessorKey: 'status',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='状态' />
    ),
    cell: ({ row }) => {
      const status = row.getValue('status') as string
      const getStatusColor = (status: string) => {
        switch (status) {
          case 'COMPLETED':
            return 'bg-green-100 text-green-800'
          case 'FAILED':
            return 'bg-red-100 text-red-800'
          case 'PENDING':
            return 'bg-yellow-100 text-yellow-800'
          case 'PROCESSING':
            return 'bg-blue-100 text-blue-800'
          default:
            return 'bg-gray-100 text-gray-800'
        }
      }
      return <Badge className={getStatusColor(status)}>{status}</Badge>
    },
    filterFn: (row, id, value) => {
      return value.includes(row.getValue(id))
    },
  },
  {
    accessorKey: 'userID',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='用户ID' />
    ),
    cell: ({ row }) => (
      <div className='font-mono text-xs'>
        {row.getValue('userID') || '未知'}
      </div>
    ),
  },
  {
    accessorKey: 'apiKeyID',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='API密钥ID' />
    ),
    cell: ({ row }) => (
      <div className='font-mono text-xs'>
        {row.getValue('apiKeyID') || '未知'}
      </div>
    ),
  },
  {
    accessorKey: 'requestBody',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='请求体' />
    ),
    cell: ({ row }) => {
      const requestBody = JSON.stringify(row.getValue('requestBody'))
      const preview = requestBody ? requestBody.substring(0, 50) + '...' : '无'
      return (
        <div className='max-w-[200px] truncate text-xs' title={requestBody}>
          {preview}
        </div>
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
      const { setDetailDialogOpen, setCurrentRequest: setSelectedRequest } =
        useRequestsContext()

      const handleViewDetails = () => {
        setSelectedRequest(request)
        setDetailDialogOpen(true)
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
