import React, { useState } from 'react'
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
import { useTranslation } from 'react-i18next'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { useApiKeysContext } from '../context/apikeys-context'
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
  loading: boolean
  pageInfo?: ApiKeyConnection['pageInfo']
  pageSize: number
  totalCount?: number
  nameFilter: string
  statusFilter: string[]
  userFilter: string[]
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onNameFilterChange: (value: string) => void
  onStatusFilterChange: (value: string[]) => void
  onUserFilterChange: (value: string[]) => void
  onResetFilters?: () => void
  onBulkDisable?: (apiKeys: ApiKey[]) => void
}

export function ApiKeysTable({
  columns,
  data,
  loading,
  pageInfo,
  pageSize,
  totalCount,
  nameFilter,
  statusFilter,
  userFilter,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onNameFilterChange,
  onStatusFilterChange,
  onUserFilterChange,
  onResetFilters,
  onBulkDisable,
}: DataTableProps) {
  const { t } = useTranslation()
  const { setResetRowSelection } = useApiKeysContext()
  const [rowSelection, setRowSelection] = useState({})
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [sorting, setSorting] = useState<SortingState>([])

  // 注册重置选择的方法到 context
  React.useEffect(() => {
    setResetRowSelection(() => () => setRowSelection({}))
  }, [setRowSelection])

  // Sync server state to local column filters (for UI display)
  React.useEffect(() => {
    const newFilters: ColumnFiltersState = []
    if (nameFilter) {
      newFilters.push({ id: 'name', value: nameFilter })
    }
    if (statusFilter.length > 0) {
      newFilters.push({ id: 'status', value: statusFilter })
    }
    if (userFilter.length > 0) {
      newFilters.push({ id: 'creator', value: userFilter })
    }
    setColumnFilters(newFilters)
  }, [nameFilter, statusFilter, userFilter])

  const handleColumnFiltersChange = (
    updater: ColumnFiltersState | ((prev: ColumnFiltersState) => ColumnFiltersState)
  ) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater
    setColumnFilters(newFilters)

    // Extract filter values
    const nameFilterValue = newFilters.find((f) => f.id === 'name')?.value
    const statusFilterValue = newFilters.find((f) => f.id === 'status')?.value
    const userFilterValue = newFilters.find((f) => f.id === 'creator')?.value

    // Only update if values actually change to prevent reset issues
    const newNameFilter = typeof nameFilterValue === 'string' ? nameFilterValue : ''
    if (newNameFilter !== nameFilter) {
      onNameFilterChange(newNameFilter)
    }

    const newStatusFilter = Array.isArray(statusFilterValue) ? statusFilterValue : []
    if (JSON.stringify(newStatusFilter.sort()) !== JSON.stringify(statusFilter.sort())) {
      onStatusFilterChange(newStatusFilter)
    }

    const newUserFilter = Array.isArray(userFilterValue) ? userFilterValue : []
    if (JSON.stringify(newUserFilter.sort()) !== JSON.stringify(userFilter.sort())) {
      onUserFilterChange(newUserFilter)
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
    manualPagination: true,
    manualFiltering: true,
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    getRowId: (row) => row.id, // 使用API Key的ID作为行ID
  })

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <DataTableToolbar table={table} onResetFilters={onResetFilters} />
      <div className='mt-4 flex-1 overflow-auto rounded-2xl shadow-soft border border-[var(--table-border)] relative'>
        <Table className='bg-background rounded-2xl border-separate border-spacing-0'>
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
                      {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody className='p-2 space-y-1 !bg-background'>
            {loading ? (
              <TableRow className='border-0 !bg-background'>
                <TableCell colSpan={columns.length} className='h-24 text-center border-0 !bg-background'>
                  {t('common.loading')}
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id} data-state={row.getIsSelected() && 'selected'} className='group/row table-row-hover rounded-xl !bg-background border-0 transition-all duration-200 ease-in-out'>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className={`${cell.column.columnDef.meta?.className ?? ''} px-4 py-3 border-0 !bg-background`}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow className='!bg-background'>
                <TableCell colSpan={columns.length} className='h-24 text-center !bg-background'>
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
          selectedRows={Object.keys(rowSelection).length}
          onNextPage={onNextPage}
          onPreviousPage={onPreviousPage}
          onPageSizeChange={onPageSizeChange}
        />
      </div>
    </div>
  )
}
