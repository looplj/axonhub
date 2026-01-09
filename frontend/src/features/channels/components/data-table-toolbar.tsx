import { useMemo, useEffect } from 'react';
import { Cross2Icon } from '@radix-ui/react-icons';
import { Table } from '@tanstack/react-table';
import { useQueryModels } from '@/gql/models';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { DataTableFacetedFilter } from '@/components/data-table-faceted-filter';
import { useAllChannelTags } from '../data/channels';
import { CHANNEL_CONFIGS } from '../data/config_channels';
import { DataTableViewOptions } from './data-table-view-options';

interface DataTableToolbarProps<TData> {
  table: Table<TData>;
  isFiltered?: boolean;
  selectedCount?: number;
  selectedTypeTab?: string;
  showErrorOnly?: boolean;
  onExitErrorOnlyMode?: () => void;
}

export function DataTableToolbar<TData>({
  table,
  isFiltered: externalIsFiltered,
  selectedCount: externalSelectedCount,
  selectedTypeTab = 'all',
  showErrorOnly,
  onExitErrorOnlyMode,
}: DataTableToolbarProps<TData>) {
  const { t } = useTranslation();
  const tableState = table.getState();
  const isFiltered = externalIsFiltered ?? tableState.columnFilters.length > 0;

  // Get all channel tags from GraphQL
  const { data: allTags = [] } = useAllChannelTags();

  // Fetch models using the models query
  const { mutate: fetchModels, data: modelsData } = useQueryModels();

  // Fetch models on component mount
  useEffect(() => {
    fetchModels({
      statusIn: ['enabled', 'disabled'],
      includeMapping: true,
      includePrefix: true,
    });
  }, [fetchModels]);

  const tagOptions = useMemo(
    () =>
      allTags.map((tag) => ({
        value: tag,
        label: tag,
      })),
    [allTags]
  );

  const modelOptions = useMemo(() => {
    if (!modelsData) return [];
    return modelsData.map((model) => ({
      value: model.id,
      label: model.id,
    }));
  }, [modelsData]);

  // Generate channel types from CHANNEL_CONFIGS
  const channelTypes = useMemo(
    () =>
      Object.values(CHANNEL_CONFIGS).map((config) => ({
        value: config.channelType,
        label: t(`channels.types.${config.channelType}`),
      })),
    [t]
  );

  const channelStatuses = useMemo(
    () => [
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
    ],
    [t]
  );

  return (
    <div className='flex items-center gap-4'>
      <div className='relative flex-1'>
        <i className='ph ph-magnifying-glass text-muted-foreground absolute top-2.5 left-3'></i>
        <Input
          placeholder={t('channels.filters.filterByName')}
          value={(table.getColumn('name')?.getFilterValue() as string) ?? ''}
          onChange={(event) => table.getColumn('name')?.setFilterValue(event.target.value)}
          className='bg-card border-border focus:ring-primary/20 placeholder-muted-foreground text-foreground w-full rounded-xl border py-2 pr-4 pl-10 text-sm shadow-sm transition-all focus:ring-2 focus:outline-none'
        />
      </div>
      {table.getColumn('status') && (
        <DataTableFacetedFilter column={table.getColumn('status')} title={t('channels.filters.status')} options={channelStatuses} />
      )}
      {table.getColumn('tags') && tagOptions?.length > 0 && (
        <DataTableFacetedFilter column={table.getColumn('tags')} title={t('channels.filters.tags')} options={tagOptions} singleSelect />
      )}
      {table.getColumn('model') && modelOptions?.length > 0 && (
        <DataTableFacetedFilter column={table.getColumn('model')} title={t('channels.filters.model')} options={modelOptions} singleSelect />
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
      {showErrorOnly && onExitErrorOnlyMode && (
        <Button
          variant='outline'
          onClick={onExitErrorOnlyMode}
          className='h-8 border-orange-600 text-orange-600 hover:bg-orange-600 hover:text-white'
        >
          {t('channels.errorBanner.exitErrorOnlyButton')}
        </Button>
      )}
      <DataTableViewOptions table={table} />
    </div>
  );
}
