import { useTranslation } from 'react-i18next';
import { Area, AreaChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { Card, CardAction, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import { formatCurrencyTick } from '@/features/analytics/utils/format-currency';
import type { AnalyticsDailyStat } from '@/features/analytics/data/analytics';

interface DailyOverviewChartProps {
  data: AnalyticsDailyStat[];
  isLoading: boolean;
  currencyCode: string;
  isOverviewLoading: boolean;
  rangeSummary: Array<{ key: string; label: string; value: string }>;
  badge?: React.ReactNode;
}

interface TooltipEntry {
  name?: string | number;
  value?: number | string;
  color?: string;
  dataKey?: string | number;
}

/** Daily overview: how request volume, token volume and cost move over the selected
 * range. The three series measure unrelated magnitudes, so each keeps its own axis and
 * is drawn as an independent area — nothing here is meant to be added up. */
export function DailyOverviewChart({
  data,
  isLoading,
  currencyCode,
  isOverviewLoading,
  rangeSummary,
  badge,
}: DailyOverviewChartProps) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const seriesLabels: Record<string, string> = {
    requests: t('analytics.overview.totalRequests'),
    tokens: t('analytics.overview.totalTokens'),
    cost: t('analytics.overview.totalCost'),
  };

  const chartData = data.map((stat) => {
    const [year, month, day] = stat.date.split('-').map(Number);
    const date = new Date(Date.UTC(year, month - 1, day));
    return {
      name: date.toLocaleDateString(locale, {
        month: '2-digit',
        day: '2-digit',
        timeZone: 'UTC',
      }),
      requests: stat.requestCount,
      tokens: stat.totalTokens,
      cost: stat.cost,
    };
  });

  if (isLoading) {
    return (
      <Card className='hover-card flex h-full flex-col'>
        <CardHeader>
          <div className='space-y-1'>
            <CardTitle>{t('dashboard.charts.dailyRequestOverview')}</CardTitle>
            <div className='text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs'>
              {rangeSummary.map((item) => (
                <Skeleton key={item.key} className='h-3 w-24' />
              ))}
            </div>
          </div>
          {badge && <CardAction>{badge}</CardAction>}
        </CardHeader>
        <CardContent>
          <Skeleton className='h-[350px] w-full' />
        </CardContent>
      </Card>
    );
  }

  const maxRequests = Math.max(...chartData.map((d) => d.requests), 0);
  const maxTokens = Math.max(...chartData.map((d) => d.tokens), 0);
  const maxCost = Math.max(...chartData.map((d) => d.cost), 0);

  const requestsMax = Math.max(10, Math.ceil(maxRequests * 1.1));
  const tokensMax = Math.max(1000, Math.ceil(maxTokens * 1.1));
  const costMax = Math.max(0.1, maxCost * 1.1);

  return (
    <Card className='hover-card flex h-full flex-col'>
      <CardHeader>
        <div className='space-y-1'>
          <CardTitle>{t('dashboard.charts.dailyRequestOverview')}</CardTitle>
          <div className='text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs'>
            {isOverviewLoading
              ? rangeSummary.map((item) => <Skeleton key={item.key} className='h-3 w-24' />)
              : rangeSummary.map((item) => (
                  <span key={item.key}>
                    {item.label}: <span className='text-foreground font-mono font-medium tabular-nums'>{item.value}</span>
                  </span>
                ))}
          </div>
        </div>
        {badge && <CardAction>{badge}</CardAction>}
      </CardHeader>
      <CardContent className='pl-2'>
        <ResponsiveContainer width='100%' height={350}>
          <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id='dailyOverviewRequests' x1='0' y1='0' x2='0' y2='1'>
                <stop offset='5%' stopColor='var(--primary)' stopOpacity={0.3} />
                <stop offset='95%' stopColor='var(--primary)' stopOpacity={0} />
              </linearGradient>
              <linearGradient id='dailyOverviewTokens' x1='0' y1='0' x2='0' y2='1'>
                <stop offset='5%' stopColor='var(--chart-2)' stopOpacity={0.2} />
                <stop offset='95%' stopColor='var(--chart-2)' stopOpacity={0} />
              </linearGradient>
              <linearGradient id='dailyOverviewCost' x1='0' y1='0' x2='0' y2='1'>
                <stop offset='5%' stopColor='var(--chart-3)' stopOpacity={0.4} />
                <stop offset='95%' stopColor='var(--chart-3)' stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
            <XAxis dataKey='name' stroke='var(--muted-foreground)' fontSize={12} tickLine={true} axisLine={true} padding={{ right: 24 }} />
            <YAxis
              yAxisId='left'
              stroke='var(--chart-1)'
              fontSize={12}
              tickLine={true}
              axisLine={true}
              domain={[0, requestsMax]}
              tickFormatter={(value) => formatNumber(value)}
              width={40}
              tickMargin={8}
            />
            <YAxis
              yAxisId='tokens'
              orientation='right'
              stroke='var(--chart-2)'
              fontSize={12}
              tickLine={true}
              axisLine={true}
              domain={[0, tokensMax]}
              tickFormatter={(value) => formatNumber(value)}
              width={40}
              tickMargin={8}
            />
            <YAxis
              yAxisId='cost'
              orientation='right'
              stroke='var(--chart-3)'
              fontSize={12}
              tickLine={true}
              axisLine={true}
              domain={[0, costMax]}
              tickFormatter={(value) => formatCurrencyTick(value, currencyCode)}
              width={60}
              tickMargin={8}
            />
            <Tooltip
              content={({ active, payload, label }: { active?: boolean; payload?: TooltipEntry[]; label?: string }) => {
                if (!active || !payload || payload.length === 0) return null;

                return (
                  <div className='rounded-md border bg-background p-2 shadow-md' style={{ fontSize: '12px' }}>
                    <p className='mb-1 font-medium'>{label}</p>
                    {payload.map((entry, index) => (
                      <p key={index} className='flex justify-between gap-4' style={{ color: entry.color, padding: '2px 0' }}>
                        <span>{entry.name}</span>
                        <span className='font-medium'>
                          {entry.dataKey === 'cost'
                            ? formatCurrencyTick(Number(entry.value), currencyCode)
                            : formatNumber(Number(entry.value))}
                        </span>
                      </p>
                    ))}
                  </div>
                );
              }}
            />
            <Legend verticalAlign='top' height={36} formatter={(value) => seriesLabels[String(value)] ?? value} />
            <Area
              yAxisId='left'
              type='monotone'
              dataKey='requests'
              name={seriesLabels.requests}
              stroke='var(--chart-1)'
              strokeWidth={2}
              fillOpacity={1}
              fill='url(#dailyOverviewRequests)'
              dot={false}
              activeDot={{ r: 5 }}
              isAnimationActive={false}
            />
            <Area
              yAxisId='tokens'
              type='monotone'
              dataKey='tokens'
              name={seriesLabels.tokens}
              stroke='var(--chart-2)'
              strokeWidth={2}
              fillOpacity={1}
              fill='url(#dailyOverviewTokens)'
              dot={false}
              activeDot={{ r: 4 }}
              isAnimationActive={false}
            />
            <Area
              yAxisId='cost'
              type='monotone'
              dataKey='cost'
              name={seriesLabels.cost}
              stroke='var(--chart-3)'
              strokeWidth={2}
              fillOpacity={1}
              fill='url(#dailyOverviewCost)'
              dot={false}
              activeDot={{ r: 4 }}
              isAnimationActive={false}
            />
          </AreaChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
