import { useMemo } from 'react'
import { Cross2Icon } from '@radix-ui/react-icons'
import { Table } from '@tanstack/react-table'
import { IconArchive, IconUserOff } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter'
import { useMe } from '@/features/auth/data/auth'
import { useUsers } from '@/features/users/data/users'
import { useApiKeysContext } from '../context/apikeys-context'
import { ApiKeyStatus } from '../data/schema'
import { DataTableViewOptions } from './data-table-view-options'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
  onResetFilters?: () => void
}

export function DataTableToolbar<TData>({ table, onResetFilters }: DataTableToolbarProps<TData>) {
  const { t } = useTranslation()
  const { openDialog } = useApiKeysContext()
  const isFiltered = table.getState().columnFilters.length > 0
  const selectedRows = table.getFilteredSelectedRowModel().rows

  // Get current user and permissions
  const { user: authUser } = useAuthStore((state) => state.auth)
  const { data: meData } = useMe()
  const user = meData || authUser
  const userScopes = user?.scopes || []
  const isOwner = user?.isOwner || false

  // Check if user has permission to view users and API keys
  const canViewUsers =
    isOwner || userScopes.includes('*') || (userScopes.includes('read_users') && userScopes.includes('read_apikeys'))

  // Fetch users data if user has permission
  const { data: usersData } = useUsers(
    {
      first: 100,
      orderBy: { field: 'CREATED_AT', direction: 'DESC' },
    },
    {
      disableAutoFetch: !canViewUsers, // Disable auto-fetch if user doesn't have permission
    }
  )

  // Prepare user options for filter
  const userOptions = useMemo(() => {
    if (!canViewUsers || !usersData?.edges) return []

    return usersData.edges.map((edge) => ({
      value: edge.node.id,
      label: `${edge.node.firstName} ${edge.node.lastName} (${edge.node.email})`,
    }))
  }, [canViewUsers, usersData])

  const statusOptions = [
    {
      value: 'enabled' as ApiKeyStatus,
      label: t('apikeys.status.enabled'),
    },
    {
      value: 'disabled' as ApiKeyStatus,
      label: t('apikeys.status.disabled'),
    },
    {
      value: 'archived' as ApiKeyStatus,
      label: t('apikeys.status.archived'),
    },
  ]

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 items-center space-x-2'>
        <Input
          placeholder={t('apikeys.filters.filterName')}
          value={(table.getColumn('name')?.getFilterValue() as string) ?? ''}
          onChange={(event) => table.getColumn('name')?.setFilterValue(event.target.value)}
          className='h-8 w-[150px] lg:w-[250px]'
        />
        {table.getColumn('status') && (
          <DataTableFacetedFilter
            column={table.getColumn('status')}
            title={t('apikeys.filters.status')}
            options={statusOptions}
          />
        )}
        {canViewUsers && table.getColumn('creator') && userOptions.length > 0 && usersData?.edges && (
          <DataTableFacetedFilter
            column={table.getColumn('creator')}
            title={t('apikeys.filters.user')}
            options={userOptions}
          />
        )}
        {isFiltered && (
          <Button
            variant='ghost'
            onClick={() => {
              table.resetColumnFilters()
              onResetFilters?.()
            }}
            className='h-8 px-2 lg:px-3'
          >
            {t('common.filters.reset')}
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
        {selectedRows.length > 0 && (
          <Button
            variant='destructive'
            size='sm'
            onClick={() =>
              openDialog(
                'bulkDisable',
                selectedRows.map((row) => row.original as any)
              )
            }
            className='h-8'
          >
            <IconUserOff className='mr-2 h-4 w-4' />
            {t('common.buttons.disable')} ({selectedRows.length})
          </Button>
        )}
        {selectedRows.length > 0 && (
          <Button
            variant='outline'
            size='sm'
            onClick={() =>
              openDialog(
                'bulkArchive',
                selectedRows.map((row) => row.original as any)
              )
            }
            className='h-8 border-orange-600 text-orange-600 hover:bg-orange-600 hover:text-white'
          >
            <IconArchive className='mr-2 h-4 w-4' />
            {t('common.buttons.archive')} ({selectedRows.length})
          </Button>
        )}
      </div>
      {/* <DataTableViewOptions table={table} /> */}
    </div>
  )
}
