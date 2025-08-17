'use client'

import { format } from 'date-fns'
import { ColumnDef } from '@tanstack/react-table'
import { zhCN, enUS } from 'date-fns/locale'
import { Eye, MoreHorizontal, FileText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
import { useRequestPermissions } from '../../../hooks/useRequestPermissions'
import { Request, RequestStatus } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { getStatusColor } from './help'

// Removed unused statusColors - using getStatusColor helper instead

export function useRequestsColumns(): ColumnDef<Request>[] {
  const { t, i18n } = useTranslation()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const permissions = useRequestPermissions()

  return [
  {
    accessorKey: 'id',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('requests.columns.id')} />
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
      <DataTableColumnHeader column={column} title={t('requests.columns.modelId')} />
    ),
    enableSorting: false,
    cell: ({ row }) => {
      const request = row.original
      return <div className='font-mono text-xs'>{request.modelID || t('requests.columns.unknown')}</div>
    },
  },
  {
    accessorKey: 'status',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('requests.columns.status')} />
    ),
    enableSorting: false,
    cell: ({ row }) => {
      const status = row.getValue('status') as string
      return <Badge className={getStatusColor(status)}>{t(`requests.status.${status}`)}</Badge>
    },
    filterFn: (row, id, value) => {
      return value.includes(row.getValue(id))
    },
  },
  {
    id: 'user',
    accessorFn: (row) => row.user?.id || '',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('requests.columns.user')} />
    ),
    enableSorting: false,
    cell: ({ row }) => {
      if (!permissions.canViewUsers) {
        return (
          <div className='font-mono text-xs text-muted-foreground'>
            -
          </div>
        )
      }
      const user = row.original.user
      const displayName = user ? `${user.firstName} ${user.lastName}` : t('requests.columns.unknown')
      return (
        <div className='font-mono text-xs'>
          {displayName}
        </div>
      )
    },
    filterFn: (row, _id, value) => {
      const user = row.original.user
      if (!user) return false
      return value.includes(user.id)
    },
  },
  {
    accessorKey: 'apiKey',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('requests.columns.apiKey')} />
    ),
    enableSorting: false,
    cell: ({ row }) => {
      if (!permissions.canViewApiKeys) {
        return (
          <div className='font-mono text-xs text-muted-foreground'>
            -
          </div>
        )
      }
      return (
        <div className='font-mono text-xs'>
          {row.original.apiKey?.name || t('requests.columns.unknown')}
        </div>
      )
    },
  },
  {
    id: 'requestBody',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('requests.columns.requestBody')} />
    ),
    cell: ({ row }) => {
      const { setJsonViewerOpen, setJsonViewerData } = useRequestsContext()

      const handleViewRequestBody = () => {
        setJsonViewerData({
          title: t('requests.dialogs.jsonViewer.requestBody'),
          data: row.original.requestBody,
        })
        setJsonViewerOpen(true)
      }

      return (
        <Button variant='outline' size='sm' onClick={handleViewRequestBody}>
          <FileText className='mr-2 h-4 w-4' />
          {t('requests.actions.viewRequest')}
        </Button>
      )
    },
  },
  {
    id: 'responseBody',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('requests.columns.responseBody')} />
    ),
    cell: ({ row }) => {
      const { setJsonViewerOpen, setJsonViewerData } = useRequestsContext()

      const handleViewResponseBody = () => {
        setJsonViewerData({
          title: t('requests.dialogs.jsonViewer.responseBody'),
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
          {t('requests.actions.viewResponse')}
        </Button>
      )
    },
  },
  {
    accessorKey: 'createdAt',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('requests.columns.createdAt')} />
    ),
    cell: ({ row }) => {
      const date = new Date(row.getValue('createdAt'))
      return (
        <div className='text-xs'>
          {format(date, 'yyyy-MM-dd HH:mm:ss', { locale })}
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
              <span className='sr-only'>{t('requests.actions.openMenu')}</span>
              <MoreHorizontal className='h-4 w-4' />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align='end'>
            <DropdownMenuItem onClick={handleViewDetails}>
              <Eye className='mr-2 h-4 w-4' />
              {t('requests.actions.viewDetails')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )
    },
  },
  ]
}
