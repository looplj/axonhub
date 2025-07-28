import { ColumnDef } from '@tanstack/react-table'
import { format } from 'date-fns'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Role } from '../data/schema'
import { DataTableRowActions } from './data-table-row-actions'

export const columns: ColumnDef<Role>[] = [
  {
    id: 'select',
    header: ({ table }) => (
      <Checkbox
        checked={
          table.getIsAllPageRowsSelected() ||
          (table.getIsSomePageRowsSelected() && 'indeterminate')
        }
        onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
        aria-label='Select all'
        className='translate-y-[2px]'
      />
    ),
    cell: ({ row }) => (
      <Checkbox
        checked={row.getIsSelected()}
        onCheckedChange={(value) => row.toggleSelected(!!value)}
        aria-label='Select row'
        className='translate-y-[2px]'
      />
    ),
    enableSorting: false,
    enableHiding: false,
  },
  {
    accessorKey: 'code',
    header: '角色代码',
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
    header: '角色名称',
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
    header: '权限范围',
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
              +{scopes.length - 3} 更多
            </Badge>
          )}
        </div>
      )
    },
  },
  {
    accessorKey: 'createdAt',
    header: '创建时间',
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
    header: '更新时间',
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

export default columns;