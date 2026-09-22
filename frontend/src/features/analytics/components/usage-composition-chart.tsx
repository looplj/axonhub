import { useTranslation } from 'react-i18next';
import { ComposedChart, Bar, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import type { AnalyticsDailyStat } from '../data/analytics';

function formatExactNumber(value: number): string {
  return Math.round(value).toLocaleString();
}

interface UsageCompositionChartProps {
  data: AnalyticsDailyStat[];
  isLoading: boolean;
}

/** Token composition over time: how the same daily token total splits between cached
 * input, uncached input and output, with cache hit rate showing how much of the input
 * traffic the cache absorbs. The three bars are parts of one whole, so they stack and
 * share a single axis; the hit rate is a ratio, so it gets its own percentage axis. */
export function UsageCompositionChart({ data, isLoading }: UsageCompositionChartProps) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const chartData = data.map((stat) => {
    const [year, month, day] = stat.date.split('-').map(Number);
    const date = new Date(Date.UTC(year, month - 1, day));
    const inputTotal = stat.cachedInputTokens + stat.uncachedInputTokens;
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
      cacheHitRate: inputTotal > 0 ? (stat.cachedInputTokens / inputTotal) * 100 : 0,
    };
  });

  if (isLoading) {
    return (
      <Card className='hover-card flex h-full flex-col'>
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
  const tokensMax = Math.max(1000, Math.ceil(maxTokens * 1.1));

  return (
    <Card className='hover-card flex h-full flex-col'>
      <CardHeader>
        <div className='space-y-1'>
          <CardTitle>{t('analytics.chart.trendTitle')}</CardTitle>
          <p className='text-muted-foreground text-xs'>{t('analytics.chart.trendDescription')}</p>
        </div>
      </CardHeader>
      <CardContent className='pl-2'>
        <ResponsiveContainer width='100%' height={350}>
          <ComposedChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
            <XAxis dataKey='name' stroke='var(--muted-foreground)' fontSize={12} tickLine={true} axisLine={true} padding={{ right: 24 }} />
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
              yAxisId='hitRate'
              orientation='right'
              stroke='var(--chart-4)'
              fontSize={12}
              tickLine={true}
              axisLine={true}
              domain={[0, 100]}
              tickFormatter={(value) => `${value}%`}
              width={48}
              tickMargin={8}
            />
            <Tooltip
              content={({ active, payload, label }: { active?: boolean; payload?: Array<{ name?: string; value?: number | string; color?: string; payload?: Record<string, number> }>; label?: string }) => {
                if (!active || !payload || payload.length === 0) return null;
                const order = [
                  t('analytics.chart.cachedInput'),
                  t('analytics.chart.uncachedInput'),
                  t('analytics.chart.outputTokens'),
                  t('analytics.chart.cacheHitRate'),
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
                          {entry.name === t('analytics.chart.cacheHitRate') ? `${Number(entry.value).toFixed(1)}%` : formatExactNumber(Number(entry.value))}
                        </span>
                      </p>
                    ))}
                    <p className='flex justify-between gap-4 border-t pt-1 font-medium' style={{ padding: '2px 0', color: 'var(--chart-6)' }}>
                      <span>{t('analytics.chart.totalTokens')}</span>
                      <span>{formatExactNumber(totalTokens)}</span>
                    </p>
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
                { value: t('analytics.chart.cacheHitRate'), type: 'line' as const, color: 'var(--chart-4)' },
              ]}
            />
            <Bar yAxisId='tokens' dataKey='cachedInput' name={t('analytics.chart.cachedInput')} stackId='tokens' fill='var(--chart-1)' isAnimationActive={false} />
            <Bar yAxisId='tokens' dataKey='uncachedInput' name={t('analytics.chart.uncachedInput')} stackId='tokens' fill='var(--chart-2)' isAnimationActive={false} />
            <Bar yAxisId='tokens' dataKey='output' name={t('analytics.chart.outputTokens')} stackId='tokens' fill='var(--chart-3)' radius={[4, 4, 0, 0]} isAnimationActive={false} />
            <Line
              yAxisId='hitRate'
              type='monotone'
              dataKey='cacheHitRate'
              name={t('analytics.chart.cacheHitRate')}
              stroke='var(--chart-4)'
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 5 }}
              isAnimationActive={false}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
