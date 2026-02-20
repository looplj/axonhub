import { useCallback, useState } from 'react';
import { ColumnDef, Row, Table } from '@tanstack/react-table';
import { format } from 'date-fns';
import { useTranslation } from 'react-i18next';
import { Checkbox } from '@/components/ui/checkbox';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import type { Agent } from '../data/schema';
import { AgentsStatusDialog } from './agents-status-dialog';
import { DataTableRowActions } from './data-table-row-actions';

function StatusSwitchCell({ row }: { row: Row<Agent> }) {
  const agent = row.original;
  const [dialogOpen, setDialogOpen] = useState(false);

  const isEnabled = agent.status === 'enabled';

  const handleSwitchClick = useCallback(() => {
    setDialogOpen(true);
  }, []);

  return (
    <>
      <Switch checked={isEnabled} onCheckedChange={handleSwitchClick} data-testid='agent-status-switch' />
      {dialogOpen && <AgentsStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={agent} />}
    </>
  );
}

export const createColumns = (t: ReturnType<typeof useTranslation>['t'], canWrite: boolean = true): ColumnDef<Agent>[] => {
  const baseColumns: ColumnDef<Agent>[] = [
    {
      accessorKey: 'name',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.name')} />,
      cell: ({ row }) => {
        const agent = row.original;
        return <div className='max-w-64 truncate font-medium'>{agent.name}</div>;
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
      accessorKey: 'model',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('agents.columns.model')} />,
      cell: ({ row }) => {
        const model = row.getValue('model') as string;
        return model ? <Badge variant='secondary'>{model}</Badge> : <span className='text-muted-foreground text-xs'>-</span>;
      },
      enableSorting: false,
    },
    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.status')} />,
      cell: StatusSwitchCell,
      enableSorting: false,
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
      header: ({ table }: { table: Table<Agent> }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('common.columns.selectAll')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }: { row: Row<Agent> }) => (
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

