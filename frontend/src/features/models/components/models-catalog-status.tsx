import { useTranslation } from 'react-i18next';
import { useDevelopersData } from '../data/providers';

export function ModelsCatalogStatus() {
  const { t, i18n } = useTranslation();
  const catalog = useDevelopersData();
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
    </div>
  );
}
