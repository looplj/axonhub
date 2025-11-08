import { Cross2Icon } from '@radix-ui/react-icons'
import { Table } from '@tanstack/react-table'
import { IconArchive } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DataTableViewOptions } from './data-table-view-options'
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter'
import { useChannels } from '../context/channels-context'
import { ChannelType } from '../data/schema'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
}

export function DataTableToolbar<TData>({
  table,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation()
  const { setOpen } = useChannels()
  const isFiltered = table.getState().columnFilters.length > 0
  const selectedRows = table.getFilteredSelectedRowModel().rows

  const channelTypes = [
    {
      value: 'openai' as ChannelType,
      label: t('channels.types.openai'),
    },
    {
      value: 'anthropic' as ChannelType,
      label: t('channels.types.anthropic'),
    },
    {
      value: 'anthropic_aws' as ChannelType,
      label: t('channels.types.anthropic_aws'),
    },
    {
      value: 'anthropic_gcp' as ChannelType,
      label: t('channels.types.anthropic_gcp'),
    },
    {
      value: 'gemini_openai' as ChannelType,
      label: t('channels.types.gemini_openai'),
    },
    {
      value: 'deepseek' as ChannelType,
      label: t('channels.types.deepseek'),
    },
    {
      value: 'doubao' as ChannelType,
      label: t('channels.types.doubao'),
    },
    {
      value: 'moonshot' as ChannelType,
      label: t('channels.types.moonshot'),
    },
    {
      value: 'openrouter' as ChannelType,
      label: t('channels.types.openrouter'),
    },
    {
      value: 'deepseek_anthropic' as ChannelType,
      label: t('channels.types.deepseek_anthropic'),
    },
    {
      value: 'moonshot_anthropic' as ChannelType,
      label: t('channels.types.moonshot_anthropic'),
    },
    {
      value: 'zhipu_anthropic' as ChannelType,
      label: t('channels.types.zhipu_anthropic'),
    },
    {
      value: 'anthropic_fake' as ChannelType,
      label: t('channels.types.anthropic_fake'),
    },
    {
      value: 'openai_fake' as ChannelType,
      label: t('channels.types.openai_fake'),
    },
  ]

  const channelStatuses = [
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
  ]

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 items-center space-x-2'>
        <Input
          placeholder={t('channels.filters.filterByName')}
          value={(table.getColumn('name')?.getFilterValue() as string) ?? ''}
          onChange={(event) =>
            table.getColumn('name')?.setFilterValue(event.target.value)
          }
          className='h-8 w-[150px] lg:w-[250px]'
        />
        {table.getColumn('type') && (
          <DataTableFacetedFilter
            column={table.getColumn('type')}
            title={t('channels.filters.type')}
            options={channelTypes}
          />
        )}
        {table.getColumn('status') && (
          <DataTableFacetedFilter
            column={table.getColumn('status')}
            title={t('channels.filters.status')}
            options={channelStatuses}
          />
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
        {selectedRows.length > 0 && (
          <Button
            variant='outline'
            size='sm'
            onClick={() => setOpen('bulkArchive')}
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