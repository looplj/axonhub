import { format } from 'date-fns'
import { ColumnDef, Row } from '@tanstack/react-table'
import { IconPlayerPlay, IconChevronDown, IconChevronRight, IconAlertTriangle } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useCallback } from 'react'
import { cn } from '@/lib/utils'
import { formatDuration } from '@/utils/format-duration'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useChannels } from '../context/channels-context'
import { useTestChannel } from '../data/channels'
import { CHANNEL_CONFIGS, getProvider } from '../data/config_channels'
import { Channel, ChannelType } from '../data/schema'
import { DataTableColumnHeader } from './data-table-column-header'
import { DataTableRowActions } from './data-table-row-actions'

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
      <Button size='sm' variant='outline' className='h-8 px-3' onClick={handleDefaultTest} disabled={testChannel.isPending}>
        <IconPlayerPlay className='mr-1 h-3 w-3' />
        {t('channels.actions.test')}
      </Button>
      <Button size='sm' variant='outline' className='h-8 w-8 p-0' onClick={handleOpenTestDialog}>
        <IconChevronDown className='h-3 w-3' />
      </Button>
    </div>
  )
}

export const createColumns = (t: ReturnType<typeof useTranslation>['t']): ColumnDef<Channel>[] => {
  return [
    {
      id: 'expand',
      header: () => null,
      meta: {
        className: 'w-8 min-w-8',
      },
      cell: ({ row }) => (
        <Button
          variant='ghost'
          size='sm'
          className='h-6 w-6 p-0'
          onClick={() => row.toggleExpanded()}
        >
          {row.getIsExpanded() ? (
            <IconChevronDown className='h-4 w-4' />
          ) : (
            <IconChevronRight className='h-4 w-4' />
          )}
        </Button>
      ),
      enableSorting: false,
      enableHiding: false,
    },
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('channels.columns.selectAll')}
          className='translate-y-[2px]'
        />
      ),
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
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.name')} />,
      cell: ({ row }) => {
        const channel = row.original
        const hasError = !!channel.errorMessage

        const content = (
          <div className='flex max-w-36 items-center gap-2'>
            {hasError && <IconAlertTriangle className='text-destructive h-4 w-4 shrink-0' />}
            <div className={cn('font-medium truncate', hasError && 'text-destructive')}>{row.getValue('name')}</div>
          </div>
        )

        if (hasError) {
          return (
            <Tooltip>
              <TooltipTrigger asChild>
                {content}
              </TooltipTrigger>
              <TooltipContent>
                <div className='space-y-1'>
                  <p className='text-destructive text-sm'>{t(`channels.messages.${channel.errorMessage}`)}</p>
                </div>
              </TooltipContent>
            </Tooltip>
          )
        }

        return content
      },
      meta: {
        className: 'md:table-cell',
      },
      enableHiding: false,
      enableSorting: false,
    },
    {
      id: 'provider',
      accessorKey: 'type',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.provider')} />,
      cell: ({ row }) => {
        const type = row.original.type
        const config = CHANNEL_CONFIGS[type]
        const provider = getProvider(type)
        const IconComponent = config.icon
        return (
          <Badge variant='outline' className={cn('capitalize', config.color)}>
            <div className='flex items-center gap-2'>
              <IconComponent size={16} className='shrink-0' />
              <span>{t(`channels.providers.${provider}`)}</span>
            </div>
          </Badge>
        )
      },
      filterFn: (row, id, value) => {
        return value.includes(row.original.type)
      },
      enableSorting: false,
      enableHiding: false,
    },
    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.status')} />,
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        const getBadgeVariant = () => {
          switch (status) {
            case 'enabled':
              return 'default'
            case 'archived':
              return 'outline'
            default:
              return 'secondary'
          }
        }
        const getStatusText = () => {
          switch (status) {
            case 'enabled':
              return t('channels.status.enabled')
            case 'archived':
              return t('channels.status.archived')
            default:
              return t('channels.status.disabled')
          }
        }
        return <Badge variant={getBadgeVariant()}>{getStatusText()}</Badge>
      },
      enableSorting: false,
      enableHiding: false,
    },

    {
      accessorKey: 'tags',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.tags')} />,
      cell: ({ row }) => {
        const tags = (row.getValue('tags') as string[]) || []
        if (tags.length === 0) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <div className='flex max-w-48 flex-wrap gap-1'>
            {tags.slice(0, 2).map((tag) => (
              <Badge key={tag} variant='outline' className='text-xs'>
                {tag}
              </Badge>
            ))}
            {tags.length > 2 && (
              <Badge variant='outline' className='text-xs'>
                +{tags.length - 2}
              </Badge>
            )}
          </div>
        )
      },
      filterFn: (row, id, value) => {
        const tags = (row.getValue(id) as string[]) || []
        // Single select: value is a string, not an array
        return tags.includes(value as string)
      },
      enableSorting: false,
      enableHiding: true,
    },
    {
      id: 'model',
      accessorFn: () => '', // Virtual column for filtering only
      header: () => null,
      cell: () => null,
      filterFn: () => true, // Server-side filtering, always return true
      enableSorting: false,
      enableHiding: true,
    },
    {
      accessorKey: 'channelPerformance',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.performance')} />,
      cell: ({ row }) => {
        const performance = row.getValue('channelPerformance') as any
        if (!performance) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }

        const avgLatency = performance.avgStreamFirstTokenLatencyMs || performance.avgLatencyMs || 0
        const avgTokensPerSec = performance.avgStreamTokenPerSecond || performance.avgTokenPerSecond || 0

        return (
          <div className='space-y-1'>
            <Tooltip>
              <TooltipTrigger asChild>
                <div className='cursor-help text-xs'>
                  <span className='text-muted-foreground'>{t('channels.columns.firstTokenLatency')}: </span>
                  <span className='font-medium'>{formatDuration(avgLatency)}</span>
                </div>
              </TooltipTrigger>
              <TooltipContent>
                <p>{t('channels.columns.firstTokenLatencyFull')}</p>
              </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <div className='cursor-help text-xs'>
                  <span className='text-muted-foreground'>{t('channels.columns.tokensPerSecond')}: </span>
                  <span className='font-medium'>{avgTokensPerSec.toFixed(1)}</span>
                </div>
              </TooltipTrigger>
              <TooltipContent>
                <p>{t('channels.columns.tokensPerSecondFull')}</p>
              </TooltipContent>
            </Tooltip>
          </div>
        )
      },
      enableSorting: false,
    },
    {
      accessorKey: 'supportedModels',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.supportedModels')} />,
      cell: ({ row }) => {
        const channel = row.original
        const models = row.getValue('supportedModels') as string[]
        const { setOpen, setCurrentRow } = useChannels()

        const handleOpenModelsDialog = useCallback(() => {
          setCurrentRow(channel)
          setOpen('viewModels')
        }, [channel, setCurrentRow, setOpen])

        return (
          <div className='flex items-center gap-2'>
            <div className='flex max-w-48 flex-wrap gap-1'>
              {models.slice(0, 2).map((model) => (
                <Badge key={model} variant='secondary' className='text-xs'>
                  {model}
                </Badge>
              ))}
              {models.length > 2 && (
                <Badge
                  variant='secondary'
                  className='text-xs cursor-pointer hover:bg-primary hover:text-primary-foreground transition-colors'
                  onClick={handleOpenModelsDialog}
                  title={t('channels.actions.viewModels')}
                >
                  +{models.length - 2}
                </Badge>
              )}
            </div>
          </div>
        )
      },
      enableSorting: false,
    },

    {
      id: 'test',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.test')} />,
      cell: TestCell,
      enableSorting: false,
      enableHiding: false,
    },
    {
      accessorKey: 'orderingWeight',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.weight')} />,
      cell: ({ row }) => {
        const weight = row.getValue('orderingWeight') as number | null
        if (weight == null) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return <span className='font-mono text-sm'>{weight}</span>
      },
      meta: {
        className: 'text-right',
      },
      sortingFn: 'alphanumeric',
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.createdAt')} />,
      cell: ({ row }) => {
        const date = row.getValue('createdAt') as Date
        return <div className='text-muted-foreground text-sm'>{format(date, 'yyyy-MM-dd HH:mm')}</div>
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      id: 'actions',
      header: () => null,
      cell: DataTableRowActions,
      meta: {
        className: 'w-[56px] min-w-[56px] pr-3 pl-0',
      },
      enableSorting: false,
      enableHiding: false,
    },
  ]
}
