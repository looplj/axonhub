import React, { useState, useMemo, useEffect } from 'react';
import {
  ColumnDef,
  ColumnFiltersState,
  RowData,
  RowSelectionState,
  SortingState,
  VisibilityState,
  flexRender,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  useReactTable,
} from '@tanstack/react-table';
import { IconX, IconUserOff, IconArchive, IconCheck } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { ServerSidePagination } from '@/components/server-side-pagination';
import { useApiKeysContext } from '../context/apikeys-context';
import { ApiKey, ApiKeyConnection } from '../data/schema';
import { DataTableToolbar } from './data-table-toolbar';

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string;
  }
}

interface DataTableProps {
  columns: ColumnDef<ApiKey>[];
  data: ApiKey[];
  loading: boolean;
  pageInfo?: ApiKeyConnection['pageInfo'];
  pageSize: number;
  totalCount?: number;
  nameFilter: string;
  statusFilter: string[];
  userFilter: string[];
  onNextPage: () => void;
  onPreviousPage: () => void;
  onPageSizeChange: (pageSize: number) => void;
  onNameFilterChange: (value: string) => void;
  onStatusFilterChange: (value: string[]) => void;
  onUserFilterChange: (value: string[]) => void;
  onResetFilters?: () => void;
}

export function ApiKeysTable({
  columns,
  data,
  loading,
  pageInfo,
  pageSize,
  totalCount,
  nameFilter,
  statusFilter,
  userFilter,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onNameFilterChange,
  onStatusFilterChange,
  onUserFilterChange,
  onResetFilters,
}: DataTableProps) {
  const { t } = useTranslation();
  const { setResetRowSelection, setSelectedApiKeys, openDialog } = useApiKeysContext();
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({});
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);
  const [sorting, setSorting] = useState<SortingState>([]);

  useEffect(() => {
    const resetFn = () => {
      setRowSelection({});
    };
    setResetRowSelection(resetFn);
  }, [setResetRowSelection]);

  React.useEffect(() => {
    const newFilters: ColumnFiltersState = [];
    if (nameFilter) {
      newFilters.push({ id: 'name', value: nameFilter });
    }
    if (statusFilter.length > 0) {
      newFilters.push({ id: 'status', value: statusFilter });
    }
    if (userFilter.length > 0) {
      newFilters.push({ id: 'creator', value: userFilter });
    }
    setColumnFilters(newFilters);
  }, [nameFilter, statusFilter, userFilter]);

  const handleColumnFiltersChange = (updater: ColumnFiltersState | ((prev: ColumnFiltersState) => ColumnFiltersState)) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater;
    setColumnFilters(newFilters);

    const nameFilterValue = newFilters.find((f) => f.id === 'name')?.value;
    const statusFilterValue = newFilters.find((f) => f.id === 'status')?.value;
    const userFilterValue = newFilters.find((f) => f.id === 'creator')?.value;

    const newNameFilter = typeof nameFilterValue === 'string' ? nameFilterValue : '';
    if (newNameFilter !== nameFilter) {
      onNameFilterChange(newNameFilter);
    }

    const newStatusFilter = Array.isArray(statusFilterValue) ? statusFilterValue : [];
    if (JSON.stringify(newStatusFilter.sort()) !== JSON.stringify(statusFilter.sort())) {
      onStatusFilterChange(newStatusFilter);
    }

    const newUserFilter = Array.isArray(userFilterValue) ? userFilterValue : [];
    if (JSON.stringify(newUserFilter.sort()) !== JSON.stringify(userFilter.sort())) {
      onUserFilterChange(newUserFilter);
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
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnFiltersChange: handleColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualFiltering: true,
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    getRowId: (row) => row.id,
  });

  const filteredSelectedRows = useMemo(() => table.getFilteredSelectedRowModel().rows, [table, rowSelection, data]);

  const selectedCount = filteredSelectedRows.length;

  useEffect(() => {
    const selected = filteredSelectedRows.map((row) => row.original as ApiKey);
    setSelectedApiKeys(selected);
  }, [filteredSelectedRows, setSelectedApiKeys]);

  useEffect(() => {
    if (selectedCount === 0) {
      setSelectedApiKeys([]);
    }
  }, [selectedCount, setSelectedApiKeys]);

  // Clear rowSelection when data changes and selected rows no longer exist
  useEffect(() => {
    if (Object.keys(rowSelection).length > 0 && data.length > 0) {
      const dataIds = new Set(data.map((apiKey) => apiKey.id));
      const selectedIds = Object.keys(rowSelection);
      const anySelectedIdMissing = selectedIds.some((id) => !dataIds.has(id));

      if (anySelectedIdMissing) {
        // Some selected rows no longer exist in the new data, clear selection
        setRowSelection({});
      }
    }
  }, [data, rowSelection]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <DataTableToolbar table={table} onResetFilters={onResetFilters} />
      <div className='shadow-soft relative mt-4 flex-1 overflow-auto rounded-2xl border border-[var(--table-border)]'>
        <Table className='border-separate border-spacing-0 rounded-2xl bg-[var(--table-background)]'>
          <TableHeader className='sticky top-0 z-20 bg-[var(--table-header)] shadow-sm'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row border-0'>
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead
                      key={header.id}
                      colSpan={header.colSpan}
                      className={`${header.column.columnDef.meta?.className ?? ''} text-muted-foreground border-0 text-xs font-semibold tracking-wider uppercase`}
                    >
                      {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody className='space-y-1 !bg-[var(--table-background)] p-2'>
            {loading ? (
              <TableRow className='border-0 !bg-[var(--table-background)]'>
                <TableCell colSpan={columns.length} className='h-24 border-0 !bg-[var(--table-background)] text-center'>
                  {t('common.loading')}
                </TableCell>
              </TableRow>
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
          selectedRows={Object.keys(rowSelection).length}
          onNextPage={onNextPage}
          onPreviousPage={onPreviousPage}
          onPageSizeChange={onPageSizeChange}
        />
      </div>
      {selectedCount > 0 && (
        <div className='fixed bottom-6 left-1/2 z-50 -translate-x-1/2'>
          <div className='bg-background flex items-center gap-2 rounded-lg border px-4 py-2 shadow-lg'>
            <Button variant='ghost' size='icon' className='h-8 w-8' onClick={() => setRowSelection({})}>
              <IconX className='h-4 w-4' />
            </Button>
            <div className='flex items-center gap-1.5 px-2'>
              <span className='bg-primary text-primary-foreground flex h-6 min-w-6 items-center justify-center rounded px-1.5 text-xs font-medium'>
                {selectedCount}
              </span>
              <span className='text-muted-foreground text-sm'>{t('common.selected')}</span>
            </div>
            <div className='bg-border mx-2 h-6 w-px' />
            <Button
              variant='ghost'
              size='icon'
              className='text-destructive h-8 w-8 hover:bg-red-100 hover:text-red-700'
              onClick={() => openDialog('bulkDisable')}
              title={t('common.buttons.disable')}
            >
              <IconUserOff className='h-4 w-4' />
            </Button>
            <Button
              variant='ghost'
              size='icon'
              className='h-8 w-8 text-green-600 hover:bg-green-100 hover:text-green-700'
              onClick={() => openDialog('bulkEnable')}
              title={t('common.buttons.enable')}
            >
              <IconCheck className='h-4 w-4' />
            </Button>
            <Button
              variant='ghost'
              size='icon'
              className='h-8 w-8 text-orange-600 hover:bg-orange-100 hover:text-orange-700'
              onClick={() => openDialog('bulkArchive')}
              title={t('common.buttons.archive')}
            >
              <IconArchive className='h-4 w-4' />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
