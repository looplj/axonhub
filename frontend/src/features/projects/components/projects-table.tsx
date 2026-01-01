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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { Project, ProjectConnection } from '../data/schema'
import { DataTableToolbar } from './data-table-toolbar'

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className: string
  }
}

interface DataTableProps {
  columns: ColumnDef<Project>[]
  loading?: boolean
  data: Project[]
  pageInfo?: ProjectConnection['pageInfo']
  pageSize: number
  totalCount?: number
  onNextPage: () => void
  onPreviousPage: () => void
  onPageSizeChange: (pageSize: number) => void
  searchFilter: string
  onSearchFilterChange: (value: string) => void
}

export function ProjectsTable({
  columns,
  data,
  loading,
  pageInfo,
  pageSize,
  totalCount,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  searchFilter,
  onSearchFilterChange,
}: DataTableProps) {
  const { t } = useTranslation()
  const [rowSelection, setRowSelection] = useState({})
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [sorting, setSorting] = useState<SortingState>([])

  // Sync server state to local column filters (for UI display)
  React.useEffect(() => {
    const newFilters: ColumnFiltersState = []
    if (searchFilter) {
      // Use 'search' as a virtual column ID for the combined search
      newFilters.push({ id: 'search', value: searchFilter })
    }
    setColumnFilters(newFilters)
  }, [searchFilter])

  const handleColumnFiltersChange = (
    updater:
      | ColumnFiltersState
      | ((prev: ColumnFiltersState) => ColumnFiltersState)
  ) => {
    const newFilters =
      typeof updater === 'function' ? updater(columnFilters) : updater
    setColumnFilters(newFilters)

    // Extract search filter value
    const searchFilterValue = newFilters.find((f) => f.id === 'search')?.value

    // Only update if values actually change to prevent reset issues
    const newSearchFilter =
      typeof searchFilterValue === 'string' ? searchFilterValue : ''
    if (newSearchFilter !== searchFilter) {
      onSearchFilterChange(newSearchFilter)
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
    <div className='flex flex-1 flex-col overflow-hidden' data-testid='projects-table'>
      <DataTableToolbar table={table} />
      <div className='mt-4 flex-1 overflow-auto rounded-2xl shadow-soft border border-[var(--table-border)] relative'>
        <Table data-testid='projects-table' className='bg-[var(--table-background)] rounded-2xl border-separate border-spacing-0'>
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
                      colSpan={columns.length}
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
                      colSpan={columns.length}
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
          selectedRows={Object.keys(rowSelection).length}
          onNextPage={onNextPage}
          onPreviousPage={onPreviousPage}
          onPageSizeChange={onPageSizeChange}
          data-testid='pagination'
        />
      </div>
    </div>
  )
}
