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
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onUserFilterChange: (filter: string) => void
}

export function RequestsTable({
  data,
  loading,
  pageInfo,
  totalCount,
  pageSize,
  userFilter,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onUserFilterChange,
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
    
    // Find the user filter and sync it with the server
    const userFilterValue = newFilters.find((filter: any) => filter.id === 'user')?.value
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
  }

  // Initialize filters in column filters if they exist
  const initialColumnFilters = []
  if (userFilter) {
    initialColumnFilters.push({ id: 'user', value: [userFilter] })
  }

  const table = useReactTable({
    data,
    columns: requestsColumns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters: columnFilters.length === 0 && userFilter ? initialColumnFilters : columnFilters,
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