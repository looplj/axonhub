import { useState } from 'react'
import { DateRange } from 'react-day-picker'
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
} from '@tanstack/react-table'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Trace, TraceConnection } from '../data/schema'
import { DataTableToolbar } from './data-table-toolbar'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { useTracesColumns } from './traces-columns'
import { useTranslation } from 'react-i18next'

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string
  }
}

interface TracesTableProps {
  data: Trace[]
  loading?: boolean
  pageInfo?: TraceConnection['pageInfo']
  pageSize: number
  totalCount?: number
  dateRange?: DateRange
  traceIdFilter: string
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onDateRangeChange: (range: DateRange | undefined) => void
  onTraceIdFilterChange: (traceId: string) => void
  onRefresh: () => void
  showRefresh: boolean
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
}: TracesTableProps) {
  const { t } = useTranslation()
  const tracesColumns = useTracesColumns()
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [rowSelection, setRowSelection] = useState({})

  const table = useReactTable({
    data: data,
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
  })

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
      />
      <div className='mt-4 flex-1 overflow-auto rounded-md border'>
        <Table>
          <TableHeader className='sticky top-0 z-10 bg-background'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row'>
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead
                      key={header.id}
                      colSpan={header.colSpan}
                      className={header.column.columnDef.meta?.className ?? ''}
                    >
                      {header.isPlaceholder
                        ? null
                        : flexRender(
                            header.column.columnDef.header,
                            header.getContext()
                          )}
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell
                  colSpan={tracesColumns.length}
                  className='h-24 text-center'
                >
                  {t('common.loading')}
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && 'selected'}
                  className='group/row'
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      key={cell.id}
                      className={cell.column.columnDef.meta?.className ?? ''}
                    >
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={tracesColumns.length}
                  className='h-24 text-center'
                >
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
  )
}
