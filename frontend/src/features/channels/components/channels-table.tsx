import { useState, useEffect } from 'react'
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
import { Channel, ChannelConnection } from '../data/schema'
import { DataTableToolbar } from './data-table-toolbar'

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string
  }
}

interface DataTableProps {
  columns: ColumnDef<Channel>[]
  loading?: boolean
  data: Channel[]
  pageInfo?: ChannelConnection['pageInfo']
  pageSize: number
  totalCount?: number
  nameFilter: string
  typeFilter: string[]
  statusFilter: string[]
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onNameFilterChange: (filter: string) => void
  onTypeFilterChange: (filters: string[]) => void
  onStatusFilterChange: (filters: string[]) => void
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
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onNameFilterChange,
  onTypeFilterChange,
  onStatusFilterChange,
}: DataTableProps) {
  const { t } = useTranslation()
  const [rowSelection, setRowSelection] = useState({})
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [sorting, setSorting] = useState<SortingState>([])

  // Sync server state to local column filters using useEffect
  useEffect(() => {
    const newColumnFilters: ColumnFiltersState = []

    if (nameFilter) {
      newColumnFilters.push({ id: 'name', value: nameFilter })
    }
    if (typeFilter.length > 0) {
      newColumnFilters.push({ id: 'type', value: typeFilter })
    }
    if (statusFilter.length > 0) {
      newColumnFilters.push({ id: 'status', value: statusFilter })
    }

    setColumnFilters(newColumnFilters)
  }, [nameFilter, typeFilter, statusFilter])

  // Handle column filter changes and sync with server
  const handleColumnFiltersChange = (
    updater:
      | ColumnFiltersState
      | ((prev: ColumnFiltersState) => ColumnFiltersState)
  ) => {
    const newFilters =
      typeof updater === 'function' ? updater(columnFilters) : updater
    setColumnFilters(newFilters)

    // Extract filter values
    const nameFilterValue = newFilters.find((filter) => filter.id === 'name')
      ?.value as string
    const typeFilterValue = newFilters.find((filter) => filter.id === 'type')
      ?.value as string[]
    const statusFilterValue = newFilters.find(
      (filter) => filter.id === 'status'
    )?.value as string[]

    // Update server filters only if changed
    const newNameFilter = nameFilterValue || ''
    const newTypeFilter = Array.isArray(typeFilterValue) ? typeFilterValue : []
    const newStatusFilter = Array.isArray(statusFilterValue)
      ? statusFilterValue
      : []

    if (newNameFilter !== nameFilter) {
      onNameFilterChange(newNameFilter)
    }

    if (
      JSON.stringify(newTypeFilter.sort()) !== JSON.stringify(typeFilter.sort())
    ) {
      onTypeFilterChange(newTypeFilter)
    }

    if (
      JSON.stringify(newStatusFilter.sort()) !==
      JSON.stringify(statusFilter.sort())
    ) {
      onStatusFilterChange(newStatusFilter)
    }
  }

  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
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
    // Enable server-side pagination and filtering
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
                  colSpan={columns.length}
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
                  colSpan={columns.length}
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
        selectedRows={Object.keys(rowSelection).length}
        onNextPage={onNextPage}
        onPreviousPage={onPreviousPage}
        onPageSizeChange={onPageSizeChange}
      />
    </div>
  )
}
