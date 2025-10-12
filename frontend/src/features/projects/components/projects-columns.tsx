'use client'

import { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { format } from 'date-fns'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Project } from '../data/schema'
import { DataTableRowActions } from './data-table-row-actions'

export const createColumns = (
  t: ReturnType<typeof useTranslation>['t'],
  canWrite: boolean = false
): ColumnDef<Project>[] => {
  const columns: ColumnDef<Project>[] = [
    {
      id: 'search',
      header: () => null,
      cell: () => null,
      enableSorting: false,
      enableHiding: false,
      enableColumnFilter: true,
      enableGlobalFilter: false,
      getUniqueValues: () => [],
    },
  ]

  // Only show select column if user has write permissions
  if (canWrite) {
    columns.push({
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={
            table.getIsAllPageRowsSelected() ||
            (table.getIsSomePageRowsSelected() && 'indeterminate')
          }
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('projects.columns.selectAll')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('projects.columns.selectRow')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    })
  }

  // Add other columns
  columns.push(
    {
      accessorKey: 'slug',
      header: t('projects.columns.slug'),
      cell: ({ row }) => {
        const slug = row.getValue('slug') as string
        return (
          <div className='font-mono text-sm'>
            {slug}
          </div>
        )
      },
    },
    {
      accessorKey: 'name',
      header: t('projects.columns.name'),
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
      accessorKey: 'description',
      header: t('projects.columns.description'),
      cell: ({ row }) => {
        const description = row.getValue('description') as string
        return (
          <div className='text-muted-foreground max-w-[300px] truncate'>
            {description || '-'}
          </div>
        )
      },
    },
    {
      accessorKey: 'status',
      header: t('projects.columns.status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return (
          <Badge variant={status === 'active' ? 'default' : 'secondary'}>
            {t(`projects.status.${status}`)}
          </Badge>
        )
      },
    },
    {
      accessorKey: 'createdAt',
      header: t('projects.columns.createdAt'),
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
      header: t('projects.columns.updatedAt'),
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
  )

  return columns
}
