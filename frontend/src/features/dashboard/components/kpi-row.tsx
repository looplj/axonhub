import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Activity, BarChart4, DollarSign, ShieldCheck } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { useGeneralSettings } from '@/features/system/data/system';
import type { AnalyticsOverview } from '@/features/analytics/data/analytics';

/** Thousands-separated integer; KPI totals must not be abbreviated. */
function formatExactNumber(value: number): string {
  return Math.round(value).toLocaleString();
}

interface KpiRowProps {
  overview: AnalyticsOverview | undefined;
  isLoading: boolean;
}

/** KPI card row: requests, tokens, cost and success rate for the selected range. */
export function KpiRow({ overview, isLoading }: KpiRowProps) {
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
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }),
    [currencyCode, locale, t]
  );

  const successRate = overview?.successRate ?? 0;

  const cards = [
    {
      key: 'requests',
      title: t('dashboard.kpi.requests'),
      icon: Activity,
      value: formatExactNumber(overview?.totalRequests || 0),
      description: `${formatExactNumber(overview?.failedRequests || 0)} ${t('dashboard.stats.failedRequests')}`,
    },
    {
      key: 'tokens',
      title: t('dashboard.kpi.tokens'),
      icon: BarChart4,
      value: formatExactNumber(overview?.totalTokens || 0),
      description: `${formatExactNumber(overview?.totalInputTokens || 0)} ${t('dashboard.stats.input')} / ${formatExactNumber(overview?.totalOutputTokens || 0)} ${t('dashboard.stats.output')}`,
    },
    {
      key: 'cost',
      title: t('dashboard.kpi.cost'),
      icon: DollarSign,
      value: formatCurrency(overview?.totalCost || 0),
      description: null,
    },
    {
      key: 'successRate',
      title: t('dashboard.kpi.successRate'),
      icon: ShieldCheck,
      value: `${successRate.toFixed(1)}%`,
      description: null,
    },
  ];

  if (isLoading) {
    return (
      <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-4'>
        {cards.map((card) => (
          <Card key={card.key}>
            <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
              <Skeleton className='h-4 w-[120px]' />
              <Skeleton className='h-4 w-4' />
            </CardHeader>
            <CardContent>
              <Skeleton className='h-8 w-[80px]' />
              <Skeleton className='mt-2 h-3 w-[140px]' />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  return (
    <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-4'>
      {cards.map((card) => (
        <Card key={card.key} className='hover-card'>
          <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
            <div className='flex items-center gap-2'>
              <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
                <card.icon className='h-4 w-4' />
              </div>
              <CardTitle className='text-sm font-medium'>{card.title}</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <div className='font-mono text-3xl font-bold'>{card.value}</div>
            {card.key === 'successRate' ? (
              <Progress value={successRate} className='mt-3 h-2' />
            ) : (
              card.description && <p className='mt-1 text-xs text-muted-foreground'>{card.description}</p>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
