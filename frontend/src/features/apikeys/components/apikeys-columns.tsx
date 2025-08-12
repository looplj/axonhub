import { useState } from 'react'
import { format } from 'date-fns'
import { ColumnDef } from '@tanstack/react-table'
import { Copy, Eye, EyeOff } from 'lucide-react'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import LongText from '@/components/long-text'
import { ApiKey } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { DataTableRowActions } from './data-table-row-actions'
import { useApiKeysContext } from '../context/apikeys-context'

function ApiKeyCell({ apiKey, fullApiKey }: { apiKey: string; fullApiKey: ApiKey }) {
  const [isVisible, setIsVisible] = useState(false)
  const { t } = useTranslation()
  const { openDialog } = useApiKeysContext()

  // 显示前8个字符和后4个字符，中间用省略号
  const shortKey = `${apiKey.slice(0, 8)}...${apiKey.slice(-4)}`
  const maskedKey = apiKey.replace(/./g, '*').slice(0, -4) + apiKey.slice(-4)

  const copyToClipboard = () => {
    navigator.clipboard.writeText(apiKey)
    toast.success(t('apikeys.messages.copied'))
  }

  const handleViewKey = () => {
    openDialog('view', fullApiKey)
  }

  return (
    <div className='flex items-center space-x-2 max-w-48'>
      <code className='bg-muted rounded px-2 py-1 font-mono text-sm truncate'>
        {isVisible ? shortKey : maskedKey}
      </code>
      <Button
        variant='ghost'
        size='sm'
        onClick={handleViewKey}
        className='h-6 w-6 p-0 flex-shrink-0'
        title={t('apikeys.actions.view')}
      >
        <Eye className='h-3 w-3' />
      </Button>
      <Button
        variant='ghost'
        size='sm'
        onClick={copyToClipboard}
        className='h-6 w-6 p-0 flex-shrink-0'
        title={t('apikeys.actions.copy')}
      >
        <Copy className='h-3 w-3' />
      </Button>
    </div>
  )
}

export const createColumns = (t: ReturnType<typeof useTranslation>['t']): ColumnDef<ApiKey>[] => [
  {
    id: 'select',
    header: ({ table }) => (
      <Checkbox
        checked={
          table.getIsAllPageRowsSelected() ||
          (table.getIsSomePageRowsSelected() && 'indeterminate')
        }
        onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
        aria-label={t('apikeys.columns.selectAll')}
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
        aria-label={t('apikeys.columns.selectRow')}
        className='translate-y-[2px]'
      />
    ),
    enableSorting: false,
    enableHiding: false,
  },
  {
    accessorKey: 'name',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('apikeys.columns.name')} />
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
      <DataTableColumnHeader column={column} title={t('apikeys.columns.key')} />
    ),
    cell: ({ row }) => <ApiKeyCell apiKey={row.getValue('key')} fullApiKey={row.original} />,
    enableSorting: false,
    meta: {
      className: 'max-w-48',
    },
  },
  {
    accessorKey: 'user',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('apikeys.columns.user')} />
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
    accessorKey: 'status',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('apikeys.columns.status')} />
    ),
    cell: ({ row }) => {
      const status = row.getValue('status') as string
      return (
        <div className={`text-sm ${status === 'enabled' ? 'text-green-600' : 'text-red-600'}`}>
          {status === 'enabled' ? t('apikeys.status.enabled') : t('apikeys.status.disabled')}
        </div>
      )
    },
  },
  {
    accessorKey: 'createdAt',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('apikeys.columns.createdAt')} />
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
      <DataTableColumnHeader column={column} title={t('apikeys.columns.updatedAt')} />
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
