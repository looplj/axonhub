'use client';

import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipProps } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { Skeleton } from '@/components/ui/skeleton';
import { useTokensByAPIKey } from '../data/dashboard';

const TOKEN_COLORS = {
  input: 'var(--chart-1)',
  output: 'var(--chart-2)',
  cached: 'var(--chart-3)',
  reasoning: 'var(--chart-4)',
};

export function TokensByAPIKeyChart() {
  const { t } = useTranslation();
  const { data: tokenData, isLoading, error } = useTokensByAPIKey();

  if (isLoading) {
    return (
      <div className='flex h-[300px] items-center justify-center'>
        <Skeleton className='h-[250px] w-full rounded-md' />
      </div>
    );
  }

  if (error) {
    return (
      <div className='flex h-[300px] items-center justify-center'>
        <div className='text-sm text-red-500'>
          {t('dashboard.charts.errorLoadingTokenData')} {error.message}
        </div>
      </div>
    );
  }

  if (!tokenData || tokenData.length === 0) {
    return (
      <div className='flex h-[300px] items-center justify-center'>
        <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noTokenData')}</div>
      </div>
    );
  }

  // Prepare data for stacked bar chart
  const chartData = tokenData.map((item) => ({
    name: item.apiKeyName,
    inputTokens: item.inputTokens,
    outputTokens: item.outputTokens,
    cachedTokens: item.cachedTokens,
    reasoningTokens: item.reasoningTokens,
    totalTokens: item.totalTokens,
  }));

  const totalAllKeys = tokenData.reduce((sum, item) => sum + item.totalTokens, 0);

  type TokenTooltipProps = TooltipProps<number, string> & {
    payload?: Array<{
      payload: {
        name: string;
        inputTokens: number;
        outputTokens: number;
        cachedTokens: number;
        reasoningTokens: number;
        totalTokens: number;
      };
    }>;
  };

  const tooltipContent = (props: TokenTooltipProps) => {
    if (!props.active || !props.payload?.length) return null;

    const data = props.payload[0].payload;
    const percent = totalAllKeys ? ((data.totalTokens ?? 0) / totalAllKeys) * 100 : 0;

    return (
      <div className='bg-background/90 rounded-md border px-3 py-2 text-xs shadow-sm backdrop-blur'>
        <div className='text-foreground text-sm font-medium mb-1'>{data.name}</div>
        <div className='space-y-1'>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.inputTokens')}:</span>
            <span className='font-medium'>{formatNumber(data.inputTokens)}</span>
          </div>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.outputTokens')}:</span>
            <span className='font-medium'>{formatNumber(data.outputTokens)}</span>
          </div>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.cachedTokens')}:</span>
            <span className='font-medium'>{formatNumber(data.cachedTokens)}</span>
          </div>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.reasoningTokens')}:</span>
            <span className='font-medium'>{formatNumber(data.reasoningTokens)}</span>
          </div>
          <div className='border-t pt-1 flex justify-between gap-4'>
            <span className='text-foreground font-medium'>{t('dashboard.stats.totalTokens')}:</span>
            <span className='font-semibold'>{formatNumber(data.totalTokens)} ({percent.toFixed(1)}%)</span>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className='space-y-6'>
      <ResponsiveContainer width='100%' height={320}>
        <BarChart data={chartData} isAnimationActive={false}>
          <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
          <XAxis
            dataKey='name'
            tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
            tickLine={false}
            axisLine={false}
          />
          <YAxis
            tickLine={false}
            axisLine={false}
            width={60}
            tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
            tickFormatter={(value) => formatNumber(value)}
          />
          <Tooltip content={tooltipContent} cursor={{ fill: 'var(--muted)' }} />
          <Legend
            wrapperStyle={{ fontSize: '12px' }}
            iconType='circle'
          />
          <Bar
            dataKey='inputTokens'
            fill={TOKEN_COLORS.input}
            name={t('dashboard.stats.inputTokens')}
            radius={[6, 6, 0, 0]}
            isAnimationActive={false}
          />
          <Bar
            dataKey='outputTokens'
            fill={TOKEN_COLORS.output}
            name={t('dashboard.stats.outputTokens')}
            radius={[6, 6, 0, 0]}
            isAnimationActive={false}
          />
          <Bar
            dataKey='cachedTokens'
            fill={TOKEN_COLORS.cached}
            name={t('dashboard.stats.cachedTokens')}
            radius={[6, 6, 0, 0]}
            isAnimationActive={false}
          />
          <Bar
            dataKey='reasoningTokens'
            fill={TOKEN_COLORS.reasoning}
            name={t('dashboard.stats.reasoningTokens')}
            radius={[6, 6, 0, 0]}
            isAnimationActive={false}
          />
        </BarChart>
      </ResponsiveContainer>

      <div className='grid gap-4 sm:grid-cols-1'>
        {chartData.map((item, index) => {
          const percent = totalAllKeys ? (item.totalTokens / totalAllKeys) * 100 : 0;
          return (
            <div key={item.name} className='grid w-full grid-cols-[auto_1fr_auto] items-start gap-3'>
              <span className='text-muted-foreground w-8 text-right text-sm font-semibold tabular-nums'>
                {(index + 1).toString().padStart(2, '0')}.
              </span>
              <span className='text-foreground min-w-0 text-sm font-medium break-words'>{item.name}</span>
              <div className='text-right leading-tight'>
                <div className='text-foreground text-sm font-medium tabular-nums'>{formatNumber(item.totalTokens)}</div>
                <div className='text-muted-foreground text-xs tabular-nums'>{percent.toFixed(1)}%</div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
