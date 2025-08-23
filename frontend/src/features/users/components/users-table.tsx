import React, { useState } from 'react'
import { useTranslation } from 'react-i18next'
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
import { ServerSidePagination } from '@/components/server-side-pagination'
import { User, UserConnection } from '../data/schema'
import { DataTableToolbar } from './data-table-toolbar'

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string
  }
}

interface DataTableProps {
  columns: ColumnDef<User>[]
  data: User[]
  pageInfo?: UserConnection['pageInfo']
  pageSize: number
  totalCount?: number
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  nameFilter: string
  statusFilter: string[]
  roleFilter: string[]
  onNameFilterChange: (value: string) => void
  onStatusFilterChange: (value: string[]) => void
  onRoleFilterChange: (value: string[]) => void
}

export function UsersTable({ 
  columns, 
  data, 
  pageInfo,
  pageSize,
  totalCount,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  nameFilter,
  statusFilter,
  roleFilter,
  onNameFilterChange,
  onStatusFilterChange,
  onRoleFilterChange
}: DataTableProps) {
  const { t } = useTranslation()
  const [rowSelection, setRowSelection] = useState({})
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [sorting, setSorting] = useState<SortingState>([])

  // Sync server state to local column filters (for UI display)
  React.useEffect(() => {
    const newFilters: ColumnFiltersState = []
    if (nameFilter) {
      newFilters.push({ id: 'firstName', value: nameFilter })
    }
    if (statusFilter.length > 0) {
      newFilters.push({ id: 'status', value: statusFilter })
    }
    if (roleFilter.length > 0) {
      newFilters.push({ id: 'role', value: roleFilter })
    }
    setColumnFilters(newFilters)
  }, [nameFilter, statusFilter, roleFilter])

  const handleColumnFiltersChange = (updater: ColumnFiltersState | ((prev: ColumnFiltersState) => ColumnFiltersState)) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater
    setColumnFilters(newFilters)
    
    // Extract filter values
    const nameFilterValue = newFilters.find(f => f.id === 'firstName')?.value
    const statusFilterValue = newFilters.find(f => f.id === 'status')?.value
    const roleFilterValue = newFilters.find(f => f.id === 'role')?.value
    
    // Only update if values actually change to prevent reset issues
    const newNameFilter = typeof nameFilterValue === 'string' ? nameFilterValue : ''
    if (newNameFilter !== nameFilter) {
      onNameFilterChange(newNameFilter)
    }
    
    const newStatusFilter = Array.isArray(statusFilterValue) ? statusFilterValue : []
    if (JSON.stringify(newStatusFilter.sort()) !== JSON.stringify(statusFilter.sort())) {
      onStatusFilterChange(newStatusFilter)
    }
    
    const newRoleFilter = Array.isArray(roleFilterValue) ? roleFilterValue : []
    if (JSON.stringify(newRoleFilter.sort()) !== JSON.stringify(roleFilter.sort())) {
      onRoleFilterChange(newRoleFilter)
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
    manualFiltering: true,
    manualPagination: true,
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
  })

  return (
    <div className='space-y-4' data-testid="users-table">
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
                  {t('users.noResults')}
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
        data-testid="pagination"
      />
    </div>
  )
}
