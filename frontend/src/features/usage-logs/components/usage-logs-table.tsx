import { useState } from 'react'
import { DateRange } from 'react-day-picker'
import {
  ColumnDef,
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
import { UsageLog, UsageLogConnection } from '../data/schema'
import { DataTableToolbar } from './data-table-toolbar'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { useUsageLogsColumns } from './usage-logs-columns'
import { useTranslation } from 'react-i18next'
import { Spinner } from '@/components/spinner'

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string
  }
}

interface UsageLogsTableProps {
  data: UsageLog[]
  loading?: boolean
  pageInfo?: UsageLogConnection['pageInfo']
  pageSize: number
  totalCount?: number
  sourceFilter: string[]
  channelFilter: string[]
  dateRange?: DateRange
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onSourceFilterChange: (filters: string[]) => void
  onChannelFilterChange: (filters: string[]) => void
  onDateRangeChange: (range: DateRange | undefined) => void
  onRefresh: () => void
  showRefresh: boolean
}

export function UsageLogsTable({
  data,
  loading,
  pageInfo,
  totalCount,
  pageSize,
  sourceFilter,
  channelFilter,
  dateRange,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onSourceFilterChange,
  onChannelFilterChange,
  onDateRangeChange,
  onRefresh,
  showRefresh,
}: UsageLogsTableProps) {
  const { t } = useTranslation()
  const usageLogsColumns = useUsageLogsColumns()
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [rowSelection, setRowSelection] = useState({})

  // Sync filters with the server state
  const handleColumnFiltersChange = (updater: any) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater
    setColumnFilters(newFilters)
    
    // Find and sync filters with the server
    const sourceFilterValue = newFilters.find((filter: any) => filter.id === 'source')?.value
    const channelFilterValue = newFilters.find((filter: any) => filter.id === 'channel')?.value
    
    // Handle source filter
    if (Array.isArray(sourceFilterValue)) {
      onSourceFilterChange(sourceFilterValue)
    } else {
      onSourceFilterChange(sourceFilterValue ? [sourceFilterValue] : [])
    }
    
    // Handle channel filter
    if (Array.isArray(channelFilterValue)) {
      onChannelFilterChange(channelFilterValue)
    } else {
      onChannelFilterChange(channelFilterValue ? [channelFilterValue] : [])
    }
  }

  const table = useReactTable({
    data,
    columns: usageLogsColumns,
    onSortingChange: setSorting,
    onColumnFiltersChange: handleColumnFiltersChange,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    onColumnVisibilityChange: setColumnVisibility,
    onRowSelectionChange: setRowSelection,
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    state: {
      sorting,
      columnFilters,
      columnVisibility,
      rowSelection,
    },
  })

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <DataTableToolbar 
        table={table} 
        dateRange={dateRange}
        onDateRangeChange={onDateRangeChange}
        onRefresh={onRefresh} 
        showRefresh={showRefresh} 
      />
      <div className='mt-4 flex-1 overflow-auto rounded-2xl shadow-soft border border-[var(--table-border)] relative'>
        <Table data-testid='usage-logs-table' className='bg-background rounded-2xl border-separate border-spacing-0'>
          <TableHeader className='sticky top-0 z-20 bg-[var(--table-header)] shadow-sm'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row border-0'>
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead
                      key={header.id}
                      className={`${header.column.columnDef.meta?.className ?? ''} text-xs font-semibold text-muted-foreground uppercase tracking-wider border-0`}
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
          <TableBody className='p-2 space-y-1 !bg-background'>
            {loading ? (
              <TableRow className='border-0 !bg-background'>
                <TableCell
                  colSpan={usageLogsColumns.length}
                  className='h-24 text-center border-0 !bg-background'
                >
                  <div className='flex items-center justify-center'>
                    <Spinner />
                  </div>
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && 'selected'}
                  className='group/row table-row-hover rounded-xl !bg-background border-0 transition-all duration-200 ease-in-out'
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      key={cell.id}
                      className={`${cell.column.columnDef.meta?.className ?? ''} px-4 py-3 border-0 !bg-background`}
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
              <TableRow className='!bg-background'>
                <TableCell
                  colSpan={usageLogsColumns.length}
                  className='h-24 text-center !bg-background'
                >
                  {t('common.noResults')}
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