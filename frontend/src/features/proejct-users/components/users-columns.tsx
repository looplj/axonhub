'use client';

import { ColumnDef, Row, Table } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import LongText from '@/components/long-text';
import { User } from '../data/schema';
import { DataTableRowActions } from './data-table-row-actions';

export const createColumns = (
  t: ReturnType<typeof useTranslation>['t'],
  canWrite: boolean = false,
  canReadRoles: boolean = false
): ColumnDef<User>[] => {
  const columns: ColumnDef<User>[] = [];

  // Only show select column if user has write permissions (for potential bulk operations)
  if (canWrite) {
    columns.push({
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label='Select all'
        />
      ),
      cell: ({ row }) => (
        <Checkbox checked={row.getIsSelected()} onCheckedChange={(value) => row.toggleSelected(!!value)} aria-label='Select row' />
      ),
      enableSorting: false,
      enableHiding: false,
    });
  }

  // Add other columns
  columns.push(
    {
      accessorKey: 'firstName',
      header: t('users.columns.firstName'),
      cell: ({ row }) => <LongText>{row.getValue('firstName')}</LongText>,
    },
    {
      accessorKey: 'lastName',
      header: t('users.columns.lastName'),
      cell: ({ row }) => <LongText>{row.getValue('lastName')}</LongText>,
    },
    {
      accessorKey: 'email',
      header: t('users.columns.email'),
      cell: ({ row }) => <LongText>{row.getValue('email')}</LongText>,
    },
    {
      accessorKey: 'isOwner',
      header: t('users.columns.projectOwner'),
      cell: ({ row }) => {
        const isOwner = row.getValue('isOwner') as boolean;
        return isOwner ? (
          <Badge variant='default'>{t('users.badges.projectOwner')}</Badge>
        ) : (
          <Badge variant='secondary'>{t('users.badges.member')}</Badge>
        );
      },
    }
  );

  // Only add roles column if user has permission to view roles
  if (canReadRoles) {
    columns.push({
      accessorKey: 'roles',
      header: t('users.columns.projectRoles'),
      cell: ({ row }) => {
        const user = row.original;
        const roles = user.roles?.edges;
        if (!roles || roles.length === 0) {
          return <span className='text-muted-foreground'>{t('users.badges.noRoles')}</span>;
        }
        return (
          <div className='flex flex-wrap gap-1'>
            {roles.slice(0, 2).map((edge, index) => (
              <Badge key={index} variant='default'>
                {edge.node.name}
              </Badge>
            ))}
            {roles.length > 2 && <Badge variant='secondary'>+{roles.length - 2}</Badge>}
          </div>
        );
      },
    });
  }

  columns.push(
    {
      accessorKey: 'scopes',
      header: t('users.columns.projectScopes'),
      cell: ({ row }) => {
        const user = row.original;
        const scopes = user.scopes;
        if (!scopes || scopes.length === 0) {
          return <span className='text-muted-foreground'>{t('users.badges.noScopes')}</span>;
        }
        return (
          <div className='flex flex-wrap gap-1'>
            {scopes.slice(0, 3).map((scope, index) => (
              <Badge key={index} variant='outline'>
                {scope}
              </Badge>
            ))}
            {scopes.length > 3 && <Badge variant='secondary'>+{scopes.length - 3}</Badge>}
          </div>
        );
      },
    },
    {
      accessorKey: 'status',
      header: t('users.columns.status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as string;
        return (
          <Badge variant={status === 'activated' ? 'default' : 'secondary'}>
            {status === 'activated' ? t('users.status.activated') : t('users.status.deactivated')}
          </Badge>
        );
      },
    },
    {
      accessorKey: 'createdAt',
      header: t('users.columns.createdAt'),
      cell: ({ row }) => {
        const date = new Date(row.getValue('createdAt'));
        return date.toLocaleDateString();
      },
    },
    {
      accessorKey: 'updatedAt',
      header: t('users.columns.updatedAt'),
      cell: ({ row }) => {
        const date = new Date(row.getValue('updatedAt'));
        return date.toLocaleDateString();
      },
    },
    {
      id: 'actions',
      cell: ({ row }) => <DataTableRowActions row={row} />,
    }
  );

  return columns;
};
