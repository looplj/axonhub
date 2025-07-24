import { DataTablePagination as CommonDataTablePagination } from '@/components/data-table-pagination'
import { Table } from '@tanstack/react-table'

interface DataTablePaginationProps<TData> {
  table: Table<TData>
}

export function DataTablePagination<TData>({
  table,
}: DataTablePaginationProps<TData>) {
  return <CommonDataTablePagination table={table} />
}
