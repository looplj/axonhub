import { format } from 'date-fns'
import { ColumnDef, Row } from '@tanstack/react-table'
import { IconPlayerPlay, IconChevronDown } from '@tabler/icons-react'
import {
  OpenAI,
  Anthropic,
  Google,
  DeepSeek,
  Doubao,
  Moonshot,
} from '@lobehub/icons'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import LongText from '@/components/long-text'
import { useChannels } from '../context/channels-context'
import { Channel, ChannelType } from '../data/schema'
import { useTestChannel } from '../data/channels'
import { DataTableColumnHeader } from './data-table-column-header'
import { DataTableRowActions } from './data-table-row-actions'

// Channel type configurations
const getChannelTypeConfig = (
  t: ReturnType<typeof useTranslation>['t']
): Record<
  ChannelType,
  {
    label: string
    color: string
    icon: React.ComponentType<{ size?: number; className?: string }>
  }
> => ({
  openai: {
    label: t('channels.types.openai'),
    color: 'bg-green-100 text-green-800 border-green-200',
    icon: OpenAI,
  },
  anthropic: {
    label: t('channels.types.anthropic'),
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    icon: Anthropic,
  },
  anthropic_aws: {
    label: t('channels.types.anthropic_aws'),
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    icon: Anthropic,
  },
  anthropic_gcp: {
    label: t('channels.types.anthropic_gcp'),
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    icon: Anthropic,
  },
  gemini: {
    label: t('channels.types.gemini'),
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: Google,
  },
  deepseek: {
    label: t('channels.types.deepseek'),
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: DeepSeek,
  },
  doubao: {
    label: t('channels.types.doubao'),
    color: 'bg-red-100 text-red-800 border-red-200',
    icon: Doubao,
  },
  kimi: {
    label: t('channels.types.kimi'),
    color: 'bg-indigo-100 text-indigo-800 border-indigo-200',
    icon: Moonshot,
  },
  anthropic_fake: {
    label: t('channels.types.anthropic_fake'),
    color: 'bg-gray-100 text-gray-800 border-gray-200',
    icon: Anthropic,
  },
  openai_fake: {
    label: t('channels.types.openai_fake'),
    color: 'bg-gray-100 text-gray-800 border-gray-200',
    icon: OpenAI,
  },
})

// Test Cell Component to handle hooks properly
function TestCell({ row }: { row: Row<Channel> }) {
  const { t } = useTranslation()
  const channel = row.original
  const { setOpen, setCurrentRow } = useChannels()
  const testChannel = useTestChannel()

  const handleDefaultTest = async () => {
    // Test with default test model
    try {
      await testChannel.mutateAsync({
        channelID: channel.id,
        modelID: channel.defaultTestModel || undefined,
      })
    } catch (_error) {
      // Error is already handled by the useTestChannel hook via toast
    }
  }

  const handleOpenTestDialog = () => {
    setCurrentRow(channel)
    setOpen('test')
  }

  return (
    <div className='flex items-center gap-1'>
      <Button
        size='sm'
        variant='outline'
        className='h-8 px-3'
        onClick={handleDefaultTest}
        disabled={testChannel.isPending}
      >
        <IconPlayerPlay className='mr-1 h-3 w-3' />
        {t('channels.actions.test')}
      </Button>
      <Button
        size='sm'
        variant='outline'
        className='h-8 w-8 p-0'
        onClick={handleOpenTestDialog}
      >
        <IconChevronDown className='h-3 w-3' />
      </Button>
    </div>
  )
}

export const createColumns = (
  t: ReturnType<typeof useTranslation>['t']
): ColumnDef<Channel>[] => {
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
        <DataTableColumnHeader
          column={column}
          title={t('channels.columns.name')}
        />
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
      accessorKey: 'type',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.columns.type')}
        />
      ),
      cell: ({ row }) => {
        const type = row.getValue('type') as ChannelType
        const config = channelTypeConfig[type]
        const IconComponent = config.icon
        return (
          <Badge variant='outline' className={cn('capitalize', config.color)}>
            <div className='flex items-center gap-2'>
              <IconComponent size={16} className='shrink-0' />
              <span>{config.label}</span>
            </div>
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
        <DataTableColumnHeader
          column={column}
          title={t('channels.columns.status')}
        />
      ),
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return (
          <Badge variant={status === 'enabled' ? 'default' : 'secondary'}>
            {status === 'enabled'
              ? t('channels.status.enabled')
              : t('channels.status.disabled')}
          </Badge>
        )
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'baseURL',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.columns.baseURL')}
        />
      ),
      cell: ({ row }) => (
        <LongText className='text-muted-foreground max-w-48'>
          {row.getValue('baseURL')}
        </LongText>
      ),
      enableSorting: false,
    },
    {
      accessorKey: 'supportedModels',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.columns.supportedModels')}
        />
      ),
      cell: ({ row }) => {
        const models = row.getValue('supportedModels') as string[]
        return (
          <div className='flex max-w-48 flex-wrap gap-1'>
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
      accessorKey: 'createdAt',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.columns.createdAt')}
        />
      ),
      cell: ({ row }) => {
        const date = row.getValue('createdAt') as Date
        return (
          <div className='text-muted-foreground text-sm'>
            {format(date, 'yyyy-MM-dd HH:mm')}
          </div>
        )
      },
    },
    {
      id: 'test',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.columns.test')}
        />
      ),
      cell: TestCell,
      enableSorting: false,
      enableHiding: false,
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
