import { useState } from 'react'
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
import { Request, RequestConnection } from '../data/schema'
import { DataTableToolbar } from './data-table-toolbar'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { useRequestsColumns } from './requests-columns'
import { useTranslation } from 'react-i18next'
import { Spinner } from '@/components/spinner'

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
  userFilter: string
  statusFilter: string[]
  sourceFilter: string[]
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onUserFilterChange: (filter: string) => void
  onStatusFilterChange: (filters: string[]) => void
  onSourceFilterChange: (filters: string[]) => void
}

export function RequestsTable({
  data,
  loading,
  pageInfo,
  totalCount,
  pageSize,
  userFilter,
  statusFilter,
  sourceFilter,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onUserFilterChange,
  onStatusFilterChange,
  onSourceFilterChange,
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
    const userFilterValue = newFilters.find((filter: any) => filter.id === 'user')?.value
    const statusFilterValue = newFilters.find((filter: any) => filter.id === 'status')?.value
    const sourceFilterValue = newFilters.find((filter: any) => filter.id === 'source')?.value
    
    // Handle user filter
    let userFilterString = ''
    if (userFilterValue) {
      if (Array.isArray(userFilterValue)) {
        userFilterString = userFilterValue.length > 0 ? userFilterValue[0] : ''
      } else {
        userFilterString = userFilterValue
      }
    }
    
    if (userFilterString !== userFilter) {
      onUserFilterChange(userFilterString)
    }
    
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
  }

  // Initialize filters in column filters if they exist
  const initialColumnFilters = []
  if (userFilter) {
    initialColumnFilters.push({ id: 'user', value: [userFilter] })
  }
  if (statusFilter.length > 0) {
    initialColumnFilters.push({ id: 'status', value: statusFilter })
  }
  if (sourceFilter.length > 0) {
    initialColumnFilters.push({ id: 'source', value: sourceFilter })
  }

  const table = useReactTable({
    data,
    columns: requestsColumns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters: columnFilters.length === 0 && (userFilter || statusFilter.length > 0 || sourceFilter.length > 0) ? initialColumnFilters : columnFilters,
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
      <DataTableToolbar table={table} />
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
                  {t('loading')}
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
                  {t('noData')}
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