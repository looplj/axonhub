import { useTranslation } from 'react-i18next';
import { BarChart4, Activity, DollarSign } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import type { AnalyticsOverview } from '../data/analytics';

function formatExactNumber(value: number): string {
  return Math.round(value).toLocaleString();
}
import { useGeneralSettings } from '@/features/system/data/system';
import { useCallback } from 'react';

interface OverviewCardsProps {
  overview: AnalyticsOverview | undefined;
  isLoading: boolean;
}

export function OverviewCards({ overview, isLoading }: OverviewCardsProps) {
  const { t, i18n } = useTranslation();
  const { data: generalSettings } = useGeneralSettings();

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const formatCurrency = useCallback(
    (val: number) =>
      t('currencies.format', {
        val,
        currency: currencyCode,
        locale,
        minimumFractionDigits: 4,
        maximumFractionDigits: 4,
      }),
    [currencyCode, locale, t]
  );

  if (isLoading) {
    return (
      <div className='grid gap-4 md:grid-cols-3'>
        <Skeleton className='h-[120px]' />
        <Skeleton className='h-[120px]' />
        <Skeleton className='h-[120px]' />
      </div>
    );
  }

  const cacheHitRate = overview?.totalInputTokens
    ? ((overview.totalCachedInputTokens / overview.totalInputTokens) * 100).toFixed(1)
    : '0';

  const cards = [
    {
      title: t('analytics.overview.totalTokens'),
      value: formatExactNumber(overview?.totalTokens || 0),
      icon: BarChart4,
      description: `${formatExactNumber(overview?.totalInputTokens || 0)} ${t('dashboard.stats.input')} / ${formatExactNumber(overview?.totalOutputTokens || 0)} ${t('dashboard.stats.output')} · ${t('analytics.overview.cacheHitRate')}: ${cacheHitRate}%`,
    },
    {
      title: t('analytics.overview.totalRequests'),
      value: formatExactNumber(overview?.totalRequests || 0),
      icon: Activity,
      description: null,
    },
    {
      title: t('analytics.overview.totalCost'),
      value: formatCurrency(overview?.totalCost || 0),
      icon: DollarSign,
      description: null,
    },
  ];

  return (
    <div className='grid gap-4 md:grid-cols-3'>
      {cards.map((card) => (
        <Card key={card.title} className='hover-card'>
          <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
            <div className='flex items-center gap-2'>
              <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
                <card.icon className='h-4 w-4' />
              </div>
              <CardTitle className='text-sm font-medium'>{card.title}</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <div className='font-mono text-2xl font-bold'>{card.value}</div>
            {card.description && (
              <p className='text-xs text-muted-foreground'>{card.description}</p>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
