import { Cross2Icon } from '@radix-ui/react-icons'
import { Table } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useRolesContext } from '../context/roles-context'
import { Role } from '../data/schema'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
}

export function DataTableToolbar<TData>({
  table,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation()
  const { setDeletingRoles } = useRolesContext()
  const isFiltered = table.getState().columnFilters.length > 0
  const selectedRows = table.getFilteredSelectedRowModel().rows

  const handleBulkDelete = () => {
    const roles = selectedRows.map((row) => row.original as Role)
    setDeletingRoles(roles)
  }

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 flex-col-reverse items-start gap-y-2 sm:flex-row sm:items-center sm:space-x-2'>
        <Input
          placeholder={t('roles.searchRoles')}
          value={(table.getColumn('search')?.getFilterValue() as string) ?? ''}
          onChange={(event) =>
            table.getColumn('search')?.setFilterValue(event.target.value)
          }
          className='h-8 w-[150px] lg:w-[300px]'
        />
        {selectedRows.length > 0 && (
          <Button
            variant='destructive'
            size='sm'
            onClick={handleBulkDelete}
            className='h-8'
          >
            <Trash2 className='mr-2 h-4 w-4' />
            {t('common.buttons.delete')} ({selectedRows.length})
          </Button>
        )}
        {isFiltered && (
          <Button
            variant='ghost'
            onClick={() => table.resetColumnFilters()}
            className='h-8 px-2 lg:px-3'
          >
            {t('common.filters.reset')}
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
      </div>
    </div>
  )
}