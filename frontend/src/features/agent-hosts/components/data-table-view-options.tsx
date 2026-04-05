import { DropdownMenuTrigger } from '@radix-ui/react-dropdown-menu';
import { IconAdjustmentsHorizontal } from '@tabler/icons-react';
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
import { AgentHost } from '../data/schema';

interface DataTableViewOptionsProps<TData> {
  table: Table<TData>;
}

export function DataTableViewOptions({ table }: DataTableViewOptionsProps<AgentHost>) {
  const { t } = useTranslation();

  return (
    <DropdownMenu>
      <DropdownMenuContent align="end" className="w-[150px]">
        <DropdownMenuLabel>{t('common.toggleColumns')}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {table
          .getAllColumns()
          .filter((column) => typeof column.accessorFn !== 'undefined' && column.getCanHide())
          .map((column) => {
            return (
              <DropdownMenuCheckboxItem
                key={column.id}
                className="capitalize"
                checked={column.getIsVisible()}
                onCheckedChange={(value) => column.toggleVisibility(!!value)}
              >
                {column.id}
              </DropdownMenuCheckboxItem>
            );
          })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
