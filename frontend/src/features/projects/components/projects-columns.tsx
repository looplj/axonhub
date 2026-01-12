'use client';

import { format } from 'date-fns';
import { ColumnDef, Row, Table } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { Project } from '../data/schema';
import { DataTableRowActions } from './data-table-row-actions';

export const createColumns = (t: ReturnType<typeof useTranslation>['t'], canWrite: boolean = false): ColumnDef<Project>[] => {
  const columns: ColumnDef<Project>[] = [
    {
      id: 'search',
      header: () => null,
      cell: () => null,
      enableSorting: false,
      enableHiding: false,
      enableColumnFilter: true,
      enableGlobalFilter: false,
      getUniqueValues: () => [],
    },
  ];

  // Only show select column if user has write permissions
  if (canWrite) {
    columns.push({
      id: 'select',
      header: ({ table }: { table: Table<Project> }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('common.columns.selectAll')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }: { row: Row<Project> }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('common.columns.selectRow')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    });
  }

  // Add other columns
  columns.push(
    {
      accessorKey: 'name',
      header: t('common.columns.name'),
      cell: ({ row }) => {
        const name = row.getValue('name') as string;
        return <div className='font-medium'>{name}</div>;
      },
    },
    {
      accessorKey: 'description',
      header: t('common.columns.description'),
      cell: ({ row }) => {
        const description = row.getValue('description') as string;
        return <div className='text-muted-foreground max-w-[300px] truncate'>{description || '-'}</div>;
      },
    },
    {
      accessorKey: 'status',
      header: t('common.columns.status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as string;
        return <Badge variant={status === 'active' ? 'default' : 'secondary'}>{t(`projects.status.${status}`)}</Badge>;
      },
    },
    {
          accessorKey: 'createdAt',
          header: t('common.columns.createdAt'),
          cell: ({ row }) => {
            const date = row.getValue('createdAt') as Date;
            return <div className='text-muted-foreground'>{format(date, 'yyyy-MM-dd HH:mm')}</div>;
          },
        },
        {
          accessorKey: 'updatedAt',
          header: t('common.columns.updatedAt'),
          cell: ({ row }) => {
            const date = row.getValue('updatedAt') as Date;
            return <div className='text-muted-foreground'>{format(date, 'yyyy-MM-dd HH:mm')}</div>;
          },
        },
        {
          id: 'actions',
          header: t('common.columns.actions'),
          cell: ({ row }) => <DataTableRowActions row={row} />,
        }
  );

  return columns;
};
