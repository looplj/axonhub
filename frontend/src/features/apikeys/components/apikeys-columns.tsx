import { format } from 'date-fns';
import { ColumnDef, Table, Row } from '@tanstack/react-table';
import { Copy, Eye } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { cn, extractNumberID } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import LongText from '@/components/long-text';
import { useApiKeysContext } from '../context/apikeys-context';
import { ApiKey } from '../data/schema';
import { DataTableRowActions } from './data-table-row-actions';

function ApiKeyCell({ apiKey, fullApiKey }: { apiKey: string; fullApiKey: ApiKey }) {
  const { t } = useTranslation();
  const { openDialog } = useApiKeysContext();

  // 显示前8个字符和后4个字符，中间用省略号
  const maskedKey = apiKey.replace(/./g, '*').slice(0, -4) + apiKey.slice(-4);

  const copyToClipboard = () => {
    navigator.clipboard.writeText(apiKey);
    toast.success(t('apikeys.messages.copied'));
  };

  const handleViewKey = () => {
    openDialog('view', fullApiKey);
  };

  return (
    <div className='flex max-w-48 items-center space-x-2'>
      <code className='bg-muted truncate rounded px-2 py-1 font-mono text-sm'>{maskedKey}</code>
      <Button variant='ghost' size='sm' onClick={handleViewKey} className='h-6 w-6 flex-shrink-0 p-0' title={t('apikeys.actions.view')}>
        <Eye className='h-3 w-3' />
      </Button>
      <Button variant='ghost' size='sm' onClick={copyToClipboard} className='h-6 w-6 flex-shrink-0 p-0' title={t('apikeys.actions.copy')}>
        <Copy className='h-3 w-3' />
      </Button>
    </div>
  );
}

export const createColumns = (t: ReturnType<typeof useTranslation>['t'], canWrite: boolean = true): ColumnDef<ApiKey>[] => [
  ...(canWrite
    ? [
        {
          id: 'select',
          header: ({ table }: { table: Table<ApiKey> }) => (
            <Checkbox
              checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
              onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
              aria-label={t('apikeys.columns.selectAll')}
              className='translate-y-[2px]'
            />
          ),
          cell: ({ row }: { row: Row<ApiKey> }) => (
            <Checkbox
              checked={row.getIsSelected()}
              onCheckedChange={(value) => row.toggleSelected(!!value)}
              aria-label={t('apikeys.columns.selectRow')}
              className='translate-y-[2px]'
            />
          ),
          enableSorting: false,
          enableHiding: false,
        },
      ]
    : []),
  {
    accessorKey: 'id',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('apikeys.columns.id')} />,
    cell: ({ row }) => <div className='font-mono text-xs'>#{extractNumberID(row.getValue('id'))}</div>,
    enableSorting: false,
  },
  {
    accessorKey: 'name',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('apikeys.columns.name')} />,
    cell: ({ row }) => <LongText className='max-w-36 font-medium'>{row.getValue('name')}</LongText>,
    meta: {
      className: 'md:table-cell',
    },
    filterFn: (row, _id, value) => {
      return String(row.getValue('name')).toLowerCase().includes(String(value).toLowerCase());
    },
    enableHiding: false,
  },
  {
    accessorKey: 'key',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('apikeys.columns.key')} />,
    cell: ({ row }) => <ApiKeyCell apiKey={row.getValue('key')} fullApiKey={row.original} />,
    enableSorting: false,
    meta: {
      className: 'max-w-48',
    },
  },
  {
    accessorKey: 'creator',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('apikeys.columns.creator')} />,
    cell: ({ row }) => {
      const creator = row.original.user;
      const displayName = creator ? `${creator.firstName} ${creator.lastName}` : '-';
      return <LongText className='text-muted-foreground max-w-24'>{displayName}</LongText>;
    },
    filterFn: (row, _id, value) => {
      const creator = row.original.user;
      if (!creator) return false;
      return value.includes(creator.id);
    },
    enableSorting: false,
  },
  {
    accessorKey: 'type',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('apikeys.columns.type')} />,
    cell: ({ row }) => {
      const type = row.getValue('type') as string;
      const typeText =
        {
          user: t('apikeys.type.user'),
          service_account: t('apikeys.type.service_account'),
        }[type] || type;

      const typeColor =
        {
          user: 'text-blue-600',
          service_account: 'text-purple-600',
        }[type] || 'text-muted-foreground';

      return <div className={`text-sm ${typeColor}`}>{typeText}</div>;
    },
    filterFn: (row, _id, value) => {
      return value.includes(row.getValue('type'));
    },
    enableSorting: false,
  },
  {
    accessorKey: 'status',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('apikeys.columns.status')} />,
    cell: ({ row }) => {
      const status = row.getValue('status') as string;
      const statusText =
        {
          enabled: t('apikeys.status.enabled'),
          disabled: t('apikeys.status.disabled'),
          archived: t('apikeys.status.archived'),
        }[status] || t('apikeys.status.disabled');

      const statusColor =
        {
          enabled: 'text-green-600',
          disabled: 'text-red-600',
          archived: 'text-orange-600',
        }[status] || 'text-red-600';

      return <div className={`text-sm ${statusColor}`}>{statusText}</div>;
    },
    filterFn: (row, _id, value) => {
      return value.includes(row.getValue('status'));
    },
    enableSorting: false,
  },
  {
    accessorKey: 'createdAt',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('apikeys.columns.createdAt')} />,
    cell: ({ row }) => {
      const date = row.getValue('createdAt') as Date;
      return <div className='text-muted-foreground'>{format(date, 'yyyy-MM-dd HH:mm')}</div>;
    },
  },
  {
    accessorKey: 'updatedAt',
    header: ({ column }) => <DataTableColumnHeader column={column} title={t('apikeys.columns.updatedAt')} />,
    cell: ({ row }) => {
      const date = row.getValue('updatedAt') as Date;
      return <div className='text-muted-foreground'>{format(date, 'yyyy-MM-dd HH:mm')}</div>;
    },
  },
  {
    id: 'actions',
    cell: DataTableRowActions,
  },
];
