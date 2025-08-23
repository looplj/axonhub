'use client'

import { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { format } from 'date-fns'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Role } from '../data/schema'
import { DataTableRowActions } from './data-table-row-actions'

export const createColumns = (t: ReturnType<typeof useTranslation>['t']): ColumnDef<Role>[] => [
  {
    id: 'search',
    header: () => null,
    cell: () => null,
    enableSorting: false,
    enableHiding: false,
    enableColumnFilter: true,
  },
  {
    id: 'select',
    header: ({ table }) => (
      <Checkbox
        checked={
          table.getIsAllPageRowsSelected() ||
          (table.getIsSomePageRowsSelected() && 'indeterminate')
        }
        onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
        aria-label={t('roles.columns.selectAll')}
        className='translate-y-[2px]'
      />
    ),
    cell: ({ row }) => (
      <Checkbox
        checked={row.getIsSelected()}
        onCheckedChange={(value) => row.toggleSelected(!!value)}
        aria-label={t('roles.columns.selectRow')}
        className='translate-y-[2px]'
      />
    ),
    enableSorting: false,
    enableHiding: false,
  },
  {
    accessorKey: 'code',
    header: t('roles.columns.code'),
    cell: ({ row }) => {
      const code = row.getValue('code') as string
      return (
        <div className='font-mono text-sm'>
          {code}
        </div>
      )
    },
  },
  {
    accessorKey: 'name',
    header: t('roles.columns.name'),
    cell: ({ row }) => {
      const name = row.getValue('name') as string
      return (
        <div className='font-medium'>
          {name}
        </div>
      )
    },
  },
  {
    accessorKey: 'scopes',
    header: t('roles.columns.scopes'),
    cell: ({ row }) => {
      const scopes = row.getValue('scopes') as string[]
      return (
        <div className='flex flex-wrap gap-1 max-w-[300px]'>
          {scopes.slice(0, 3).map((scope) => (
            <Badge key={scope} variant='secondary' className='text-xs'>
              {scope}
            </Badge>
          ))}
          {scopes.length > 3 && (
            <Badge variant='outline' className='text-xs'>
              +{scopes.length - 3} {t('roles.columns.moreScopes')}
            </Badge>
          )}
        </div>
      )
    },
  },
  {
    accessorKey: 'createdAt',
    header: t('roles.columns.createdAt'),
    cell: ({ row }) => {
      const date = row.getValue('createdAt') as Date
      return (
        <div className='text-muted-foreground'>
          {format(date, 'yyyy-MM-dd HH:mm')}
        </div>
      )
    },
  },
  {
    accessorKey: 'updatedAt',
    header: t('roles.columns.updatedAt'),
    cell: ({ row }) => {
      const date = row.getValue('updatedAt') as Date
      return (
        <div className='text-muted-foreground'>
          {format(date, 'yyyy-MM-dd HH:mm')}
        </div>
      )
    },
  },
  {
    id: 'actions',
    cell: ({ row }) => <DataTableRowActions row={row} />,
  },
]