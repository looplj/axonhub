import { ColumnDef } from '@tanstack/react-table'
import { format } from 'date-fns'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import LongText from '@/components/long-text'
import { Channel, ChannelType } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { DataTableRowActions } from './data-table-row-actions'

// Channel type configurations
const channelTypeConfig: Record<ChannelType, { label: string; color: string }> = {
  openai: { label: 'OpenAI', color: 'bg-green-100 text-green-800 border-green-200' },
  anthropic: { label: 'Anthropic', color: 'bg-orange-100 text-orange-800 border-orange-200' },
  gemini: { label: 'Gemini', color: 'bg-blue-100 text-blue-800 border-blue-200' },
  deepseek: { label: 'DeepSeek', color: 'bg-purple-100 text-purple-800 border-purple-200' },
  doubao: { label: 'Doubao', color: 'bg-red-100 text-red-800 border-red-200' },
  kimi: { label: 'Kimi', color: 'bg-indigo-100 text-indigo-800 border-indigo-200' },
}

export const columns: ColumnDef<Channel>[] = [
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
      <LongText className='max-w-36 font-medium'>{row.getValue('name')}</LongText>
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
    accessorKey: 'type',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='类型' />
    ),
    cell: ({ row }) => {
      const type = row.getValue('type') as ChannelType
      const config = channelTypeConfig[type]
      return (
        <Badge variant='outline' className={cn('capitalize', config.color)}>
          {config.label}
        </Badge>
      )
    },
    filterFn: (row, id, value) => {
      return value.includes(row.getValue(id))
    },
    enableSorting: false,
  },
  {
    accessorKey: 'status',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='状态' />
    ),
    cell: ({ row }) => {
      const status = row.getValue('status') as string
      return (
        <Badge variant={status === 'enabled' ? 'default' : 'secondary'}>
          {status === 'enabled' ? '启用' : '禁用'}
        </Badge>
      )
    },
    enableSorting: true,
    enableHiding: false,
  },
  {
    accessorKey: 'baseURL',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='Base URL' />
    ),
    cell: ({ row }) => (
      <LongText className='max-w-48 text-muted-foreground'>
        {row.getValue('baseURL')}
      </LongText>
    ),
    enableSorting: false,
  },
  {
    accessorKey: 'supportedModels',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='支持的模型' />
    ),
    cell: ({ row }) => {
      const models = row.getValue('supportedModels') as string[]
      return (
        <div className='flex flex-wrap gap-1 max-w-48'>
          {models.slice(0, 2).map((model) => (
            <Badge key={model} variant='secondary' className='text-xs'>
              {model}
            </Badge>
          ))}
          {models.length > 2 && (
            <Badge variant='secondary' className='text-xs'>
              +{models.length - 2}
            </Badge>
          )}
        </div>
      )
    },
    enableSorting: false,
  },
  {
    accessorKey: 'defaultTestModel',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='默认测试模型' />
    ),
    cell: ({ row }) => (
      <Badge variant='outline' className='text-xs'>
        {row.getValue('defaultTestModel')}
      </Badge>
    ),
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
        <div className='text-sm text-muted-foreground'>
          {format(date, 'yyyy-MM-dd HH:mm')}
        </div>
      )
    },
  },
  {
    id: 'actions',
    cell: DataTableRowActions,
    meta: {
      className: 'w-8',
    },
  },
]