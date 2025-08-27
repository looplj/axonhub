import { Cross2Icon } from '@radix-ui/react-icons'
import { Table } from '@tanstack/react-table'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DataTableViewOptions } from './data-table-view-options'
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter'
import { ChannelType } from '../../channels/data/schema'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
}

const channelTypes = [
  {
    value: 'openai' as ChannelType,
    label: 'OpenAI',
  },
  {
    value: 'anthropic' as ChannelType,
    label: 'Anthropic',
  },
  {
    value: 'gemini' as ChannelType,
    label: 'Gemini',
  },
  {
    value: 'deepseek' as ChannelType,
    label: 'DeepSeek',
  },
  {
    value: 'doubao' as ChannelType,
    label: 'Doubao',
  },
  {
    value: 'kimi' as ChannelType,
    label: 'Kimi',
  },
]

export function DataTableToolbar<TData>({
  table,
}: DataTableToolbarProps<TData>) {
  const isFiltered = table.getState().columnFilters.length > 0

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 items-center space-x-2'>
        <Input
          placeholder='按名称筛选...'
          value={(table.getColumn('name')?.getFilterValue() as string) ?? ''}
          onChange={(event) =>
            table.getColumn('name')?.setFilterValue(event.target.value)
          }
          className='h-8 w-[150px] lg:w-[250px]'
        />
        {table.getColumn('type') && (
          <DataTableFacetedFilter
            column={table.getColumn('type')}
            title='类型'
            options={channelTypes}
          />
        )}
        {isFiltered && (
          <Button
            variant='ghost'
            onClick={() => table.resetColumnFilters()}
            className='h-8 px-2 lg:px-3'
          >
            重置
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
      </div>
      <DataTableViewOptions table={table} />
    </div>
  )
}