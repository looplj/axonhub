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
import { useTranslation } from 'react-i18next'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { ApiKey, ApiKeyConnection } from '../data/schema'
import { DataTableToolbar } from './data-table-toolbar'

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string
  }
}

interface DataTableProps {
  columns: ColumnDef<ApiKey>[]
  data: ApiKey[]
  pageInfo?: ApiKeyConnection['pageInfo']
  pageSize: number
  totalCount?: number
  nameFilter: string
  userFilter: string
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onNameFilterChange: (filter: string) => void
  onUserFilterChange: (filter: string) => void
}

export function ApiKeysTable({ 
  columns, 
  data, 
  pageInfo,
  pageSize,
  totalCount,
  nameFilter,
  userFilter,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onNameFilterChange,
  onUserFilterChange
}: DataTableProps) {
  const { t } = useTranslation()
  const [rowSelection, setRowSelection] = useState({})
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [sorting, setSorting] = useState<SortingState>([])

  // Sync filters with the server state
  const handleColumnFiltersChange = (updater: any) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater
    setColumnFilters(newFilters)
    
    // Find the name filter and sync it with the server
    const nameFilterValue = newFilters.find((filter: any) => filter.id === 'name')?.value || ''
    if (nameFilterValue !== nameFilter) {
      onNameFilterChange(nameFilterValue)
    }
    
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
  if (nameFilter) {
    initialColumnFilters.push({ id: 'name', value: nameFilter })
  }
  if (userFilter) {
    initialColumnFilters.push({ id: 'user', value: [userFilter] })
  }

  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters: columnFilters.length === 0 && (nameFilter || userFilter) ? initialColumnFilters : columnFilters,
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
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead
                      key={header.id}
                      colSpan={header.colSpan}
                      className={header.column.columnDef.meta?.className}
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
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && 'selected'}
                  className='group/row'
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      key={cell.id}
                      className={cell.column.columnDef.meta?.className}
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
                  colSpan={columns.length}
                  className='h-24 text-center'
                >
                  {t('apikeys.noData')}
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
        selectedRows={Object.keys(rowSelection).length}
        onNextPage={onNextPage}
        onPreviousPage={onPreviousPage}
        onPageSizeChange={onPageSizeChange}
      />
    </div>
  )
}