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
import { Request, RequestConnection } from '../data/schema'
import { DataTableToolbar } from './data-table-toolbar'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { useRequestsColumns } from './requests-columns'
import { useTranslation } from 'react-i18next'

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string
  }
}

interface RequestsTableProps {
  data: Request[]
  loading?: boolean
  pageInfo?: RequestConnection['pageInfo']
  pageSize: number
  totalCount?: number
  statusFilter: string[]
  sourceFilter: string[]
  channelFilter: string[]
  apiKeyFilter: string[]
  dateRange?: DateRange
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onStatusFilterChange: (filters: string[]) => void
  onSourceFilterChange: (filters: string[]) => void
  onChannelFilterChange: (filters: string[]) => void
  onApiKeyFilterChange: (filters: string[]) => void
  onDateRangeChange: (range: DateRange | undefined) => void
  onRefresh: () => void
  showRefresh: boolean
}

export function RequestsTable({
  data,
  loading,
  pageInfo,
  totalCount,
  pageSize,
  statusFilter,
  sourceFilter,
  channelFilter,
  apiKeyFilter,
  dateRange,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onStatusFilterChange,
  onSourceFilterChange,
  onChannelFilterChange,
  onApiKeyFilterChange,
  onDateRangeChange,
  onRefresh,
  showRefresh,
}: RequestsTableProps) {
  const { t } = useTranslation()
  const requestsColumns = useRequestsColumns()
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [rowSelection, setRowSelection] = useState({})

  // Sync filters with the server state
  const handleColumnFiltersChange = (updater: any) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater
    setColumnFilters(newFilters)
    
    const statusFilterValue = newFilters.find((filter: any) => filter.id === 'status')?.value
    const sourceFilterValue = newFilters.find((filter: any) => filter.id === 'source')?.value
    const channelFilterValue = newFilters.find((filter: any) => filter.id === 'channel')?.value
    const apiKeyFilterValue = newFilters.find((filter: any) => filter.id === 'apiKey')?.value
    
    const statusFilterArray = Array.isArray(statusFilterValue) ? statusFilterValue : []
    if (JSON.stringify(statusFilterArray.sort()) !== JSON.stringify(statusFilter.sort())) {
      onStatusFilterChange(statusFilterArray)
    }
    
    const sourceFilterArray = Array.isArray(sourceFilterValue) ? sourceFilterValue : []
    if (JSON.stringify(sourceFilterArray.sort()) !== JSON.stringify(sourceFilter.sort())) {
      onSourceFilterChange(sourceFilterArray)
    }
    
    const channelFilterArray = Array.isArray(channelFilterValue) ? channelFilterValue : []
    if (JSON.stringify(channelFilterArray.sort()) !== JSON.stringify(channelFilter.sort())) {
      onChannelFilterChange(channelFilterArray)
    }
    
    const apiKeyFilterArray = Array.isArray(apiKeyFilterValue) ? apiKeyFilterValue : []
    if (JSON.stringify(apiKeyFilterArray.sort()) !== JSON.stringify(apiKeyFilter.sort())) {
      onApiKeyFilterChange(apiKeyFilterArray)
    }
  }

  // Initialize filters in column filters if they exist
  const initialColumnFilters = []
  if (statusFilter.length > 0) {
    initialColumnFilters.push({ id: 'status', value: statusFilter })
  }
  if (sourceFilter.length > 0) {
    initialColumnFilters.push({ id: 'source', value: sourceFilter })
  }
  if (channelFilter.length > 0) {
    initialColumnFilters.push({ id: 'channel', value: channelFilter })
  }
  if (apiKeyFilter.length > 0) {
    initialColumnFilters.push({ id: 'apiKey', value: apiKeyFilter })
  }


  const table = useReactTable({
    data: data,
    columns: requestsColumns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters: columnFilters.length === 0 && (statusFilter.length > 0 || sourceFilter.length > 0 || channelFilter.length > 0 || apiKeyFilter.length > 0) ? initialColumnFilters : columnFilters,
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
    manualFiltering: true, // Enable manual filtering for server-side filtering
  })

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <DataTableToolbar 
        table={table} 
        dateRange={dateRange}
        onDateRangeChange={onDateRangeChange}
        onRefresh={onRefresh} 
        showRefresh={showRefresh} 
        apiKeyFilter={apiKeyFilter}
        onApiKeyFilterChange={onApiKeyFilterChange}
      />
      <div className='mt-4 flex-1 overflow-auto rounded-2xl shadow-soft border border-[var(--table-border)] relative'>
        <Table data-testid='requests-table' className='bg-[var(--table-background)] rounded-2xl border-separate border-spacing-0'>
          <TableHeader className='sticky top-0 z-20 bg-[var(--table-header)] shadow-sm'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row border-0'>
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
                  colSpan={requestsColumns.length}
                  className='h-24 text-center border-0 !bg-[var(--table-background)]'
                >
                  {t('common.loading')}
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && 'selected'}
                  className='group/row table-row-hover rounded-xl !bg-[var(--table-background)] border-0 transition-all duration-200 ease-in-out'
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      key={cell.id}
                      className={`${cell.column.columnDef.meta?.className ?? ''} px-4 py-3 border-0 !bg-[var(--table-background)]`}
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
              <TableRow className='!bg-[var(--table-background)]'>
                <TableCell
                  colSpan={requestsColumns.length}
                  className='h-24 text-center !bg-[var(--table-background)]'
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