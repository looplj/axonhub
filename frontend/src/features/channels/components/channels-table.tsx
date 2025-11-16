import { useState, useEffect, useMemo } from 'react'
import {
  ColumnDef,
  ColumnFiltersState,
  RowData,
  RowSelectionState,
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { useChannels } from '../context/channels-context'
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
  tagFilter: string
  selectedTypeTab?: string
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  onNameFilterChange: (filter: string) => void
  onTypeFilterChange: (filters: string[]) => void
  onStatusFilterChange: (filters: string[]) => void
  onTagFilterChange: (filter: string) => void
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
  tagFilter,
  selectedTypeTab = 'all',
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onNameFilterChange,
  onTypeFilterChange,
  onStatusFilterChange,
  onTagFilterChange,
}: DataTableProps) {
  const { t } = useTranslation()
  const { setSelectedChannels, setResetRowSelection } = useChannels()
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    tags: false, // Hide tags column by default but keep it for filtering
  })
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
    if (tagFilter) {
      newColumnFilters.push({ id: 'tags', value: tagFilter })
    }

    setColumnFilters(newColumnFilters)
  }, [nameFilter, typeFilter, statusFilter, tagFilter])

  // Handle column filter changes and sync with server
  const handleColumnFiltersChange = (
    updater: ColumnFiltersState | ((prev: ColumnFiltersState) => ColumnFiltersState)
  ) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater
    setColumnFilters(newFilters)

    // Extract filter values
    const nameFilterValue = newFilters.find((filter) => filter.id === 'name')?.value as string
    const typeFilterValue = newFilters.find((filter) => filter.id === 'type')?.value as string[]
    const statusFilterValue = newFilters.find((filter) => filter.id === 'status')?.value as string[]
    const tagFilterValue = newFilters.find((filter) => filter.id === 'tags')?.value as string

    // Update server filters only if changed
    const newNameFilter = nameFilterValue || ''
    const newTypeFilter = Array.isArray(typeFilterValue) ? typeFilterValue : []
    const newStatusFilter = Array.isArray(statusFilterValue) ? statusFilterValue : []
    const newTagFilter = tagFilterValue || ''

    if (newNameFilter !== nameFilter) {
      onNameFilterChange(newNameFilter)
    }

    if (JSON.stringify(newTypeFilter.sort()) !== JSON.stringify(typeFilter.sort())) {
      onTypeFilterChange(newTypeFilter)
    }

    if (JSON.stringify(newStatusFilter.sort()) !== JSON.stringify(statusFilter.sort())) {
      onStatusFilterChange(newStatusFilter)
    }

    if (newTagFilter !== tagFilter) {
      onTagFilterChange(newTagFilter)
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
    getRowId: (row) => row.id,
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

  const filteredSelectedRows = useMemo(
    () => table.getFilteredSelectedRowModel().rows,
    [table, rowSelection, data]
  )
  const selectedCount = filteredSelectedRows.length
  const isFiltered = columnFilters.length > 0

  useEffect(() => {
    const resetFn = () => {
      setRowSelection({})
    }
    setResetRowSelection(resetFn)
  }, [setResetRowSelection])

  useEffect(() => {
    const selected = filteredSelectedRows.map((row) => row.original as Channel)
    setSelectedChannels(selected)
  }, [filteredSelectedRows, setSelectedChannels])

  useEffect(() => {
    if (selectedCount === 0) {
      setSelectedChannels([])
    }
  }, [selectedCount, setSelectedChannels])

  // Clear rowSelection when data changes and selected rows no longer exist
  useEffect(() => {
    if (Object.keys(rowSelection).length > 0 && data.length > 0) {
      const dataIds = new Set(data.map((channel) => channel.id))
      const selectedIds = Object.keys(rowSelection)
      const anySelectedIdMissing = selectedIds.some((id) => !dataIds.has(id))
      
      if (anySelectedIdMissing) {
        // Some selected rows no longer exist in the new data, clear selection
        setRowSelection({})
      }
    }
  }, [data, rowSelection])

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <DataTableToolbar 
        table={table} 
        isFiltered={isFiltered} 
        selectedCount={selectedCount}
        selectedTypeTab={selectedTypeTab}
      />
      <div className='mt-4 flex-1 overflow-auto rounded-md border'>
        <Table data-testid='channels-table'>
          <TableHeader className='bg-background sticky top-0 z-10'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row'>
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead
                      key={header.id}
                      colSpan={header.colSpan}
                      className={header.column.columnDef.meta?.className ?? ''}
                    >
                      {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={columns.length} className='h-24 text-center'>
                  {t('common.loading')}
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id} data-state={row.getIsSelected() && 'selected'} className='group/row'>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className={cell.column.columnDef.meta?.className ?? ''}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className='h-24 text-center'>
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
          selectedRows={selectedCount}
          onNextPage={onNextPage}
          onPreviousPage={onPreviousPage}
          onPageSizeChange={onPageSizeChange}
        />
      </div>
    </div>
  )
}
