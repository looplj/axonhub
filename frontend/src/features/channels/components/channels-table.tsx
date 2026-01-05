import React, { useState, useEffect, useMemo, useCallback } from 'react';
import {
  ColumnDef,
  ColumnFiltersState,
  ExpandedState,
  RowData,
  RowSelectionState,
  SortingState,
  VisibilityState,
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table';
import { IconArchive, IconBan, IconCheck, IconTrash, IconTemplate, IconX } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { TableSkeleton } from '@/components/ui/table-skeleton';
import { ServerSidePagination } from '@/components/server-side-pagination';
import { ChannelExpandedRow } from './channel-expanded-row';
import { useChannels } from '../context/channels-context';
import { Channel, ChannelConnection } from '../data/schema';
import { DataTableToolbar } from './data-table-toolbar';

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string;
  }
}

interface DataTableProps {
  columns: ColumnDef<Channel>[];
  loading?: boolean;
  data: Channel[];
  pageInfo?: ChannelConnection['pageInfo'];
  pageSize: number;
  totalCount?: number;
  nameFilter: string;
  typeFilter: string[];
  statusFilter: string[];
  tagFilter: string;
  modelFilter: string;
  selectedTypeTab?: string;
  showErrorOnly?: boolean;
  onExitErrorOnlyMode?: () => void;
  sorting: SortingState;
  onSortingChange: (updater: SortingState | ((prev: SortingState) => SortingState)) => void;
  onNextPage: () => void;
  onPreviousPage: () => void;
  onPageSizeChange: (pageSize: number) => void;
  onResetCursor?: () => void;
  onNameFilterChange: (filter: string) => void;
  onTypeFilterChange: (filters: string[]) => void;
  onStatusFilterChange: (filters: string[]) => void;
  onTagFilterChange: (filter: string) => void;
  onModelFilterChange: (filter: string) => void;
  canWrite?: boolean;
}

export function ChannelsTable({
  columns,
  loading,
  data,
  pageInfo,
  pageSize,
  totalCount,
  nameFilter,
  typeFilter,
  statusFilter,
  tagFilter,
  modelFilter,
  selectedTypeTab = 'all',
  showErrorOnly,
  sorting,
  onSortingChange,
  onExitErrorOnlyMode,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onResetCursor,
  onNameFilterChange,
  onTypeFilterChange,
  onStatusFilterChange,
  onTagFilterChange,
  onModelFilterChange,
  canWrite = true,
}: DataTableProps) {
  const { t } = useTranslation();
  const { setSelectedChannels, setResetRowSelection, setOpen } = useChannels();
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [expanded, setExpanded] = useState<ExpandedState>({});
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);

  // Load column visibility from localStorage with useMemo to avoid re-parsing
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>(() => {
    const stored = localStorage.getItem('channels-table-column-visibility');
    if (stored) {
      try {
        return JSON.parse(stored);
      } catch {
        return { tags: false };
      }
    }
    return { tags: false }; // Hide tags column by default but keep it for filtering
  });

  // Sync server state to local column filters using useMemo instead of useEffect
  useEffect(() => {
    const newColumnFilters: ColumnFiltersState = [];

    if (nameFilter) {
      newColumnFilters.push({ id: 'name', value: nameFilter });
    }
    if (typeFilter.length > 0) {
      newColumnFilters.push({ id: 'provider', value: typeFilter });
    }
    if (statusFilter.length > 0) {
      newColumnFilters.push({ id: 'status', value: statusFilter });
    }
    if (tagFilter) {
      newColumnFilters.push({ id: 'tags', value: tagFilter });
    }
    if (modelFilter) {
      newColumnFilters.push({ id: 'model', value: modelFilter });
    }

    setColumnFilters(newColumnFilters);
  }, [nameFilter, typeFilter, statusFilter, tagFilter, modelFilter]);

  // Save column visibility to localStorage whenever it changes
  useEffect(() => {
    localStorage.setItem('channels-table-column-visibility', JSON.stringify(columnVisibility));
  }, [columnVisibility]);

  // Handle column filter changes and sync with server
  const handleColumnFiltersChange = useCallback(
    (updater: ColumnFiltersState | ((prev: ColumnFiltersState) => ColumnFiltersState)) => {
      const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater;
      setColumnFilters(newFilters);

      // Extract filter values
      const nameFilterValue = newFilters.find((filter) => filter.id === 'name')?.value as string;
      const typeFilterValue = newFilters.find((filter) => filter.id === 'provider')?.value as string[];
      const statusFilterValue = newFilters.find((filter) => filter.id === 'status')?.value as string[];
      const tagFilterValue = newFilters.find((filter) => filter.id === 'tags')?.value as string;
      const modelFilterValue = newFilters.find((filter) => filter.id === 'model')?.value as string;

      // Update server filters only if changed
      const newNameFilter = nameFilterValue || '';
      const newTypeFilter = Array.isArray(typeFilterValue) ? typeFilterValue : [];
      const newStatusFilter = Array.isArray(statusFilterValue) ? statusFilterValue : [];
      const newTagFilter = tagFilterValue || '';
      const newModelFilter = modelFilterValue || '';

      if (newNameFilter !== nameFilter) {
        onNameFilterChange(newNameFilter);
      }

      if (JSON.stringify(newTypeFilter.sort()) !== JSON.stringify(typeFilter.sort())) {
        onTypeFilterChange(newTypeFilter);
      }

      if (JSON.stringify(newStatusFilter.sort()) !== JSON.stringify(statusFilter.sort())) {
        onStatusFilterChange(newStatusFilter);
      }

      if (newTagFilter !== tagFilter) {
        onTagFilterChange(newTagFilter);
      }

      if (newModelFilter !== modelFilter) {
        onModelFilterChange(newModelFilter);
      }
    },
    [
      columnFilters,
      nameFilter,
      typeFilter,
      statusFilter,
      tagFilter,
      modelFilter,
      onNameFilterChange,
      onTypeFilterChange,
      onStatusFilterChange,
      onTagFilterChange,
      onModelFilterChange,
    ]
  );

  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
      expanded,
    },
    enableRowSelection: true,
    getRowId: (row) => row.id,
    onRowSelectionChange: setRowSelection,
    onExpandedChange: setExpanded,
    onSortingChange,
    onColumnFiltersChange: handleColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getExpandedRowModel: getExpandedRowModel(),
    // Enable server-side pagination and filtering
    manualPagination: true,
    manualFiltering: true, // Enable manual filtering for server-side filtering
  });

  const filteredSelectedRows = useMemo(
    () => table.getFilteredSelectedRowModel().rows,
    [table.getState().rowSelection, table.getFilteredRowModel().rows]
  );

  const getApiFormatLabel = useCallback(
    (apiFormat?: string) => {
      if (!apiFormat) return '-';

      const key = `channels.dialogs.fields.apiFormat.formats.${apiFormat}`;
      const label = t(key);
      return label === key ? apiFormat : label;
    },
    [t]
  );
  
  const selectedCount = useMemo(() => filteredSelectedRows.length, [filteredSelectedRows]);
  const isFiltered = useMemo(() => columnFilters.length > 0, [columnFilters.length]);

  useEffect(() => {
    const resetFn = () => {
      setRowSelection({});
    };
    setResetRowSelection(resetFn);
  }, [setResetRowSelection]);

  // Combine two useEffects into one to reduce re-renders
  useEffect(() => {
    if (selectedCount === 0) {
      setSelectedChannels([]);
    } else {
      const selected = filteredSelectedRows.map((row) => row.original as Channel);
      setSelectedChannels(selected);
    }
  }, [filteredSelectedRows, selectedCount, setSelectedChannels]);

  // Clear rowSelection when data changes and selected rows no longer exist
  useEffect(() => {
    if (Object.keys(rowSelection).length > 0 && data.length > 0) {
      const dataIds = new Set(data.map((channel) => channel.id));
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
      <DataTableToolbar
        table={table}
        isFiltered={isFiltered}
        selectedCount={selectedCount}
        selectedTypeTab={selectedTypeTab}
        showErrorOnly={showErrorOnly}
        onExitErrorOnlyMode={onExitErrorOnlyMode}
      />
      <div className='shadow-soft relative mt-4 flex-1 overflow-auto overflow-x-hidden rounded-2xl border border-[var(--table-border)]'>
        <Table data-testid='channels-table' className='border-separate border-spacing-0 rounded-2xl bg-[var(--table-background)]'>
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
              table.getRowModel().rows.map((row) => {
                const channel = row.original;
                return (
                  <React.Fragment key={row.id}>
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
                    {row.getIsExpanded() && (
                      <ChannelExpandedRow channel={channel} columnsLength={columns.length} getApiFormatLabel={getApiFormatLabel} />
                    )}
                  </React.Fragment>
                );
              })
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
          onResetCursor={onResetCursor}
        />
      </div>
      {/* Floating Bulk Actions Bar */}
      {selectedCount > 0 && canWrite && (
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
              className='h-8 w-8 text-blue-600 hover:bg-blue-100 hover:text-blue-700'
              onClick={() => setOpen('bulkApplyTemplate')}
              title={t('channels.templates.bulk.applyButton')}
            >
              <IconTemplate className='h-4 w-4' />
            </Button>
            <Button
              variant='ghost'
              size='icon'
              className='h-8 w-8 text-green-600 hover:bg-green-100 hover:text-green-700'
              onClick={() => setOpen('bulkEnable')}
              title={t('common.buttons.enable')}
            >
              <IconCheck className='h-4 w-4' />
            </Button>
            <Button
              variant='ghost'
              size='icon'
              className='h-8 w-8 text-amber-600 hover:bg-amber-100 hover:text-amber-700'
              onClick={() => setOpen('bulkDisable')}
              title={t('common.buttons.disable')}
            >
              <IconBan className='h-4 w-4' />
            </Button>
            <Button
              variant='ghost'
              size='icon'
              className='h-8 w-8 text-orange-600 hover:bg-orange-100 hover:text-orange-700'
              onClick={() => setOpen('bulkArchive')}
              title={t('common.buttons.archive')}
            >
              <IconArchive className='h-4 w-4' />
            </Button>
            <Button
              variant='ghost'
              size='icon'
              className='text-destructive h-8 w-8 hover:bg-red-100 hover:text-red-700'
              onClick={() => setOpen('bulkDelete')}
              title={t('common.buttons.delete')}
            >
              <IconTrash className='h-4 w-4' />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
