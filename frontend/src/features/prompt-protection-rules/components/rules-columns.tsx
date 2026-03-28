import { useCallback, useState } from 'react';
import { format } from 'date-fns';
import { ColumnDef, Row, Table } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { PromptProtectionRule } from '../data/schema';
import { RulesStatusDialog } from './rules-status-dialog';
import { DataTableRowActions } from './data-table-row-actions';

function StatusSwitchCell({ row }: { row: Row<PromptProtectionRule> }) {
  const rule = row.original;
  const [dialogOpen, setDialogOpen] = useState(false);
  const isEnabled = rule.status === 'enabled';

  const handleSwitchClick = useCallback(() => {
    setDialogOpen(true);
  }, []);

  return (
    <>
      <Switch checked={isEnabled} onCheckedChange={handleSwitchClick} />
      {dialogOpen && <RulesStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={rule} />}
    </>
  );
}

export const createColumns = (t: ReturnType<typeof useTranslation>['t'], canWrite: boolean = true): ColumnDef<PromptProtectionRule>[] => [
  ...(canWrite
    ? [
        {
          id: 'select',
          header: ({ table }: { table: Table<PromptProtectionRule> }) => (
            <Checkbox
              checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
              onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
              aria-label={t('common.columns.selectAll')}
              className='translate-y-[2px]'
            />
          ),
          cell: ({ row }: { row: Row<PromptProtectionRule> }) => (
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
      ]
    : []),
  {
    accessorKey: 'name',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.name')} />,
    cell: ({ row }) => <div className='truncate font-medium'>{row.original.name}</div>,
    meta: {
      className: 'min-w-40',
    },
    enableSorting: true,
    enableHiding: false,
  },
  {
    accessorKey: 'description',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.description')} />,
    cell: ({ row }) => {
      const description = row.original.description;
      return (
        <Tooltip>
          <TooltipTrigger asChild>
            <div className='max-w-64 truncate text-sm'>{description || '-'}</div>
          </TooltipTrigger>
          {description ? <TooltipContent>{description}</TooltipContent> : null}
        </Tooltip>
      );
    },
    meta: {
      className: 'min-w-48',
    },
    enableSorting: false,
  },
  {
    accessorKey: 'pattern',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('promptProtectionRules.columns.pattern')} />,
    cell: ({ row }) => (
      <Tooltip>
        <TooltipTrigger asChild>
          <div className='max-w-80 truncate font-mono text-xs'>{row.original.pattern}</div>
        </TooltipTrigger>
        <TooltipContent className='max-w-xl break-all font-mono text-xs'>{row.original.pattern}</TooltipContent>
      </Tooltip>
    ),
    meta: {
      className: 'min-w-48 max-w-80',
    },
    enableSorting: false,
  },
  {
    id: 'action',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('promptProtectionRules.columns.action')} />,
    cell: ({ row }) => (
      <Badge variant={row.original.settings.action === 'reject' ? 'destructive' : 'secondary'}>
        {t(`promptProtectionRules.actions.${row.original.settings.action}`)}
      </Badge>
    ),
    enableSorting: false,
  },
  {
    id: 'scopes',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('promptProtectionRules.columns.scopes')} />,
    cell: ({ row }) => {
      const scopes = row.original.settings.scopes || [];
      return <Badge variant='outline'>{scopes.length}</Badge>;
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
      const date = row.original.createdAt;
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
  },
  {
    id: 'actions',
    header: t('common.columns.actions'),
    cell: DataTableRowActions,
    meta: {
      className: 'w-[56px] min-w-[56px] pr-3 pl-0 sticky right-0 bg-inherit',
    },
    enableSorting: false,
    enableHiding: false,
  },
];
