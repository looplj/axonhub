import { Cross2Icon } from '@radix-ui/react-icons'
import { Table } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DataTableViewOptions } from './data-table-view-options'
import { DataTableFacetedFilter } from './data-table-faceted-filter'
import { ChannelType } from '../data/schema'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
}

export function DataTableToolbar<TData>({
  table,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation()
  const isFiltered = table.getState().columnFilters.length > 0

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
      value: 'gemini' as ChannelType,
      label: t('channels.types.gemini'),
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
      value: 'kimi' as ChannelType,
      label: t('channels.types.kimi'),
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
        {isFiltered && (
          <Button
            variant='ghost'
            onClick={() => table.resetColumnFilters()}
            className='h-8 px-2 lg:px-3'
          >
            {t('channels.filters.reset')}
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
      </div>
      <DataTableViewOptions table={table} />
    </div>
  )
}