'use client'

import { format } from 'date-fns'
import { ColumnDef } from '@tanstack/react-table'
import { zhCN, enUS } from 'date-fns/locale'
import { Eye, MoreHorizontal, FileText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
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
import { useRequestPermissions } from '../../../gql/useRequestPermissions'
import { Request } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { getStatusColor } from './help'

// Removed unused statusColors - using getStatusColor helper instead

export function useRequestsColumns(): ColumnDef<Request>[] {
  const { t, i18n } = useTranslation()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const permissions = useRequestPermissions()

  // Define all columns
  const columns: ColumnDef<Request>[] = [
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
      accessorKey: 'source',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('requests.columns.source')} />
      ),
      enableSorting: false,
      cell: ({ row }) => {
        const source = row.getValue('source') as string
        const sourceColors: Record<string, string> = {
          api: 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:border-blue-800',
          playground: 'bg-purple-100 text-purple-800 border-purple-200 dark:bg-purple-900/20 dark:text-purple-300 dark:border-purple-800',
          test: 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-300 dark:border-green-800',
        }
        return (
          <Badge className={sourceColors[source] || 'bg-gray-100 text-gray-800 border-gray-200 dark:bg-gray-900/20 dark:text-gray-300 dark:border-gray-800'}>
            {t(`requests.source.${source}`)}
          </Badge>
        )
      },
      filterFn: (row, id, value) => {
        return value.includes(row.getValue(id))
      },
    },
    // Channel column - only show if user has permission to view channels
    ...(permissions.canViewChannels ? [{
      id: 'channel',
      accessorFn: (row) => row.executions?.edges?.[0]?.node?.channel?.id || '',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('requests.columns.channel')} />
      ),
      enableSorting: false,
      cell: ({ row }) => {
        // 获取第一个 execution 的 channel 信息
        const execution = row.original.executions?.edges?.[0]?.node
        const channel = execution?.channel
        
        if (!channel) {
          return (
            <div className='font-mono text-xs text-muted-foreground'>
              {t('requests.columns.unknown')}
            </div>
          )
        }
        
        return (
          <div className='font-mono text-xs'>
            {channel.name}
          </div>
        )
      },
      filterFn: (row, _id, value) => {
        // For client-side filtering, check if any of the selected channels match
        if (value.length === 0) return true // No filter applied
        
        const execution = row.original.executions?.edges?.[0]?.node
        const channel = execution?.channel
        if (!channel) return false
        
        return value.includes(channel.id)
      },
    }] as ColumnDef<Request>[] : []),
    // User column - only show if user has permission to view users
    ...(permissions.canViewUsers ? [{
      id: 'user',
      accessorFn: (row) => row.user?.id || '',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('requests.columns.user')} />
      ),
      enableSorting: false,
      cell: ({ row }) => {
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
    }] as ColumnDef<Request>[] : []),
    // API Key column - only show if user has permission to view API keys
    ...(permissions.canViewApiKeys ? [{
      accessorKey: 'apiKey',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('requests.columns.apiKey')} />
      ),
      enableSorting: false,
      cell: ({ row }) => {
        return (
          <div className='font-mono text-xs'>
            {row.original.apiKey?.name || t('requests.columns.unknown')}
          </div>
        )
      },
    }] as ColumnDef<Request>[] : []),
    {
      id: 'details',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('requests.columns.details')} />
      ),
      cell: ({ row }) => {
        const navigate = useNavigate()

        const handleViewDetails = () => {
          navigate({ to: '/requests/$requestId', params: { requestId: row.original.id } })
        }

        return (
          <Button variant='outline' size='sm' onClick={handleViewDetails}>
            <FileText className='mr-2 h-4 w-4' />
            {t('requests.actions.viewDetails')}
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

  return columns
}
