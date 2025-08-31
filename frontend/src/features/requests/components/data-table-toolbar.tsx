import { useMemo } from 'react'
import { Cross2Icon } from '@radix-ui/react-icons'
import { Table } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useMe } from '@/features/auth/data/auth'
import { useUsers } from '@/features/users/data/users'
import { useChannels } from '@/features/channels/data/channels'
import { RequestStatus } from '../data/schema'
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter'
import { DataTableViewOptions } from './data-table-view-options'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
}

export function DataTableToolbar<TData>({
  table,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation()
  const isFiltered = table.getState().columnFilters.length > 0

  // Get current user and permissions
  const { user: authUser } = useAuthStore((state) => state.auth)
  const { data: meData } = useMe()
  const user = meData || authUser
  const userScopes = user?.scopes || []
  const isOwner = user?.isOwner || false

  // Check if user has permission to view users and requests
  const canViewUsers =
    isOwner ||
    userScopes.includes('*') ||
    (userScopes.includes('read_users') && userScopes.includes('read_requests'))
    
  // Check if user has permission to view channels
  const canViewChannels =
    isOwner ||
    userScopes.includes('*') ||
    userScopes.includes('read_channels')

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

  // Fetch channels data if user has permission
  const { data: channelsData } = useChannels(
    {
      first: 100,
      orderBy: { field: 'CREATED_AT', direction: 'DESC' },
    },
    {
      disableAutoFetch: !canViewChannels, // Disable auto-fetch if user doesn't have permission
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

  // Prepare channel options for filter
  const channelOptions = useMemo(() => {
    if (!canViewChannels || !channelsData?.edges) return []

    return channelsData.edges.map((edge) => ({
      value: edge.node.id,
      label: edge.node.name,
    }))
  }, [canViewChannels, channelsData])

  const requestStatuses = [
    {
      value: 'pending' as RequestStatus,
      label: t('requests.status.pending'),
    },
    {
      value: 'processing' as RequestStatus,
      label: t('requests.status.processing'),
    },
    {
      value: 'completed' as RequestStatus,
      label: t('requests.status.completed'),
    },
    {
      value: 'failed' as RequestStatus,
      label: t('requests.status.failed'),
    },
  ]

  const requestSources = [
    {
      value: 'api',
      label: t('requests.source.api'),
    },
    {
      value: 'playground', 
      label: t('requests.source.playground'),
    },
    {
      value: 'test',
      label: t('requests.source.test'),
    },
  ]

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 items-center space-x-2'>
        <Input
          placeholder={t('requests.filters.filterId')}
          value={(table.getColumn('id')?.getFilterValue() as string) ?? ''}
          onChange={(event) =>
            table.getColumn('id')?.setFilterValue(event.target.value)
          }
          className='h-8 w-[150px] lg:w-[250px]'
        />
        {table.getColumn('status') && (
          <DataTableFacetedFilter
            column={table.getColumn('status')}
            title={t('requests.filters.status')}
            options={requestStatuses}
          />
        )}
        {table.getColumn('source') && (
          <DataTableFacetedFilter
            column={table.getColumn('source')}
            title={t('requests.filters.source')}
            options={requestSources}
          />
        )}
        {canViewChannels &&
          table.getColumn('channel') &&
          channelOptions.length > 0 && (
            <DataTableFacetedFilter
              column={table.getColumn('channel')}
              title={t('requests.filters.channel')}
              options={channelOptions}
            />
          )}
        {canViewUsers &&
          table.getColumn('user') &&
          userOptions.length > 0 && (
            <DataTableFacetedFilter
              column={table.getColumn('user')}
              title={t('requests.filters.user')}
              options={userOptions}
            />
          )}
        {isFiltered && (
          <Button
            variant='ghost'
            onClick={() => table.resetColumnFilters()}
            className='h-8 px-2 lg:px-3'
          >
            {t('requests.filters.reset')}
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
      </div>
      <DataTableViewOptions table={table} />
    </div>
  )
}
