import { DropdownMenuTrigger } from '@radix-ui/react-dropdown-menu';
import { MixerHorizontalIcon } from '@radix-ui/react-icons';
import { Table } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu';

interface DataTableViewOptionsProps<TData> {
  table: Table<TData>;
}

const COLUMN_LABEL_KEYS: Record<string, string> = {
  firstUserQuery: 'traces.columns.userQuery',
  traceID: 'traces.columns.traceId',
  thread: 'traces.columns.thread',
  requestCount: 'traces.columns.requestCount',
  details: 'traces.columns.details',
  createdAt: 'common.columns.createdAt',
  lastRequestAt: 'traces.columns.lastRequestAt',
};

export function DataTableViewOptions<TData>({ table }: DataTableViewOptionsProps<TData>) {
  const { t } = useTranslation();

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant='outline' size='sm' className='ml-auto hidden h-8 lg:flex'>
          <MixerHorizontalIcon className='mr-2 h-4 w-4' />
          {t('common.view')}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[150px]'>
        <DropdownMenuLabel>{t('common.toggleColumns')}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {table
          .getAllColumns()
          .filter((column) => {
            const isDataColumn =
              typeof column.accessorFn !== 'undefined' ||
              ('accessorKey' in column.columnDef && column.columnDef.accessorKey !== undefined);
            return (isDataColumn || column.id === 'details') && column.getCanHide();
          })
          .map((column) => {
            return (
              <DropdownMenuCheckboxItem
                key={column.id}
                className='capitalize'
                checked={column.getIsVisible()}
                onCheckedChange={(value) => column.toggleVisibility(!!value)}
              >
                {t(COLUMN_LABEL_KEYS[column.id] ?? `traces.columns.${column.id}`)}
              </DropdownMenuCheckboxItem>
            );
          })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
