import { useTranslation } from 'react-i18next';
import {
  ComposedChart,
  Bar,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import { formatCurrencySimple, formatCurrencyTick } from '../utils/format-currency';

function formatExactNumber(value: number): string {
  return Math.round(value).toLocaleString();
}
import type { AnalyticsDailyStat } from '../data/analytics';

interface CombinedTrendChartProps {
  data: AnalyticsDailyStat[];
  isLoading: boolean;
  currencyCode: string;
}

export function CombinedTrendChart({ data, isLoading, currencyCode }: CombinedTrendChartProps) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const chartData = data.map((stat) => {
    const [year, month, day] = stat.date.split('-').map(Number);
    const date = new Date(Date.UTC(year, month - 1, day));
    return {
      name: date.toLocaleDateString(locale, {
        month: '2-digit',
        day: '2-digit',
        timeZone: 'UTC',
      }),
      cachedInput: stat.cachedInputTokens,
      uncachedInput: stat.uncachedInputTokens,
      output: stat.outputTokens,
      totalTokens: stat.totalTokens,
      requests: stat.requestCount,
      cost: stat.cost,
    };
  });

  if (isLoading) {
    return (
      <Card className='hover-card'>
        <CardHeader>
          <CardTitle>{t('analytics.chart.trendTitle')}</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className='h-[350px] w-full' />
        </CardContent>
      </Card>
    );
  }

  const maxTokens = Math.max(...chartData.map((d) => d.cachedInput + d.uncachedInput + d.output), 0);
  const maxRequests = Math.max(...chartData.map((d) => d.requests), 0);
  const maxCost = Math.max(...chartData.map((d) => d.cost), 0);

  const tokensMax = Math.max(1000, Math.ceil(maxTokens * 1.1));
  const requestsMax = Math.max(10, Math.ceil(maxRequests * 1.1));
  const costMax = Math.max(0.1, maxCost * 1.1);

  return (
    <Card className='hover-card'>
      <CardHeader>
        <CardTitle>{t('analytics.chart.trendTitle')}</CardTitle>
      </CardHeader>
      <CardContent className='pl-2'>
        <ResponsiveContainer width='100%' height={350}>
          <ComposedChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
            <XAxis
              dataKey='name'
              stroke='var(--muted-foreground)'
              fontSize={12}
              tickLine={true}
              axisLine={true}
              padding={{ right: 24 }}
            />
            <YAxis
              yAxisId='tokens'
              stroke='var(--chart-1)'
              fontSize={12}
              tickLine={true}
              axisLine={true}
              domain={[0, tokensMax]}
              tickFormatter={(value) => formatNumber(value)}
              width={60}
              tickMargin={8}
            />
            <YAxis
              yAxisId='requests'
              orientation='right'
              stroke='var(--chart-4)'
              fontSize={12}
              tickLine={true}
              axisLine={true}
              domain={[0, requestsMax]}
              tickFormatter={(value) => formatNumber(value)}
              width={40}
              tickMargin={8}
            />
            <YAxis
              yAxisId='cost'
              orientation='right'
              stroke='var(--chart-5)'
              fontSize={12}
              tickLine={true}
              axisLine={true}
              domain={[0, costMax]}
              tickFormatter={(value) => formatCurrencyTick(value, currencyCode)}
              width={60}
              tickMargin={8}
            />
            <Tooltip
              content={({ active, payload, label }: { active?: boolean; payload?: Array<{ name?: string; value?: number | string; color?: string; payload?: Record<string, number> }>; label?: string }) => {
                if (!active || !payload || payload.length === 0) return null;
                const order = [
                  t('analytics.chart.cachedInput'),
                  t('analytics.chart.uncachedInput'),
                  t('analytics.chart.outputTokens'),
                  t('analytics.chart.totalTokens'),
                  t('analytics.chart.requestCount'),
                  t('analytics.chart.cost'),
                ];
                const sorted = [...payload].sort((a, b) => order.indexOf(String(a.name)) - order.indexOf(String(b.name)));
                const raw = payload[0]?.payload as Record<string, number> | undefined;
                const totalTokens = raw?.totalTokens ?? 0;
                return (
                  <div className='rounded-md border bg-background p-2 shadow-md' style={{ fontSize: '12px' }}>
                    <p className='mb-1 font-medium'>{label}</p>
                    {sorted.map((entry, index) => (
                      <p key={index} className='flex justify-between gap-4' style={{ color: entry.color, padding: '2px 0' }}>
                        <span>{entry.name}</span>
                        <span className='font-medium'>
                          {entry.name === t('analytics.chart.cost')
                            ? formatCurrencySimple(Number(entry.value), currencyCode)
                            : formatExactNumber(Number(entry.value))}
                        </span>
                      </p>
                    ))}
                    <p className='flex justify-between gap-4 border-t pt-1 font-medium' style={{ padding: '2px 0', color: 'var(--chart-6)' }}>
                      <span>{t('analytics.chart.totalTokens')}</span>
                      <span>{formatExactNumber(totalTokens)}</span>
                    </p>
                    {(() => {
                      const inputTotal = (raw?.cachedInput ?? 0) + (raw?.uncachedInput ?? 0);
                      const hitRate = inputTotal > 0 ? (((raw?.cachedInput ?? 0) / inputTotal) * 100).toFixed(1) : '0';
                      return (
                        <p className='flex justify-between gap-4' style={{ padding: '2px 0', color: 'var(--chart-3)' }}>
                          <span>{t('analytics.chart.cacheHitRate')}</span>
                          <span className='font-medium'>{hitRate}%</span>
                        </p>
                      );
                    })()}
                  </div>
                );
              }}
            />
            <Legend
              verticalAlign='top'
              height={36}
              itemSorter={null}
              payload={[
                { value: t('analytics.chart.cachedInput'), type: 'square' as const, color: 'var(--chart-1)' },
                { value: t('analytics.chart.uncachedInput'), type: 'square' as const, color: 'var(--chart-2)' },
                { value: t('analytics.chart.outputTokens'), type: 'square' as const, color: 'var(--chart-3)' },
                { value: t('analytics.chart.requestCount'), type: 'line' as const, color: 'var(--chart-4)' },
                { value: t('analytics.chart.cost'), type: 'line' as const, color: 'var(--chart-5)' },
              ]}
            />
            <Bar yAxisId='tokens' dataKey='cachedInput' name={t('analytics.chart.cachedInput')} stackId='tokens' fill='var(--chart-1)' isAnimationActive={false} />
            <Bar yAxisId='tokens' dataKey='uncachedInput' name={t('analytics.chart.uncachedInput')} stackId='tokens' fill='var(--chart-2)' isAnimationActive={false} />
            <Bar yAxisId='tokens' dataKey='output' name={t('analytics.chart.outputTokens')} stackId='tokens' fill='var(--chart-3)' radius={[4, 4, 0, 0]} isAnimationActive={false} />
            <Line yAxisId='requests' type='monotone' dataKey='requests' name={t('analytics.chart.requestCount')} stroke='var(--chart-4)' strokeWidth={2} dot={false} activeDot={{ r: 5 }} />
            <Line yAxisId='cost' type='monotone' dataKey='cost' name={t('analytics.chart.cost')} stroke='var(--chart-5)' strokeWidth={2} dot={false} activeDot={{ r: 4 }} />
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
