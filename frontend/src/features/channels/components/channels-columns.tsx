import { ColumnDef } from '@tanstack/react-table'
import { format } from 'date-fns'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import LongText from '@/components/long-text'
import { Channel, ChannelType } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { DataTableRowActions } from './data-table-row-actions'

// Channel type configurations
const getChannelTypeConfig = (t: ReturnType<typeof useTranslation>['t']): Record<ChannelType, { label: string; color: string }> => ({
  openai: { label: t('channels.types.openai'), color: 'bg-green-100 text-green-800 border-green-200' },
  anthropic: { label: t('channels.types.anthropic'), color: 'bg-orange-100 text-orange-800 border-orange-200' },
  anthropic_aws: { label: t('channels.types.anthropic_aws'), color: 'bg-orange-100 text-orange-800 border-orange-200' },
  gemini: { label: t('channels.types.gemini'), color: 'bg-blue-100 text-blue-800 border-blue-200' },
  deepseek: { label: t('channels.types.deepseek'), color: 'bg-purple-100 text-purple-800 border-purple-200' },
  doubao: { label: t('channels.types.doubao'), color: 'bg-red-100 text-red-800 border-red-200' },
  kimi: { label: t('channels.types.kimi'), color: 'bg-indigo-100 text-indigo-800 border-indigo-200' },
})

export const createColumns = (t: ReturnType<typeof useTranslation>['t']): ColumnDef<Channel>[] => {
  const channelTypeConfig = getChannelTypeConfig(t)
  
  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={
            table.getIsAllPageRowsSelected() ||
            (table.getIsSomePageRowsSelected() && 'indeterminate')
          }
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('channels.columns.selectAll')}
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
          aria-label={t('channels.columns.selectRow')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    },
    {
      accessorKey: 'name',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('channels.columns.name')} />
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
        <DataTableColumnHeader column={column} title={t('channels.columns.type')} />
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
        <DataTableColumnHeader column={column} title={t('channels.columns.status')} />
      ),
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return (
          <Badge variant={status === 'enabled' ? 'default' : 'secondary'}>
            {status === 'enabled' ? t('channels.status.enabled') : t('channels.status.disabled')}
          </Badge>
        )
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'baseURL',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('channels.columns.baseURL')} />
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
        <DataTableColumnHeader column={column} title={t('channels.columns.supportedModels')} />
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
        <DataTableColumnHeader column={column} title={t('channels.columns.defaultTestModel')} />
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
        <DataTableColumnHeader column={column} title={t('channels.columns.createdAt')} />
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
}