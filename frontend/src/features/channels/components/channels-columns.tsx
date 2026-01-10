import { useCallback, useState, memo } from 'react';
import { format } from 'date-fns';
import { DotsHorizontalIcon } from '@radix-ui/react-icons';
import { ColumnDef, Row, Table } from '@tanstack/react-table';
import {
  IconPlayerPlay,
  IconChevronDown,
  IconChevronRight,
  IconAlertTriangle,
  IconEdit,
  IconArchive,
  IconTrash,
  IconCheck,
  IconWeight,
  IconTransform,
  IconNetwork,
  IconAdjustments,
  IconRoute,
  IconCopy,
} from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { formatDuration } from '@/utils/format-duration';
import { usePermissions } from '@/hooks/usePermissions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { useChannels } from '../context/channels-context';
import { useTestChannel } from '../data/channels';
import { CHANNEL_CONFIGS, getProvider } from '../data/config_channels';
import { Channel } from '../data/schema';
import { ChannelsStatusDialog } from './channels-status-dialog';

// Status Switch Cell Component to handle status toggle with confirmation dialog
const StatusSwitchCell = memo(({ row }: { row: Row<Channel> }) => {
  const channel = row.original;
  const [dialogOpen, setDialogOpen] = useState(false);

  const isEnabled = channel.status === 'enabled';
  const isArchived = channel.status === 'archived';

  const handleSwitchClick = useCallback(() => {
    if (!isArchived) {
      setDialogOpen(true);
    }
  }, [isArchived]);

  return (
    <>
      <Switch checked={isEnabled} onCheckedChange={handleSwitchClick} disabled={isArchived} data-testid='channel-status-switch' />
      {dialogOpen && <ChannelsStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={channel} />}
    </>
  );
});

StatusSwitchCell.displayName = 'StatusSwitchCell';

// Action Cell Component to handle hooks properly
const ActionCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const channel = row.original;
  const { setOpen, setCurrentRow } = useChannels();
  const { channelPermissions } = usePermissions();
  const testChannel = useTestChannel();
  const hasError = !!channel.errorMessage;

  const handleDefaultTest = async () => {
    try {
      await testChannel.mutateAsync({
        channelID: channel.id,
        modelID: channel.defaultTestModel || undefined,
      });
    } catch (_error) {}
  };

  const handleOpenTestDialog = useCallback(() => {
    setCurrentRow(channel);
    setOpen('test');
  }, [channel, setCurrentRow, setOpen]);

  const handleEdit = useCallback(() => {
    setCurrentRow(channel);
    setOpen('edit');
  }, [channel, setCurrentRow, setOpen]);

  return (
    <div className='flex items-center gap-1'>
      {channelPermissions.canEdit && (
        <Button size='sm' variant='outline' className='h-8 w-8 p-0' onClick={handleEdit}>
          <IconEdit className='h-3 w-3' />
        </Button>
      )}
      <Button size='sm' variant='outline' className='h-8 px-3' onClick={handleDefaultTest} disabled={testChannel.isPending}>
        <IconPlayerPlay className='mr-1 h-3 w-3' />
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button size='sm' variant='outline' className='h-8 w-8 p-0'>
            <DotsHorizontalIcon className='h-3 w-3' />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[160px]'>
          {channelPermissions.canWrite && (
            <>
              <DropdownMenuItem onClick={handleOpenTestDialog}>
                <IconPlayerPlay size={16} className='mr-2' />
                {t('channels.actions.test')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
            </>
          )}
          {channelPermissions.canEdit && (
            <DropdownMenuItem onClick={handleEdit}>
              <IconEdit size={16} className='mr-2' />
              {t('common.actions.edit')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canEdit && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('duplicate');
              }}
            >
              <IconCopy size={16} className='mr-2' />
              {t('common.actions.duplicate')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('modelMapping');
              }}
            >
              <IconRoute size={16} className='mr-2' />
              {t('channels.dialogs.settings.modelMapping.title')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('overrides');
              }}
            >
              <IconAdjustments size={16} className='mr-2' />
              {t('channels.dialogs.settings.overrides.action')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('proxy');
              }}
            >
              <IconNetwork size={16} className='mr-2' />
              {t('channels.dialogs.proxy.action')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('transformOptions');
              }}
            >
              <IconTransform size={16} className='mr-2' />
              {t('channels.dialogs.transformOptions.action')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('weight');
              }}
            >
              <IconWeight size={16} className='mr-2' />
              {t('channels.dialogs.weight.action')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && hasError && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('errorResolved');
              }}
              className='text-green-500!'
            >
              <IconCheck size={16} className='mr-2' />
              {t('channels.actions.errorResolved')}
            </DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          {channelPermissions.canWrite && channel.status !== 'archived' && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('archive');
              }}
              className='text-orange-500!'
            >
              <IconArchive size={16} className='mr-2' />
              {t('common.buttons.archive')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('delete');
              }}
              className='text-red-500!'
            >
              <IconTrash size={16} className='mr-2' />
              {t('common.buttons.delete')}
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
});

ActionCell.displayName = 'ActionCell';

const ExpandCell = memo(({ row }: { row: any }) => (
  <Button variant='ghost' size='sm' className='h-6 w-6 p-0' onClick={() => row.toggleExpanded()}>
    {row.getIsExpanded() ? <IconChevronDown className='h-4 w-4' /> : <IconChevronRight className='h-4 w-4' />}
  </Button>
));

ExpandCell.displayName = 'ExpandCell';

// Memoized cell components to avoid recreating on every render
const NameCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const channel = row.original;
  const hasError = !!channel.errorMessage;

  const content = (
    <div className='flex max-w-56 items-center gap-2'>
      {hasError && <IconAlertTriangle className='text-destructive h-4 w-4 shrink-0' />}
      <div className={cn('truncate font-medium', hasError && 'text-destructive')}>{row.getValue('name')}</div>
    </div>
  );

  if (hasError) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>{content}</TooltipTrigger>
        <TooltipContent>
          <div className='space-y-1'>
            <p className='text-destructive text-sm'>
              {t(`channels.messages.${channel.errorMessage}`, {
                fallback: channel.errorMessage,
              })}
            </p>
          </div>
        </TooltipContent>
      </Tooltip>
    );
  }

  return content;
});

NameCell.displayName = 'NameCell';

const ProviderCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const type = row.original.type;
  const config = CHANNEL_CONFIGS[type];
  const provider = getProvider(type);
  const IconComponent = config.icon;
  return (
    <Badge variant='outline' className={cn('capitalize', config.color)}>
      <div className='flex items-center gap-2'>
        <IconComponent size={16} className='shrink-0' />
        <span>{t(`channels.providers.${provider}`)}</span>
      </div>
    </Badge>
  );
});

ProviderCell.displayName = 'ProviderCell';

const TagsCell = memo(({ row }: { row: Row<Channel> }) => {
  const tags = (row.getValue('tags') as string[]) || [];
  if (tags.length === 0) {
    return <span className='text-muted-foreground text-xs'>-</span>;
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
  );
});

TagsCell.displayName = 'TagsCell';

const PerformanceCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const performance = row.getValue('channelPerformance') as any;
  if (!performance) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  const avgLatency = performance.avgStreamFirstTokenLatencyMs || performance.avgLatencyMs || 0;
  const avgTokensPerSec = performance.avgStreamTokenPerSecond || performance.avgTokenPerSecond || 0;

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
  );
});

PerformanceCell.displayName = 'PerformanceCell';

const SupportedModelsCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const channel = row.original;
  const models = row.getValue('supportedModels') as string[];
  const { setOpen, setCurrentRow } = useChannels();

  const handleOpenModelsDialog = useCallback(() => {
    setCurrentRow(channel);
    setOpen('viewModels');
  }, [channel, setCurrentRow, setOpen]);

  return (
    <div className='flex items-center gap-2'>
      <div className='flex flex-wrap gap-1 overflow-hidden'>
        {models.slice(0, 5).map((model) => (
          <Badge key={model} variant='secondary' className='block max-w-48 truncate text-left text-xs'>
            {model}
          </Badge>
        ))}
        {models.length > 5 && (
          <Badge
            variant='secondary'
            className='hover:bg-primary hover:text-primary-foreground cursor-pointer text-xs transition-colors'
            onClick={handleOpenModelsDialog}
            title={t('channels.actions.viewModels')}
          >
            +{models.length - 5}
          </Badge>
        )}
      </div>
    </div>
  );
});

SupportedModelsCell.displayName = 'SupportedModelsCell';

const OrderingWeightCell = memo(({ row }: { row: Row<Channel> }) => {
  const weight = row.getValue('orderingWeight') as number | null;
  if (weight == null) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }
  return <span className='font-mono text-sm'>{weight}</span>;
});

OrderingWeightCell.displayName = 'OrderingWeightCell';

const CreatedAtCell = memo(({ row }: { row: Row<Channel> }) => {
  const raw = row.getValue('createdAt') as unknown;
  const date = raw instanceof Date ? raw : new Date(raw as string);

  if (Number.isNaN(date.getTime())) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className='text-muted-foreground cursor-help text-sm'>{format(date, 'yyyy-MM-dd')}</div>
      </TooltipTrigger>
      <TooltipContent>{format(date, 'yyyy-MM-dd HH:mm:ss')}</TooltipContent>
    </Tooltip>
  );
});

CreatedAtCell.displayName = 'CreatedAtCell';

export const createColumns = (t: ReturnType<typeof useTranslation>['t'], canWrite: boolean = true): ColumnDef<Channel>[] => {
  return [
    {
      id: 'expand',
      header: () => null,
      meta: {
        className: 'w-8 min-w-8',
      },
      cell: ExpandCell,
      enableSorting: false,
      enableHiding: false,
    },
    ...(canWrite
      ? [
          {
            id: 'select',
            header: ({ table }: { table: Table<Channel> }) => (
              <Checkbox
                checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
                onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
                aria-label={t('channels.columns.selectAll')}
                className='translate-y-[2px]'
              />
            ),
            cell: ({ row }: { row: Row<Channel> }) => (
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
        ]
      : []),
    {
      accessorKey: 'name',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.name')} />,
      cell: NameCell,
      meta: {
        className: 'md:table-cell min-w-48',
      },
      enableHiding: false,
      enableSorting: true,
    },
    {
      id: 'provider',
      accessorKey: 'type',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.provider')} />,
      cell: ProviderCell,
      filterFn: (row, _id, value) => {
        return value.includes(row.original.type);
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.status')} />,
      cell: StatusSwitchCell,
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'tags',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.tags')} />,
      cell: TagsCell,
      filterFn: (row, id, value) => {
        const tags = (row.getValue(id) as string[]) || [];
        // Single select: value is a string, not an array
        return tags.includes(value as string);
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
      enableColumnFilter: false,
      enableGlobalFilter: false,
    },
    {
      accessorKey: 'channelPerformance',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.channelPerformance')} />,
      cell: PerformanceCell,
      enableSorting: false,
    },
    {
      accessorKey: 'supportedModels',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.supportedModels')} />,
      cell: SupportedModelsCell,
      meta: {
        className: 'max-w-64',
      },
      enableSorting: false,
    },
    {
      accessorKey: 'orderingWeight',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('channels.columns.orderingWeight')} className='justify-center' />
      ),
      cell: OrderingWeightCell,
      meta: {
        className: 'w-20 min-w-20 text-center',
      },
      sortingFn: 'alphanumeric',
      enableSorting: true,
      enableHiding: true,
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.createdAt')} />,
      cell: CreatedAtCell,
      enableSorting: true,
      enableHiding: false,
    },
    ...(canWrite
      ? [
          {
            id: 'action',
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.action')} />,
            cell: ActionCell,
            enableSorting: false,
            enableHiding: false,
          },
        ]
      : []),
  ];
};
