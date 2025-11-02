'use client'

import { format } from 'date-fns'
import { useNavigate } from '@tanstack/react-router'
import { ColumnDef } from '@tanstack/react-table'
import { zhCN, enUS } from 'date-fns/locale'
import { Eye, MoreHorizontal, FileText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { extractNumberID } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Trace } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { usePaginationSearch } from '@/hooks/use-pagination-search'

export function useTracesColumns(): ColumnDef<Trace>[] {
  const { t, i18n } = useTranslation()
  const locale = i18n.language === 'zh' ? zhCN : enUS
    const { navigateWithSearch } = usePaginationSearch({ defaultPageSize: 20 })
  

  // Define all columns
  const columns: ColumnDef<Trace>[] = [
    {
      accessorKey: 'id',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('traces.columns.id')} />,
      cell: ({ row }) => <div className='font-mono text-xs'>#{extractNumberID(row.getValue('id'))}</div>,
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'traceID',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('traces.columns.traceId')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const traceID = row.getValue('traceID') as string
        return (
          <div className='max-w-64 truncate font-mono text-xs' title={traceID}>
            {traceID}
          </div>
        )
      },
    },
    // {
    //   id: 'project',
    //   header: ({ column }) => <DataTableColumnHeader column={column} title={t('traces.columns.project')} />,
    //   enableSorting: false,
    //   cell: ({ row }) => {
    //     const project = row.original.project
    //     return <div className='font-mono text-xs'>{project?.name || t('traces.columns.unknown')}</div>
    //   },
    // },
    {
      id: 'thread',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('traces.columns.thread')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const thread = row.original.thread
        if (!thread) {
          return <div className='text-muted-foreground font-mono text-xs'>{t('traces.columns.noThread')}</div>
        }
        return (
          <div className='max-w-64 truncate font-mono text-xs' title={thread.threadID || thread.id}>
            {thread.threadID || thread.id}
          </div>
        )
      },
    },
    {
      id: 'requestCount',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('traces.columns.requestCount')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const count = row.original.requests?.totalCount || 0
        return (
          <Badge variant='secondary' className='font-mono text-xs'>
            {count}
          </Badge>
        )
      },
    },
    {
      id: 'details',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('traces.columns.details')} />,
      cell: ({ row }) => {
        const handleViewDetails = () => {
          navigateWithSearch({ to: '/project/traces/$traceId', params: { traceId: row.original.id } })
        }

        return (
          <Button variant='outline' size='sm' onClick={handleViewDetails}>
            <FileText className='mr-2 h-4 w-4' />
            {t('traces.actions.viewDetails')}
          </Button>
        )
      },
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('traces.columns.createdAt')} />,
      cell: ({ row }) => {
        const date = new Date(row.getValue('createdAt'))
        return <div className='text-xs'>{format(date, 'yyyy-MM-dd HH:mm:ss', { locale })}</div>
      },
    },
    // {
    //   accessorKey: 'updatedAt',
    //   header: ({ column }) => <DataTableColumnHeader column={column} title={t('traces.columns.updatedAt')} />,
    //   cell: ({ row }) => {
    //     const date = new Date(row.getValue('updatedAt'))
    //     return <div className='text-xs'>{format(date, 'yyyy-MM-dd HH:mm:ss', { locale })}</div>
    //   },
    // },
    // {
    //   id: 'actions',
    //   cell: ({ row }) => {
    //     const trace = row.original
    //     const navigate = useNavigate()

    //     return (
    //       <DropdownMenu>
    //         <DropdownMenuTrigger asChild>
    //           <Button variant='ghost' className='h-8 w-8 p-0'>
    //             <span className='sr-only'>{t('traces.actions.openMenu')}</span>
    //             <MoreHorizontal className='h-4 w-4' />
    //           </Button>
    //         </DropdownMenuTrigger>
    //         <DropdownMenuContent align='end'>
    //           <DropdownMenuItem onClick={() => {
    //             navigate({ to: '/project/traces/$traceId', params: { traceId: trace.id } })
    //           }}>
    //             <Eye className='mr-2 h-4 w-4' />
    //             {t('traces.actions.viewDetails')}
    //           </DropdownMenuItem>
    //         </DropdownMenuContent>
    //       </DropdownMenu>
    //     )
    //   },
    // },
  ]

  return columns
}
