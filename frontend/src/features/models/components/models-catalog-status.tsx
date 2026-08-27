import { RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { useDevelopersData, useRefreshProvidersCatalog } from '../data/providers';

export function ModelsCatalogStatus() {
  const { t, i18n } = useTranslation();
  const catalog = useDevelopersData();
  const refreshCatalog = useRefreshProvidersCatalog();
  const source = catalog.data ? catalog.source : 'fallback';
  const fetchedAt = catalog.fetchedAt;
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';
  const formattedFetchedAt = fetchedAt
    ? new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(fetchedAt))
    : '';

  return (
    <div className='text-muted-foreground flex flex-wrap items-center gap-3 text-sm'>
      <span>
        {t('models.catalog.sourceLabel')}: {t(`models.catalog.source.${source}`, { defaultValue: source })}
      </span>
      {formattedFetchedAt ? <span>{t('models.catalog.updatedAt', { time: formattedFetchedAt })}</span> : null}
      <Button
        type='button'
        variant='ghost'
        size='sm'
        className='h-7 px-2'
        onClick={() => refreshCatalog.mutate()}
        disabled={refreshCatalog.isPending}
      >
        <RefreshCw className={`mr-1 h-3.5 w-3.5 ${refreshCatalog.isPending ? 'animate-spin' : ''}`} />
        {t('models.catalog.refreshNow')}
      </Button>
    </div>
  );
}
