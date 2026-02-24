import { Cross2Icon } from '@radix-ui/react-icons';
import { IconSearch, IconFilter } from '@tabler/icons-react';
import { Table } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { DataTableViewOptions } from './data-table-view-options';
import { AgentRuntime } from '../data/schema';

interface DataTableToolbarProps {
  table: Table<AgentRuntime>;
  isFiltered: boolean;
  selectedCount: number;
}

export function DataTableToolbar({ table, isFiltered, selectedCount }: DataTableToolbarProps) {
  const { t } = useTranslation();

  const nameColumn = table.getColumn('name');
  const nameFilter = (nameColumn?.getFilterValue() as string) ?? '';

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-1 items-center gap-2">
          <div className="relative flex-1 max-w-sm">
            <IconSearch className="text-muted-foreground absolute top-1/2 left-2 h-4 w-4 -translate-y-1/2" />
            <Input
              placeholder={t('agentRuntimes.searchPlaceholder')}
              value={nameFilter}
              onChange={(event) => nameColumn?.setFilterValue(event.target.value)}
              className="h-8 w-full pl-8"
            />
          </div>
          {isFiltered && (
            <Button variant="ghost" onClick={() => table.resetColumnFilters()} className="h-8 px-2 lg:px-3">
              {t('common.reset')}
              <Cross2Icon className="ml-2 h-4 w-4" />
            </Button>
          )}
        </div>
        <div className="flex items-center gap-2">
          <DataTableViewOptions table={table} />
        </div>
      </div>
    </div>
  );
}
