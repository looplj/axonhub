import { useCallback, useState } from 'react';
import { format } from 'date-fns';
import { ColumnDef, Row, Table } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { usePrompts } from '../context/prompts-context';
import { Prompt } from '../data/schema';
import { DataTableRowActions } from './data-table-row-actions';
import { PromptsStatusDialog } from './prompts-status-dialog';

function StatusSwitchCell({ row }: { row: Row<Prompt> }) {
  const prompt = row.original;
  const [dialogOpen, setDialogOpen] = useState(false);

  const isEnabled = prompt.status === 'enabled';

  const handleSwitchClick = useCallback(() => {
    setDialogOpen(true);
  }, []);

  return (
    <>
      <Switch checked={isEnabled} onCheckedChange={handleSwitchClick} data-testid='prompt-status-switch' />
      {dialogOpen && <PromptsStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={prompt} />}
    </>
  );
}

export const createColumns = (t: ReturnType<typeof useTranslation>['t'], canWrite: boolean = true): ColumnDef<Prompt>[] => {
  return [
    ...(canWrite
      ? [
          {
            id: 'select',
            header: ({ table }: { table: Table<Prompt> }) => (
              <Checkbox
                checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
                onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
                aria-label={t('common.columns.selectAll')}
                className='translate-y-[2px]'
              />
            ),
            cell: ({ row }: { row: Row<Prompt> }) => (
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
      cell: ({ row }) => {
        const prompt = row.original;
        return (
          <div className='flex max-w-56 items-center gap-2'>
            <div className='truncate font-medium'>{prompt.name}</div>
          </div>
        );
      },
      meta: {
        className: 'md:table-cell min-w-48',
      },
      enableHiding: false,
      enableSorting: true,
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
      meta: {
        className: 'min-w-48',
      },
      enableSorting: false,
    },
    {
      accessorKey: 'role',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('prompts.columns.role')} />,
      cell: ({ row }) => {
        const role = row.getValue('role') as string;
        return <Badge variant='outline'>{role}</Badge>;
      },
      enableSorting: false,
    },
    {
      id: 'actionType',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('prompts.columns.actionType')} />,
      cell: ({ row }) => {
        const prompt = row.original;
        const actionType = prompt.settings?.action?.type;
        return <Badge variant='secondary'>{actionType ? t(`prompts.actionTypes.${actionType}`) : '-'}</Badge>;
      },
      enableSorting: false,
    },
    {
      id: 'conditions',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('prompts.columns.conditions')} />,
      cell: ({ row }) => {
        const prompt = row.original;
        const conditionsCount = prompt.settings?.conditions?.length || 0;
        return (
          <div className='flex justify-center'>
            <Badge variant='secondary'>{conditionsCount}</Badge>
          </div>
        );
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
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      id: 'actions',
      header: t('common.columns.actions'),
      cell: DataTableRowActions,
      meta: {
        className: 'w-[56px] min-w-[56px] pr-3 pl-0',
      },
      enableSorting: false,
      enableHiding: false,
    },
  ];
};
