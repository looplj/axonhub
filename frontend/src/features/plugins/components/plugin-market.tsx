'use client';

import { useState } from 'react';
import { IconArrowUp, IconCircleCheckFilled, IconDownload, IconLoader2, IconBuildingStore, IconSearch } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { usePluginMarket, type PluginMarketEntry } from '../data/plugins';
import { matchesPluginSearch, PluginAvatar, PluginEmptyState, PluginGrid, PluginGridSkeleton } from './plugin-visuals';

interface PluginMarketProps {
  search: string;
  canManage: boolean;
  busy: boolean;
  onInstall: (entry: PluginMarketEntry, intent: 'install' | 'upgrade') => Promise<unknown>;
}

function isSet(value: string | null | undefined): value is string {
  return typeof value === 'string' && value.length > 0;
}

export function PluginMarket({ search, canManage, busy, onInstall }: PluginMarketProps) {
  const { t } = useTranslation();
  const marketQuery = usePluginMarket();
  const [activeID, setActiveID] = useState<string | null>(null);

  const entries = marketQuery.data?.pluginMarket ?? [];
  const visible = entries.filter((entry) => matchesPluginSearch(search, entry.name, entry.id, entry.description));

  const begin = async (entry: PluginMarketEntry, intent: 'install' | 'upgrade') => {
    setActiveID(entry.id);
    try {
      await onInstall(entry, intent);
    } finally {
      setActiveID(null);
    }
  };

  if (marketQuery.isLoading) {
    return <PluginGridSkeleton />;
  }
  if (marketQuery.isError) {
    return (
      <PluginEmptyState
        icon={<IconBuildingStore />}
        title={t('plugins.market.loadFailed')}
        description={marketQuery.error instanceof Error ? marketQuery.error.message : undefined}
        action={
          <Button type='button' variant='outline' size='sm' onClick={() => marketQuery.refetch()}>
            {t('plugins.page.retry')}
          </Button>
        }
      />
    );
  }
  if (entries.length === 0) {
    return <PluginEmptyState icon={<IconBuildingStore />} title={t('plugins.market.empty')} />;
  }
  if (visible.length === 0) {
    return <PluginEmptyState icon={<IconSearch />} title={t('plugins.noMatch')} />;
  }

  return (
    <PluginGrid>
      {visible.map((entry) => (
        <div
          key={entry.id}
          data-testid='plugin-market-entry'
          className='bg-card flex flex-col overflow-hidden rounded-2xl border shadow-xs transition-all hover:shadow-md'
        >
          <div className='flex flex-1 flex-col gap-3 p-4'>
            <div className='flex items-start gap-3'>
              <PluginAvatar id={entry.id} name={entry.name} />
              <div className='min-w-0 flex-1'>
                <div className='truncate font-semibold'>{entry.name || entry.id}</div>
                <div className='text-muted-foreground truncate font-mono text-xs'>{entry.id}</div>
              </div>
              <span className='text-muted-foreground bg-muted shrink-0 rounded-md px-1.5 py-0.5 font-mono text-[11px]'>{entry.version}</span>
            </div>
            <p className='text-muted-foreground line-clamp-3 min-h-10 text-sm'>{entry.description}</p>
          </div>
          <div className='bg-muted/30 flex min-h-[52px] items-center gap-3 border-t px-4 py-2.5'>
            <MarketState entry={entry} />
            <div className='ml-auto'>
              <MarketAction entry={entry} canManage={canManage} busy={busy} active={activeID === entry.id} onBegin={begin} />
            </div>
          </div>
        </div>
      ))}
    </PluginGrid>
  );
}

function MarketState({ entry }: { entry: PluginMarketEntry }) {
  const { t } = useTranslation();

  if (isSet(entry.updateFrom)) {
    return (
      <span className='text-primary inline-flex items-center gap-1 text-xs font-medium'>
        <IconArrowUp className='size-3.5' />
        {t('plugins.market.updateAvailable', { from: entry.updateFrom, to: entry.version })}
      </span>
    );
  }
  if (isSet(entry.installedVersion)) {
    return (
      <span className='text-muted-foreground inline-flex items-center gap-1 text-xs font-medium'>
        <IconCircleCheckFilled className='size-3.5 text-emerald-500' />
        {t('plugins.market.installedVersion', { version: entry.installedVersion })}
      </span>
    );
  }
  return null;
}

function MarketAction({
  entry,
  canManage,
  busy,
  active,
  onBegin,
}: {
  entry: PluginMarketEntry;
  canManage: boolean;
  busy: boolean;
  active: boolean;
  onBegin: (entry: PluginMarketEntry, intent: 'install' | 'upgrade') => void;
}) {
  const { t } = useTranslation();

  if (!canManage || (isSet(entry.installedVersion) && !isSet(entry.updateFrom))) {
    return null;
  }

  const upgrade = isSet(entry.updateFrom);
  const Icon = active ? IconLoader2 : upgrade ? IconArrowUp : IconDownload;

  return (
    <Button
      type='button'
      size='sm'
      variant={upgrade ? 'default' : 'outline'}
      disabled={busy}
      onClick={() => onBegin(entry, upgrade ? 'upgrade' : 'install')}
    >
      <Icon className={active ? 'size-4 animate-spin' : 'size-4'} />
      {upgrade ? t('plugins.market.upgrade') : t('plugins.market.install')}
    </Button>
  );
}
