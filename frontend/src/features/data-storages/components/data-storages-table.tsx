'use client'

import { useTranslation } from 'react-i18next'
import {
  ColumnDef,
  flexRender,
  getCoreRowModel,
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
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DataStorage, PageInfo } from '../data/data-storages'
import { ChevronLeft, ChevronRight } from 'lucide-react'

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
    <div className='space-y-4'>
      <div className='flex items-center gap-2'>
        <Input
          placeholder={t('dataStorages.filters.searchByName', '按名称搜索...')}
          value={nameFilter}
          onChange={(e) => onNameFilterChange(e.target.value)}
          className='max-w-sm'
        />
      </div>

      <div className='rounded-md border'>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                          header.column.columnDef.header,
                          header.getContext()
                        )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && 'selected'}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
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
                  {t('common.noResults')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <div className='flex items-center justify-between'>
        <div className='text-muted-foreground text-sm'>
          {totalCount !== undefined && (
            <span>
              {t('common.totalCount', { count: totalCount })}
            </span>
          )}
        </div>
        <div className='flex items-center space-x-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={onPreviousPage}
            disabled={!pageInfo?.hasPreviousPage}
          >
            <ChevronLeft className='h-4 w-4' />
            {t('common.previous')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={onNextPage}
            disabled={!pageInfo?.hasNextPage}
          >
            {t('common.next')}
            <ChevronRight className='h-4 w-4' />
          </Button>
        </div>
      </div>
    </div>
  )
}
