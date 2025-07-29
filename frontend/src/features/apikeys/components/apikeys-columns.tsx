import { useState } from 'react'
import { format } from 'date-fns'
import { ColumnDef } from '@tanstack/react-table'
import { Copy, Eye, EyeOff } from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import LongText from '@/components/long-text'
import { ApiKey } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { DataTableRowActions } from './data-table-row-actions'

function ApiKeyCell({ apiKey }: { apiKey: string }) {
  const [isVisible, setIsVisible] = useState(false)

  const maskedKey = apiKey.replace(/./g, '*').slice(0, -4) + apiKey.slice(-4)

  const copyToClipboard = () => {
    navigator.clipboard.writeText(apiKey)
    toast.success('API Key 已复制到剪贴板')
  }

  return (
    <div className='flex items-center space-x-2'>
      <code className='bg-muted rounded px-2 py-1 font-mono text-sm'>
        {isVisible ? apiKey : maskedKey}
      </code>
      <Button
        variant='ghost'
        size='sm'
        onClick={() => setIsVisible(!isVisible)}
        className='h-6 w-6 p-0'
      >
        {isVisible ? (
          <EyeOff className='h-3 w-3' />
        ) : (
          <Eye className='h-3 w-3' />
        )}
      </Button>
      <Button
        variant='ghost'
        size='sm'
        onClick={copyToClipboard}
        className='h-6 w-6 p-0'
      >
        <Copy className='h-3 w-3' />
      </Button>
    </div>
  )
}

export const columns: ColumnDef<ApiKey>[] = [
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
    meta: {
      className: cn(
        'sticky md:table-cell left-0 z-10 rounded-tl',
        'bg-background transition-colors duration-200 group-hover/row:bg-muted group-data-[state=selected]/row:bg-muted'
      ),
    },
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
    accessorKey: 'name',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='名称' />
    ),
    cell: ({ row }) => (
      <LongText className='max-w-36 font-medium'>
        {row.getValue('name')}
      </LongText>
    ),
    meta: {
      className: cn(
        'drop-shadow-[0_1px_2px_rgb(0_0_0_/_0.1)] dark:drop-shadow-[0_1px_2px_rgb(255_255_255_/_0.1)] lg:drop-shadow-none',
        'bg-background transition-colors duration-200 group-hover/row:bg-muted group-data-[state=selected]/row:bg-muted',
        'sticky left-6 md:table-cell'
      ),
    },
    enableHiding: false,
  },
  {
    accessorKey: 'key',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='API Key' />
    ),
    cell: ({ row }) => <ApiKeyCell apiKey={row.getValue('key')} />,
    enableSorting: false,
  },
  {
    accessorKey: 'user',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='用户' />
    ),
    cell: ({ row }) => {
      const user = row.original.user
      const displayName = user ? `${user.firstName} ${user.lastName}` : '-'
      return (
        <LongText className='text-muted-foreground max-w-24'>
          {displayName}
        </LongText>
      )
    },
    enableSorting: false,
  },
  {
    accessorKey: 'createdAt',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='创建时间' />
    ),
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
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='更新时间' />
    ),
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
    cell: DataTableRowActions,
  },
]
