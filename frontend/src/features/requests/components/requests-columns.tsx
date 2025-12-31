'use client'

import { format } from 'date-fns'
import { ColumnDef } from '@tanstack/react-table'
import { zhCN, enUS } from 'date-fns/locale'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { extractNumberID } from '@/lib/utils'
import { usePaginationSearch } from '@/hooks/use-pagination-search'
import { Badge } from '@/components/ui/badge'
import { useRequestPermissions } from '../../../hooks/useRequestPermissions'
import { Request } from '../data/schema'
import { DataTableColumnHeader } from '@/components/data-table-column-header'
import { getStatusColor } from './help'
import { formatDuration } from '@/utils/format-duration'

// Removed unused statusColors - using getStatusColor helper instead

export function useRequestsColumns(): ColumnDef<Request>[] {
  const { t, i18n } = useTranslation()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const permissions = useRequestPermissions()
  const { navigateWithSearch } = usePaginationSearch({ defaultPageSize: 20 })

  // Define all columns
  const columns: ColumnDef<Request>[] = [
    {
      accessorKey: 'id',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.id')} />,
      cell: ({ row }) => {
        const handleClick = useCallback(() => {
          navigateWithSearch({
            to: '/project/requests/$requestId',
            params: { requestId: row.original.id },
          })
        }, [row.original.id, navigateWithSearch])
        
        return (
          <button
            onClick={handleClick}
            className='font-mono text-xs text-primary hover:underline cursor-pointer'
          >
            #{extractNumberID(row.getValue('id'))}
          </button>
        )
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      id: 'modelId',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.modelId')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const request = row.original
        return <div className='font-mono text-xs'>{request.modelID || t('requests.columns.unknown')}</div>
      },
    },

    {
      id: 'stream',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.stream')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const isStream = row.original.stream
        return (
          <Badge
            className={
              isStream
                ? 'border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900/20 dark:text-green-300'
                : 'border-gray-200 bg-gray-100 text-gray-800 dark:border-gray-800 dark:bg-gray-900/20 dark:text-gray-300'
            }
          >
            {isStream ? t('requests.stream.streaming') : t('requests.stream.nonStreaming')}
          </Badge>
        )
      },
      filterFn: (row, _id, value) => {
        return value.includes(row.original.stream?.toString() || '-')
      },
    },
    {
      accessorKey: 'source',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.source')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const source = row.getValue('source') as string
        const sourceColors: Record<string, string> = {
          api: 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:border-blue-800',
          playground: 'bg-purple-100 text-purple-800 border-purple-200 dark:bg-purple-900/20 dark:text-purple-300 dark:border-purple-800',
          test: 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-300 dark:border-green-800',
        }
        return (
          <Badge
            className={
              sourceColors[source] ||
              'border-gray-200 bg-gray-100 text-gray-800 dark:border-gray-800 dark:bg-gray-900/20 dark:text-gray-300'
            }
          >
            {t(`requests.source.${source}`)}
          </Badge>
        )
      },
      filterFn: (row, id, value) => {
        return value.includes(row.getValue(id))
      },
    },
    // Channel column - only show if user has permission to view channels
    ...(permissions.canViewChannels
      ? ([
          {
            id: 'channel',
            accessorFn: (row) => row.channel?.id || '',
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.channel')} />,
            enableSorting: false,
            cell: ({ row }) => {
              const channel = row.original.channel

              if (!channel) {
                return <div className='text-muted-foreground font-mono text-xs'>-</div>
              }

              return <div className='font-mono text-xs'>{channel.name}</div>
            },
            filterFn: (row, _id, value) => {
              // For client-side filtering, check if any of the selected channels match
              if (value.length === 0) return true // No filter applied

              const channel = row.original.channel
              if (!channel) return false

              return value.includes(channel.id)
            },
          },
        ] as ColumnDef<Request>[])
      : []),
    // API Key column - only show if user has permission to view API keys
    ...(permissions.canViewApiKeys
      ? ([
          {
            accessorKey: 'apiKey',
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.apiKey')} />,
            enableSorting: false,
            cell: ({ row }) => {
              return <div className='font-mono text-xs'>{row.original.apiKey?.name || '-'}</div>
            },
          },
        ] as ColumnDef<Request>[])
      : []),

    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.status')} />,
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
      id: 'latency',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.latency')} />,
      cell: ({ row }) => {
        const request = row.original
        if (request.status !== 'completed' || request.metricsLatencyMs == null) {
          return <div className='text-muted-foreground text-xs'>-</div>
        }

        return <div className='font-mono text-xs'>{formatDuration(request.metricsLatencyMs)}</div>
      },
      enableSorting: false,
    },
    {
      id: 'firstTokenLatency',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.firstTokenLatency')} />,
      cell: ({ row }) => {
        const request = row.original
        if (!request.stream || request.status !== 'completed' || request.metricsFirstTokenLatencyMs == null) {
          return <div className='text-muted-foreground text-xs'>-</div>
        }

        return <div className='font-mono text-xs'>{formatDuration(request.metricsFirstTokenLatencyMs)}</div>
      },
      enableSorting: false,
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.createdAt')} />,
      cell: ({ row }) => {
        const date = new Date(row.getValue('createdAt'))
        return <div className='text-xs'>{format(date, 'yyyy-MM-dd HH:mm:ss', { locale })}</div>
      },
    },
  ]
  return columns
}
