import { memo, useCallback, useState } from 'react';
import { ColumnDef, Row, Table } from '@tanstack/react-table';
import { format } from 'date-fns';
import { useTranslation } from 'react-i18next';
import { Checkbox } from '@/components/ui/checkbox';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import type { MessageChannel } from '../data/schema';
import { MessageChannelsStatusDialog } from './message-channels-status-dialog';
import { DataTableRowActions } from './data-table-row-actions';

const StatusSwitchCell = memo(({ row }: { row: Row<MessageChannel> }) => {
  const channel = row.original;
  const [dialogOpen, setDialogOpen] = useState(false);

  const isEnabled = channel.status === 'enabled';

  const handleSwitchClick = useCallback(() => {
    setDialogOpen(true);
  }, []);

  return (
    <div className='flex justify-center'>
      <Switch checked={isEnabled} onCheckedChange={handleSwitchClick} data-testid='message-channel-status-switch' />
      {dialogOpen && <MessageChannelsStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={channel} />}
    </div>
  );
});

StatusSwitchCell.displayName = 'StatusSwitchCell';

function AgentInstanceBindingsCell({ row }: { row: Row<MessageChannel> }) {
  const channel = row.original;
  const { t } = useTranslation();

  const bindings = channel.agentInstanceBindings?.edges || [];
  const enabledCount = bindings.filter((edge) => edge.node.enabled).length;
  const totalCount = bindings.length;

  if (totalCount === 0) {
    return (
      <span className='text-muted-foreground text-xs'>
        {t('messageChannels.columns.notBound')}
      </span>
    );
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant='outline' className='cursor-help'>
          {enabledCount}/{totalCount} {t('messageChannels.columns.bindings')}
        </Badge>
      </TooltipTrigger>
      <TooltipContent>
        <div className='space-y-1 max-w-xs'>
          <p className='font-medium'>{t('messageChannels.columns.boundAgents')}</p>
          {bindings.map((edge) => (
            <div key={edge.node.id} className='text-xs flex items-center gap-2'>
              <span className={edge.node.enabled ? 'text-green-500' : 'text-gray-400'}>
                {edge.node.enabled ? '●' : '○'}
              </span>
              <span className='truncate'>{edge.node.agentInstance.name}</span>
              {edge.node.order > 0 && <span className='text-muted-foreground'>({edge.node.order})</span>}
            </div>
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  );
}

export const createColumns = (
  t: ReturnType<typeof useTranslation>['t'],
  canWrite: boolean = true
): ColumnDef<MessageChannel>[] => {
  const baseColumns: ColumnDef<MessageChannel>[] = [
    {
      accessorKey: 'name',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.name')} />,
      cell: ({ row }) => {
        const channel = row.original;
        return <div className='max-w-64 truncate font-medium'>{channel.name}</div>;
      },
      meta: { className: 'min-w-48' },
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'description',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.description')} />,
      cell: ({ row }) => {
        const description = row.getValue('description') as string;
        return (
          <Tooltip>
            <TooltipTrigger asChild>
              <div className='max-w-64 truncate text-sm'>{description || '-'}</div>
            </TooltipTrigger>
            {description && <TooltipContent>{description}</TooltipContent>}
          </Tooltip>
        );
      },
      meta: { className: 'min-w-48' },
      enableSorting: false,
    },
    {
      accessorKey: 'type',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('messageChannels.columns.type')} />,
      cell: ({ row }) => {
        const type = row.getValue('type') as string;
        return <Badge variant='secondary'>{type.toUpperCase()}</Badge>;
      },
      enableSorting: true,
    },
    {
      accessorKey: 'agentInstanceBindings',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('messageChannels.columns.agentInstances')} />,
      cell: AgentInstanceBindingsCell,
      enableSorting: false,
    },
    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.status')} />,
      cell: StatusSwitchCell,
      meta: {
        className: 'text-center',
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.createdAt')} />,
      cell: ({ row }) => {
        const raw = row.getValue('createdAt') as unknown;
        const date = raw instanceof Date ? raw : new Date(raw as string);
        if (Number.isNaN(date.getTime())) return <span className='text-muted-foreground text-xs'>-</span>;

        return (
          <Tooltip>
            <TooltipTrigger asChild>
              <div className='text-muted-foreground cursor-help text-sm'>{format(date, 'yyyy-MM-dd')}</div>
            </TooltipTrigger>
            <TooltipContent>{format(date, 'yyyy-MM-dd HH:mm:ss')}</TooltipContent>
          </Tooltip>
        );
      },
      enableSorting: true,
      enableHiding: false,
    },
  ];

  if (!canWrite) return baseColumns;

  return [
    {
      id: 'select',
      header: ({ table }: { table: Table<MessageChannel> }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('common.columns.selectAll')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }: { row: Row<MessageChannel> }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('common.columns.selectRow')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    },
    ...baseColumns,
    {
      id: 'actions',
      header: t('common.columns.actions'),
      cell: DataTableRowActions,
      meta: { className: 'w-[56px] min-w-[56px] pr-3 pl-0' },
      enableSorting: false,
      enableHiding: false,
    },
  ];
};
