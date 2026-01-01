'use client'

import { ColumnDef, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { DataStorage } from '../data/data-storages'
import type { PageInfo } from '@/gql/pagination'

interface DataStoragesTableProps {
  data: DataStorage[]
  columns: ColumnDef<DataStorage>[]
  pageInfo?: PageInfo
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

export function DataStoragesTable({
  data,
  columns,
  pageInfo,
  pageSize,
  totalCount,
  nameFilter,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onNameFilterChange,
}: DataStoragesTableProps) {
  const { t } = useTranslation()

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <div className='flex items-center gap-2'>
        <Input
          placeholder={t('dataStorages.filters.searchByName')}
          value={nameFilter}
          onChange={(e) => onNameFilterChange(e.target.value)}
          className='max-w-sm'
        />
      </div>

      <div className='mt-4 flex-1 overflow-auto rounded-2xl shadow-soft border border-[var(--table-border)] relative'>
        <Table className='bg-[var(--table-background)] rounded-2xl border-separate border-spacing-0'>
          <TableHeader className='sticky top-0 z-20 bg-[var(--table-header)] shadow-sm'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row border-0'>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id} className='text-xs font-semibold text-muted-foreground uppercase tracking-wider border-0'>
                    {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody className='p-2 space-y-1 !bg-[var(--table-background)]'>
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id} data-state={row.getIsSelected() && 'selected'} className='group/row table-row-hover rounded-xl !bg-[var(--table-background)] border-0 transition-all duration-200 ease-in-out'>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className='px-4 py-3 border-0 !bg-[var(--table-background)]'>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow className='!bg-[var(--table-background)]'>
                <TableCell colSpan={columns.length} className='h-24 text-center !bg-[var(--table-background)]'>
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
          selectedRows={0}
          onNextPage={onNextPage}
          onPreviousPage={onPreviousPage}
          onPageSizeChange={onPageSizeChange}
        />
      </div>
    </div>
  )
}
