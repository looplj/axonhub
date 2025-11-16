import { Cross2Icon } from '@radix-ui/react-icons'
import { Table } from '@tanstack/react-table'
import { IconArchive, IconBan, IconCheck, IconTrash } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter'
import { useChannels } from '../context/channels-context'
import { CHANNEL_CONFIGS } from '../data/constants'
import { useAllChannelTags } from '../data/channels'
import { useMemo } from 'react'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
  isFiltered?: boolean
  selectedCount?: number
  selectedTypeTab?: string
}

export function DataTableToolbar<TData>({
  table,
  isFiltered: externalIsFiltered,
  selectedCount: externalSelectedCount,
  selectedTypeTab = 'all',
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation()
  const { setOpen } = useChannels()
  const tableState = table.getState()
  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedCount = externalSelectedCount ?? selectedRows.length
  const isFiltered = externalIsFiltered ?? tableState.columnFilters.length > 0
  const hasSelectedRows = selectedCount > 0

  // Get all channel tags from GraphQL
  const { data: allTags = [] } = useAllChannelTags()

  const tagOptions = useMemo(() => allTags.map((tag) => ({
    value: tag,
    label: tag,
  })), [allTags])

  // Generate channel types from CHANNEL_CONFIGS
  const channelTypes = useMemo(() => Object.values(CHANNEL_CONFIGS).map((config) => ({
    value: config.channelType,
    label: t(`channels.types.${config.channelType}`),
  })), [t])

  const channelStatuses = useMemo(() => [
    {
      value: 'enabled',
      label: t('channels.status.enabled'),
    },
    {
      value: 'disabled',
      label: t('channels.status.disabled'),
    },
    {
      value: 'archived',
      label: t('channels.status.archived'),
    },
  ], [t])

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 items-center space-x-2'>
        <Input
          placeholder={t('channels.filters.filterByName')}
          value={(table.getColumn('name')?.getFilterValue() as string) ?? ''}
          onChange={(event) => table.getColumn('name')?.setFilterValue(event.target.value)}
          className='h-8 w-[150px] lg:w-[250px]'
        />
        {table.getColumn('type') && selectedTypeTab === 'all' && (
          <DataTableFacetedFilter column={table.getColumn('type')} title={t('channels.filters.type')} options={channelTypes} />
        )}
        {table.getColumn('status') && (
          <DataTableFacetedFilter column={table.getColumn('status')} title={t('channels.filters.status')} options={channelStatuses} />
        )}
        {table.getColumn('tags') && tagOptions.length > 0 && (
          <DataTableFacetedFilter column={table.getColumn('tags')} title={t('channels.filters.tags')} options={tagOptions} singleSelect />
        )}
        {isFiltered && (
          <Button variant='ghost' onClick={() => table.resetColumnFilters()} className='h-8 px-2 lg:px-3'>
            {t('common.filters.reset')}
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
        {hasSelectedRows && (
          <>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setOpen('bulkEnable')}
              className='h-8 border-green-600 text-green-600 hover:bg-green-600 hover:text-white'
            >
              <IconCheck className='mr-2 h-4 w-4' />
              {t('common.buttons.enable')} ({selectedCount})
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setOpen('bulkDisable')}
              className='h-8 border-amber-600 text-amber-600 hover:bg-amber-600 hover:text-white'
            >
              <IconBan className='mr-2 h-4 w-4' />
              {t('common.buttons.disable')} ({selectedCount})
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setOpen('bulkArchive')}
              className='h-8 border-orange-600 text-orange-600 hover:bg-orange-600 hover:text-white'
            >
              <IconArchive className='mr-2 h-4 w-4' />
              {t('common.buttons.archive')} ({selectedCount})
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setOpen('bulkDelete')}
              className='h-8 border-red-600 text-red-600 hover:bg-red-600 hover:text-white'
            >
              <IconTrash className='mr-2 h-4 w-4' />
              {t('common.buttons.delete')} ({selectedCount})
            </Button>
          </>
        )}
      </div>
      {/* <DataTableViewOptions table={table} /> */}
    </div>
  )
}
