import React, { useState, useEffect, useMemo } from 'react';
import {
  ColumnDef,
  ColumnFiltersState,
  RowSelectionState,
  SortingState,
  VisibilityState,
  flexRender,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table';
import { IconBan, IconCheck, IconX, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { TableSkeleton } from '@/components/ui/table-skeleton';
import { PermissionGuard } from '@/components/permission-guard';
import { ServerSidePagination } from '@/components/server-side-pagination';
import { usePrompts } from '../context/prompts-context';
import { Prompt, PromptConnection } from '../data/schema';

interface PromptsTableProps {
  columns: ColumnDef<Prompt>[];
  data: Prompt[];
  loading?: boolean;
  pageInfo?: PromptConnection['pageInfo'];
  pageSize: number;
  totalCount?: number;
  nameFilter: string;
  sorting: SortingState;
  onSortingChange: (updater: SortingState | ((prev: SortingState) => SortingState)) => void;
  onNextPage: () => void;
  onPreviousPage: () => void;
  onPageSizeChange: (pageSize: number) => void;
  onNameFilterChange: (filter: string) => void;
  canWrite?: boolean;
}

export function PromptsTable({
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
  canWrite = true,
}: PromptsTableProps) {
  const { t } = useTranslation();
  const { setSelectedPrompts, setResetRowSelection, setOpen } = usePrompts();
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
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    manualPagination: true,
    manualFiltering: true,
  });

  const filteredSelectedRows = useMemo(() => table.getFilteredSelectedRowModel().rows, [table, rowSelection, data]);

  const selectedCount = filteredSelectedRows.length;

  useEffect(() => {
    const resetFn = () => {
      setRowSelection({});
    };
    setResetRowSelection(resetFn);
  }, [setResetRowSelection]);

  useEffect(() => {
    const selected = filteredSelectedRows.map((row) => row.original as Prompt);
    setSelectedPrompts(selected);
  }, [filteredSelectedRows, setSelectedPrompts]);

  useEffect(() => {
    if (selectedCount === 0) {
      setSelectedPrompts([]);
    }
  }, [selectedCount, setSelectedPrompts]);

  useEffect(() => {
    if (Object.keys(rowSelection).length > 0 && data.length > 0) {
      const dataIds = new Set(data.map((prompt) => prompt.id));
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
            placeholder={t('prompts.filters.filterByName')}
            value={(table.getColumn('name')?.getFilterValue() as string) ?? ''}
            onChange={(event) => table.getColumn('name')?.setFilterValue(event.target.value)}
            className='h-8 w-[150px] lg:w-[250px]'
          />
        </div>
      </div>

      <div className='shadow-soft relative mt-4 flex-1 overflow-auto overflow-x-hidden rounded-2xl border border-[var(--table-border)]'>
        <Table data-testid='prompts-table' className='border-separate border-spacing-0 rounded-2xl bg-[var(--table-background)]'>
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

      {selectedCount > 0 && canWrite && (
        <div className='fixed bottom-6 left-1/2 z-50 -translate-x-1/2'>
          <div className='flex items-center gap-2 rounded-lg border bg-[var(--table-background)] px-4 py-2 shadow-lg'>
            <div className='bg-border mx-2 h-6 w-px' />
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
            <PermissionGuard requiredScope='write_prompts'>
              <Button
                variant='ghost'
                size='icon'
                className='h-8 w-8 text-green-600 hover:bg-green-100 hover:text-green-700'
                onClick={() => setOpen('bulkEnable')}
                title={t('common.buttons.enable')}
              >
                <IconCheck className='h-4 w-4' />
              </Button>
            </PermissionGuard>
            <PermissionGuard requiredScope='write_prompts'>
              <Button
                variant='ghost'
                size='icon'
                className='h-8 w-8 text-amber-600 hover:bg-amber-100 hover:text-amber-700'
                onClick={() => setOpen('bulkDisable')}
                title={t('common.buttons.disable')}
              >
                <IconBan className='h-4 w-4' />
              </Button>
            </PermissionGuard>
            <PermissionGuard requiredScope='write_prompts'>
              <Button
                variant='ghost'
                size='icon'
                className='text-destructive h-8 w-8 hover:bg-red-100 hover:text-red-700'
                onClick={() => setOpen('bulkDelete')}
                title={t('common.buttons.delete')}
              >
                <IconTrash className='h-4 w-4' />
              </Button>
            </PermissionGuard>
          </div>
        </div>
      )}
    </div>
  );
}
