import { useState, useMemo } from 'react'
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
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onStatusFilterChange: (filters: string[]) => void
  onSourceFilterChange: (filters: string[]) => void
  onChannelFilterChange: (filters: string[]) => void
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
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onStatusFilterChange,
  onSourceFilterChange,
  onChannelFilterChange,
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
    
    // Find and sync filters with the server
    const statusFilterValue = newFilters.find((filter: any) => filter.id === 'status')?.value
    const sourceFilterValue = newFilters.find((filter: any) => filter.id === 'source')?.value
    const channelFilterValue = newFilters.find((filter: any) => filter.id === 'channel')?.value
    
    // Handle status filter
    const statusFilterArray = Array.isArray(statusFilterValue) ? statusFilterValue : []
    if (JSON.stringify(statusFilterArray.sort()) !== JSON.stringify(statusFilter.sort())) {
      onStatusFilterChange(statusFilterArray)
    }
    
    // Handle source filter
    const sourceFilterArray = Array.isArray(sourceFilterValue) ? sourceFilterValue : []
    if (JSON.stringify(sourceFilterArray.sort()) !== JSON.stringify(sourceFilter.sort())) {
      onSourceFilterChange(sourceFilterArray)
    }
    
    // Handle channel filter
    const channelFilterArray = Array.isArray(channelFilterValue) ? channelFilterValue : []
    if (JSON.stringify(channelFilterArray.sort()) !== JSON.stringify(channelFilter.sort())) {
      onChannelFilterChange(channelFilterArray)
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


  const table = useReactTable({
    data: data,
    columns: requestsColumns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters: columnFilters.length === 0 && (statusFilter.length > 0 || sourceFilter.length > 0 || channelFilter.length > 0) ? initialColumnFilters : columnFilters,
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
    <div className='space-y-4'>
      <DataTableToolbar
        table={table}
        onRefresh={onRefresh}
        showRefresh={showRefresh}
      />
      <div className='rounded-md border'>
        <Table>
          <TableHeader>
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
                  colSpan={requestsColumns.length}
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
                  colSpan={requestsColumns.length}
                  className='h-24 text-center'
                >
                  {t('common.noData')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
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
  )
}