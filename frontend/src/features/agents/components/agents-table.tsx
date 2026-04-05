import { useEffect, useMemo, useState } from 'react';
import {
  ColumnDef,
  ColumnFiltersState,
  RowSelectionState,
  SortingState,
  VisibilityState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { TableSkeleton } from '@/components/ui/table-skeleton';
import { ServerSidePagination } from '@/components/server-side-pagination';
import type { Agent, AgentConnection } from '../data/schema';

interface AgentsTableProps {
  columns: ColumnDef<Agent>[];
  data: Agent[];
  loading?: boolean;
  pageInfo?: AgentConnection['pageInfo'];
  pageSize: number;
  totalCount?: number;
  nameFilter: string;
  sorting: SortingState;
  onSortingChange: (updater: SortingState | ((prev: SortingState) => SortingState)) => void;
  onNextPage: () => void;
  onPreviousPage: () => void;
  onPageSizeChange: (pageSize: number) => void;
  onNameFilterChange: (filter: string) => void;
}

export function AgentsTable({
  columns,
  data,
  loading,
  pageInfo,
  pageSize,
  totalCount,
  nameFilter,
  sorting,
  onSortingChange,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onNameFilterChange,
}: AgentsTableProps) {
  const { t } = useTranslation();
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({});
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);

  useEffect(() => {
    const newColumnFilters: ColumnFiltersState = [];
    if (nameFilter) {
      newColumnFilters.push({ id: 'name', value: nameFilter });
    }
    setColumnFilters(newColumnFilters);
  }, [nameFilter]);

  const handleColumnFiltersChange = (updater: ColumnFiltersState | ((prev: ColumnFiltersState) => ColumnFiltersState)) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater;
    setColumnFilters(newFilters);

    const nameFilterValue = newFilters.find((filter) => filter.id === 'name')?.value as string;
    const newNameFilter = nameFilterValue || '';

    if (newNameFilter !== nameFilter) {
      onNameFilterChange(newNameFilter);
    }
  };

  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
    },
    enableRowSelection: true,
    getRowId: (row) => row.id,
    onRowSelectionChange: setRowSelection,
    onSortingChange,
    onColumnFiltersChange: handleColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    manualPagination: true,
    manualFiltering: true,
  });

  const selectedCount = useMemo(() => table.getFilteredSelectedRowModel().rows.length, [table, rowSelection, data]);

  useEffect(() => {
    if (Object.keys(rowSelection).length > 0 && data.length > 0) {
      const dataIds = new Set(data.map((agent) => agent.id));
      const selectedIds = Object.keys(rowSelection);
      const anySelectedIdMissing = selectedIds.some((id) => !dataIds.has(id));
      if (anySelectedIdMissing) {
        setRowSelection({});
      }
    }
  }, [data, rowSelection]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <div className='mb-4 flex items-center justify-between'>
        <div className='flex flex-1 items-center space-x-2'>
          <Input
            placeholder={t('agents.filters.filterByName')}
            value={(table.getColumn('name')?.getFilterValue() as string) ?? ''}
            onChange={(event) => table.getColumn('name')?.setFilterValue(event.target.value)}
            className='h-8 w-[150px] lg:w-[250px]'
          />
        </div>
      </div>

      <div className='shadow-soft relative mt-4 flex-1 overflow-auto overflow-x-hidden rounded-2xl border border-[var(--table-border)]'>
        <Table data-testid='agents-table' className='border-separate border-spacing-0 rounded-2xl bg-[var(--table-background)]'>
          <TableHeader className='sticky top-0 z-20 bg-[var(--table-header)] shadow-sm'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row border-0'>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    colSpan={header.colSpan}
                    className={`${header.column.columnDef.meta?.className ?? ''} text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase`}
                  >
                    {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody className='space-y-1 !bg-[var(--table-background)] p-2'>
            {loading ? (
              <TableSkeleton rows={pageSize} columns={columns.length} />
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && 'selected'}
                  className='group/row table-row-hover rounded-xl border-0 !bg-[var(--table-background)] transition-all duration-200 ease-in-out'
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className={`${cell.column.columnDef.meta?.className ?? ''} border-0 bg-inherit px-4 py-3`}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow className='!bg-[var(--table-background)]'>
                <TableCell colSpan={columns.length} className='h-24 !bg-[var(--table-background)] text-center'>
                  {t('common.noData')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <div className='mt-4 flex-shrink-0'>
        <ServerSidePagination
          pageInfo={pageInfo}
          pageSize={pageSize}
          dataLength={data.length}
          totalCount={totalCount}
          selectedRows={selectedCount}
          onNextPage={onNextPage}
          onPreviousPage={onPreviousPage}
          onPageSizeChange={onPageSizeChange}
        />
      </div>
    </div>
  );
}

