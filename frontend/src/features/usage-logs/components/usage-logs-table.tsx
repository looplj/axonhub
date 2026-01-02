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
import { motion, AnimatePresence } from 'framer-motion'
import { useAnimatedList } from '@/hooks/useAnimatedList'

const MotionTableRow = motion(TableRow)

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
  autoRefresh?: boolean
  onAutoRefreshChange?: (enabled: boolean) => void
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
  autoRefresh = false,
  onAutoRefreshChange,
}: UsageLogsTableProps) {
  const { t } = useTranslation()
  const usageLogsColumns = useUsageLogsColumns()
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [rowSelection, setRowSelection] = useState({})

  const displayedData = useAnimatedList(data, autoRefresh)

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

  // Initialize filters in column filters if they exist
  const initialColumnFilters = []
  if (sourceFilter.length > 0) {
    initialColumnFilters.push({ id: 'source', value: sourceFilter })
  }
  if (channelFilter.length > 0) {
    initialColumnFilters.push({ id: 'channel', value: channelFilter })
  }

  const table = useReactTable({
    data: displayedData,
    getRowId: (row) => row.id,
    columns: usageLogsColumns,
    state: {
      sorting,
      columnFilters: columnFilters.length === 0 && (sourceFilter.length > 0 || channelFilter.length > 0) ? initialColumnFilters : columnFilters,
      columnVisibility,
      rowSelection,
    },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnFiltersChange: handleColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    // Disable client-side pagination since we're using server-side
    manualPagination: true,
    manualFiltering: true,
  })

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <DataTableToolbar
        table={table}
        dateRange={dateRange}
        onDateRangeChange={onDateRangeChange}
        onRefresh={onRefresh}
        showRefresh={showRefresh}
        autoRefresh={autoRefresh}
        onAutoRefreshChange={onAutoRefreshChange}
      />
      <div className="mt-4 flex-1 overflow-auto rounded-2xl shadow-soft border border-[var(--table-border)] relative">
        <Table data-testid="usage-logs-table" className="bg-[var(--table-background)] rounded-2xl border-separate border-spacing-0">
          <TableHeader className="sticky top-0 z-20 bg-[var(--table-header)] shadow-sm">
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="group/row border-0">
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead
                      key={header.id}
                      colSpan={header.colSpan}
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
          <TableBody className='p-2 space-y-1 !bg-[var(--table-background)]'>
            {loading ? (
              <TableRow className='border-0 !bg-[var(--table-background)]'>
                <TableCell
                  colSpan={usageLogsColumns.length}
                  className='h-24 text-center border-0 !bg-[var(--table-background)]'
                >
                  {t('common.loading')}
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows?.length ? (
              <AnimatePresence initial={false} mode="popLayout">
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
                      <TableCell key={cell.id} className={`${cell.column.columnDef.meta?.className ?? ''} py-3 border-b border-[var(--table-border)] group-last/row:border-0`}>
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext()
                        )}
                      </TableCell>
                    ))}
                  </MotionTableRow>
                ))}
              </AnimatePresence>
            ) : (
              <TableRow className="!bg-[var(--table-background)]">
                <TableCell
                  colSpan={usageLogsColumns.length}
                  className="h-24 text-center !bg-[var(--table-background)]"
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