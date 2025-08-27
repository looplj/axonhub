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
  userFilter: string
  sourceFilter: string[]
  channelFilter: string[]
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onUserFilterChange: (filter: string) => void
  onSourceFilterChange: (filters: string[]) => void
  onChannelFilterChange: (filters: string[]) => void
}

export function UsageLogsTable({
  data,
  loading,
  pageInfo,
  totalCount,
  pageSize,
  userFilter,
  sourceFilter,
  channelFilter,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onUserFilterChange,
  onSourceFilterChange,
  onChannelFilterChange,
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
    const userFilterValue = newFilters.find((filter: any) => filter.id === 'user')?.value
    const sourceFilterValue = newFilters.find((filter: any) => filter.id === 'source')?.value
    const channelFilterValue = newFilters.find((filter: any) => filter.id === 'channel')?.value
    
    // Handle user filter
    let userFilterString = ''
    if (userFilterValue) {
      if (Array.isArray(userFilterValue)) {
        userFilterString = userFilterValue[0] || ''
      } else {
        userFilterString = userFilterValue
      }
    }
    onUserFilterChange(userFilterString)
    
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
                  colSpan={usageLogsColumns.length}
                  className='h-24 text-center'
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
                  colSpan={usageLogsColumns.length}
                  className='h-24 text-center'
                >
                  {t('usageLogs.noResults')}
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