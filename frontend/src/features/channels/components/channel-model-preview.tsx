import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { filterChannelModelGroups, groupChannelModelEntries, type ChannelModelGroup } from '../data/channel-model-aliases';
import type { ChannelModelEntry } from '../data/schema';

export function ChannelModelAliases({ group }: { group: ChannelModelGroup }) {
  const { t } = useTranslation();
  return (
    <div className='space-y-1 px-2 pb-2 text-xs' data-testid='model-aliases'>
      {!group.originalVisible && <p className='text-muted-foreground'>{t('channels.modelPreview.originalHidden')}</p>}
      {group.aliases.length > 0 && (
        <div className='ml-2 space-y-1 border-l pl-3'>
          {group.aliases.map((alias) => (
            <div key={alias.requestModel} className='flex min-w-0 flex-wrap items-center gap-1.5'>
              <span className='min-w-0 font-mono break-all'>{alias.requestModel}</span>
              <Badge variant='outline' className='shrink-0 text-[10px] font-normal'>
                {t(`channels.modelPreview.sources.${alias.source}`, t('channels.modelPreview.alias'))}
              </Badge>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

interface Props {
  models: string[];
  entries: ChannelModelEntry[];
  isUpdating: boolean;
  isError: boolean;
}

export function ChannelModelPreview({ models, entries, isUpdating, isError }: Props) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const debouncedSearch = useDebounce(search, 200);
  const groups = useMemo(
    () => filterChannelModelGroups(groupChannelModelEntries(models, entries), debouncedSearch),
    [models, entries, debouncedSearch]
  );
  return (
    <div className='flex min-h-0 flex-1 flex-col' aria-busy={isUpdating}>
      <div className='shrink-0 space-y-2 pb-3'>
        <h3 className='text-sm font-semibold'>{t('channels.modelPreview.title')}</h3>
        <p className='text-muted-foreground text-xs'>{t('channels.modelPreview.description')}</p>
        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('channels.modelPreview.search')}
          className='h-8'
        />
      </div>
      <div className='min-h-0 flex-1 space-y-3 overflow-y-auto pr-2' data-testid='model-alias-preview-scroll'>
        {isError ? (
          <p role='alert' className='text-destructive text-sm'>
            {t('common.errors.loadFailed')}
          </p>
        ) : isUpdating ? (
          <p role='status' className='text-muted-foreground text-sm'>
            {t('channels.modelPreview.loading')}
          </p>
        ) : groups.length === 0 ? (
          <p className='text-muted-foreground py-6 text-center text-sm'>{t('channels.modelPreview.empty')}</p>
        ) : (
          groups.map((group) => (
            <div key={group.model} className='rounded-lg border p-2'>
              <div className='px-2 py-2 font-mono text-sm font-medium break-all'>{group.model}</div>
              <ChannelModelAliases group={group} />
            </div>
          ))
        )}
      </div>
    </div>
  );
}
