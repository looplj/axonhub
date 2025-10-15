import { useMemo } from 'react'
import { Cross2Icon } from '@radix-ui/react-icons'
import { Table } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter'
import { useUsageLogPermissions } from '../../../gql/useUsageLogPermissions'
import { useChannels } from '../../channels/data'
import { UsageLogSource } from '../data/schema'
import { DataTableViewOptions } from './data-table-view-options'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
}

export function DataTableToolbar<TData>({ table }: DataTableToolbarProps<TData>) {
  const { t } = useTranslation()
  const permissions = useUsageLogPermissions()
  const { canViewChannels } = permissions

  const isFiltered = table.getState().columnFilters.length > 0

  // Fetch channels data if user has permission
  const { data: channelsData } = useChannels(
    canViewChannels
      ? {
          first: 100,
          orderBy: { field: 'CREATED_AT', direction: 'DESC' },
        }
      : undefined
  )

  // Prepare channel options for filter
  const channelOptions = useMemo(() => {
    if (!canViewChannels || !channelsData?.edges) return []

    return channelsData.edges.map((edge) => ({
      value: edge.node.id,
      label: edge.node.name || `Channel ${edge.node.id}`,
    }))
  }, [canViewChannels, channelsData])

  const usageLogSources = [
    {
      value: 'api' as UsageLogSource,
      label: t('usageLogs.source.api'),
    },
    {
      value: 'playground' as UsageLogSource,
      label: t('usageLogs.source.playground'),
    },
    {
      value: 'test' as UsageLogSource,
      label: t('usageLogs.source.test'),
    },
  ]

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 items-center space-x-2'>
        <Input
          placeholder={t('usageLogs.filters.filterId')}
          value={(table.getColumn('id')?.getFilterValue() as string) ?? ''}
          onChange={(event) => table.getColumn('id')?.setFilterValue(event.target.value)}
          className='h-8 w-[150px] lg:w-[250px]'
        />
        {table.getColumn('source') && (
          <DataTableFacetedFilter
            column={table.getColumn('source')}
            title={t('usageLogs.filters.source')}
            options={usageLogSources}
          />
        )}
        {canViewChannels && table.getColumn('channel') && channelOptions.length > 0 && channelsData?.edges && (
          <DataTableFacetedFilter
            column={table.getColumn('channel')}
            title={t('usageLogs.filters.channel')}
            options={channelOptions}
          />
        )}
        {isFiltered && (
          <Button variant='ghost' onClick={() => table.resetColumnFilters()} className='h-8 px-2 lg:px-3'>
            {t('usageLogs.filters.reset')}
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
      </div>
      <DataTableViewOptions table={table} />
    </div>
  )
}
