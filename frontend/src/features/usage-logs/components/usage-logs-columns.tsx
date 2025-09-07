'use client'

import { format } from 'date-fns'
import { ColumnDef } from '@tanstack/react-table'
import { zhCN, enUS } from 'date-fns/locale'
import { Eye, MoreHorizontal } from 'lucide-react'
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
import { useUsageLogsContext } from '../context'
import { useUsageLogPermissions } from '../../../gql/useUsageLogPermissions'
import { UsageLog, UsageLogSource } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'

// Source color mapping for badges
const getSourceColor = (source: UsageLogSource) => {
  switch (source) {
    case 'api':
      return 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:border-blue-800'
    case 'playground':
      return 'bg-purple-100 text-purple-800 border-purple-200 dark:bg-purple-900/20 dark:text-purple-300 dark:border-purple-800'
    case 'test':
      return 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-300 dark:border-green-800'
    default:
      return 'bg-gray-100 text-gray-800 border-gray-200 dark:bg-gray-900/20 dark:text-gray-300 dark:border-gray-800'
  }
}

export function useUsageLogsColumns(): ColumnDef<UsageLog>[] {
  const { t, i18n } = useTranslation()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const permissions = useUsageLogPermissions()

  // Define all columns
  const columns: ColumnDef<UsageLog>[] = [
    {
      accessorKey: 'id',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('usageLogs.columns.id')} />
      ),
      cell: ({ row }) => (
        <div className='font-mono text-xs'>
          #{extractNumberID(row.getValue('id'))}
        </div>
      ),
      enableSorting: true,
      enableHiding: false,
    },
    // User column - only show if user has permission to view users
    ...(permissions.canViewUsers ? [{
      id: 'user',
      accessorKey: 'user',
      header: ({ column }) => (
        <DataTableColumnHeader 
          column={column} 
          title={t('usageLogs.columns.user')} 
        />
      ),
      cell: ({ row }) => {
        const user = row.original.user
        return (
          <div className='text-sm'>
            {user.firstName && user.lastName 
              ? `${user.firstName} ${user.lastName}`
              : user.email || t('usageLogs.columns.unknown')
            }
          </div>
        )
      },
      enableSorting: false,
      filterFn: (row, _id, value) => {
        // For client-side filtering, check if any of the selected users match
        if (value.length === 0) return true // No filter applied
        
        const user = row.original.user
        if (!user) return false
        
        return value.includes(user.id)
      },
    }] as ColumnDef<UsageLog>[] : []),
    {
      id: 'requestId',
      header: ({ column }) => (
        <DataTableColumnHeader 
          column={column} 
          title={t('usageLogs.columns.requestId')} 
        />
      ),
      cell: ({ row }) => (
        <div className='font-mono text-xs'>
          #{extractNumberID(row.original.requestID)}
        </div>
      ),
      enableSorting: false,
    },
    // Channel column - only show if user has permission to view channels
    ...(permissions.canViewChannels ? [{
      id: 'channel',
      accessorKey: 'channel',
      header: ({ column }) => (
        <DataTableColumnHeader 
          column={column} 
          title={t('usageLogs.columns.channel')} 
        />
      ),
      cell: ({ row }) => {
        const channel = row.original.channel
        return (
          <div className='text-sm'>
            {channel.name || t('usageLogs.columns.unknown')}
          </div>
        )
      },
      enableSorting: false,
      filterFn: (row, _id, value) => {
        // For client-side filtering, check if any of the selected channels match
        if (value.length === 0) return true // No filter applied
        
        const channel = row.original.channel
        if (!channel) return false
        
        return value.includes(channel.id)
      },
    }] as ColumnDef<UsageLog>[] : []),
    {
      id: 'modelId',
      header: ({ column }) => (
        <DataTableColumnHeader 
          column={column} 
          title={t('usageLogs.columns.modelId')} 
        />
      ),
      cell: ({ row }) => (
        <div className='font-mono text-sm max-w-[200px] truncate'>
          {row.original.modelID}
        </div>
      ),
      enableSorting: false,
      filterFn: (row, _id, value) => {
        // For client-side filtering, check if the model ID matches any of the selected values
        if (value.length === 0) return true // No filter applied
        
        const modelID = row.original.modelID
        return value.some((filterValue: string) => 
          modelID.toLowerCase().includes(filterValue.toLowerCase())
        )
      },
    },
    {
      id: 'tokens',
      header: ({ column }) => (
        <DataTableColumnHeader 
          column={column} 
          title={t('usageLogs.columns.totalTokens')} 
        />
      ),
      cell: ({ row }) => {
        const usage = row.original
        return (
          <div className='text-sm space-y-1'>
            <div className='flex items-center gap-1'>
              <span className='text-xs text-muted-foreground'>{t('usageLogs.columns.totalLabel')}</span>
              <span className='font-medium'>{usage.totalTokens.toLocaleString()}</span>
            </div>
            <div className='flex items-center gap-2 text-xs text-muted-foreground'>
              <span>{t('usageLogs.columns.inputLabel')} {usage.promptTokens.toLocaleString()}</span>
              <span>{t('usageLogs.columns.outputLabel')} {usage.completionTokens.toLocaleString()}</span>
            </div>
          </div>
        )
      },
      enableSorting: false,
    },
    {
      id: 'cacheTokens',
      header: ({ column }) => (
        <DataTableColumnHeader 
          column={column} 
          title={t('usageLogs.columns.cacheTokens')} 
        />
      ),
      cell: ({ row }) => {
        const usage = row.original
        const cachedTokens = usage.promptCachedTokens || 0
        const hasCache = cachedTokens > 0
        
        return (
          <div className='text-sm space-y-1'>
            <div className='flex items-center gap-1'>
              <span className={`text-xs font-medium ${
                hasCache 
                  ? 'text-green-600 dark:text-green-400' 
                  : 'text-muted-foreground'
              }`}>
                {hasCache ? '✓' : '—'}
              </span>
              <span className='font-medium'>{cachedTokens.toLocaleString()}</span>
            </div>
            {hasCache && (
              <div className='text-xs text-muted-foreground'>
                {t('usageLogs.columns.cacheHitRate', {
                  rate: ((cachedTokens / usage.promptTokens) * 100).toFixed(1)
                })}
              </div>
            )}
          </div>
        )
      },
      enableSorting: true,
      sortingFn: (rowA, rowB) => {
        const a = rowA.original.promptCachedTokens || 0
        const b = rowB.original.promptCachedTokens || 0
        return a - b
      },
    },
    {
      id: 'source',
      accessorKey: 'source',
      header: ({ column }) => (
        <DataTableColumnHeader 
          column={column} 
          title={t('usageLogs.columns.source')} 
        />
      ),
      cell: ({ row }) => {
        const source = row.getValue('source') as UsageLogSource
        return (
          <Badge className={getSourceColor(source)}>
            {t(`usageLogs.source.${source}`)}
          </Badge>
        )
      },
      enableSorting: false,
      filterFn: (row, id, value) => {
        return value.includes(row.getValue(id))
      },
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('usageLogs.columns.createdAt')} />
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
        const usageLog = row.original
        const { setCurrentUsageLog, setDetailDialogOpen } = useUsageLogsContext()

        const handleViewDetails = () => {
          setCurrentUsageLog(usageLog)
          setDetailDialogOpen(true)
        }

        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant='ghost'
                className='h-8 w-8 p-0'
                aria-label={t('usageLogs.actions.openMenu')}
              >
                <MoreHorizontal className='h-4 w-4' />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align='end'>
              <DropdownMenuItem onClick={handleViewDetails}>
                <Eye className='mr-2 h-4 w-4' />
                {t('usageLogs.actions.viewDetails')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    },
  ]

  return columns
}