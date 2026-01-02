import { useState } from 'react';
import {
  ColumnFiltersState,
  RowData,
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
import { motion, AnimatePresence } from 'framer-motion';
import { DateRange } from 'react-day-picker';
import { useTranslation } from 'react-i18next';
import { useAnimatedList } from '@/hooks/useAnimatedList';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { ServerSidePagination } from '@/components/server-side-pagination';
import { Trace, TraceConnection } from '../data/schema';
import { DataTableToolbar } from './data-table-toolbar';
import { useTracesColumns } from './traces-columns';

const MotionTableRow = motion(TableRow);

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string;
  }
}

interface TracesTableProps {
  data: Trace[];
  loading?: boolean;
  pageInfo?: TraceConnection['pageInfo'];
  pageSize: number;
  totalCount?: number;
  dateRange?: DateRange;
  traceIdFilter: string;
  onNextPage: () => void;
  onPreviousPage: () => void;
  onPageSizeChange: (pageSize: number) => void;
  onDateRangeChange: (range: DateRange | undefined) => void;
  onTraceIdFilterChange: (traceId: string) => void;
  onRefresh: () => void;
  showRefresh: boolean;
  autoRefresh?: boolean;
  onAutoRefreshChange?: (enabled: boolean) => void;
}

export function TracesTable({
  data,
  loading,
  pageInfo,
  totalCount,
  pageSize,
  dateRange,
  traceIdFilter,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onDateRangeChange,
  onTraceIdFilterChange,
  onRefresh,
  showRefresh,
  autoRefresh = false,
  onAutoRefreshChange,
}: TracesTableProps) {
  const { t } = useTranslation();
  const tracesColumns = useTracesColumns();
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({});
  const [rowSelection, setRowSelection] = useState({});

  const displayedData = useAnimatedList(data, autoRefresh);

  const table = useReactTable({
    data: displayedData,
    getRowId: (row) => row.id,
    columns: tracesColumns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
    },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    manualPagination: true,
    manualFiltering: true,
  });

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <DataTableToolbar
        table={table}
        dateRange={dateRange}
        onDateRangeChange={onDateRangeChange}
        traceIdFilter={traceIdFilter}
        onTraceIdFilterChange={onTraceIdFilterChange}
        onRefresh={onRefresh}
        showRefresh={showRefresh}
        autoRefresh={autoRefresh}
        onAutoRefreshChange={onAutoRefreshChange}
      />
      <div className='shadow-soft relative mt-4 flex-1 overflow-auto rounded-2xl border border-[var(--table-border)]'>
        <Table data-testid='traces-table' className='border-separate border-spacing-0 rounded-2xl bg-[var(--table-background)]'>
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
                <TableCell colSpan={tracesColumns.length} className='h-24 border-0 !bg-[var(--table-background)] text-center'>
                  {t('common.loading')}
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows?.length ? (
              <AnimatePresence initial={false} mode='popLayout'>
                {table.getRowModel().rows.map((row) => (
                  <MotionTableRow
                    key={row.id}
                    data-state={row.getIsSelected() && 'selected'}
                    initial={{ opacity: 0, y: -20, height: 0 }}
                    animate={{ opacity: 1, y: 0, height: 'auto' }}
                    exit={{ opacity: 0, height: 0 }}
                    transition={{
                      type: 'spring',
                      stiffness: 500,
                      damping: 30,
                      mass: 1,
                      opacity: { duration: 0.2 },
                    }}
                    layout
                    className='group/row hover:bg-muted/50 data-[state=selected]:bg-muted'
                  >
                    {row.getVisibleCells().map((cell) => (
                      <TableCell
                        key={cell.id}
                        className={`${cell.column.columnDef.meta?.className ?? ''} border-b border-[var(--table-border)] py-3 group-last/row:border-0`}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </MotionTableRow>
                ))}
              </AnimatePresence>
            ) : (
              <TableRow className='!bg-[var(--table-background)]'>
                <TableCell colSpan={tracesColumns.length} className='h-24 !bg-[var(--table-background)] text-center'>
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
          selectedRows={table.getFilteredSelectedRowModel().rows.length}
          onNextPage={onNextPage}
          onPreviousPage={onPreviousPage}
          onPageSizeChange={onPageSizeChange}
        />
      </div>
    </div>
  );
}
