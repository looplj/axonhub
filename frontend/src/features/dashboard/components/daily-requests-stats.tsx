'use client';

import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { CartesianGrid, ResponsiveContainer, XAxis, YAxis, Tooltip, Area, AreaChart, Legend } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { Skeleton } from '@/components/ui/skeleton';
import { useGeneralSettings } from '../../system/data/system';
import { useDailyRequestStats } from '../data/dashboard';

export function DailyRequestStats() {
  const { t, i18n } = useTranslation();
  const { data: dailyStats, isLoading: isStatsLoading, error } = useDailyRequestStats();
  const { data: generalSettings, isLoading: isSettingsLoading } = useGeneralSettings();

  const isLoading = isStatsLoading || isSettingsLoading;

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const formatCurrency = useCallback(
    (val: number, fractionDigits: number) =>
      t('currencies.format', {
        val,
        currency: currencyCode,
        locale,
        minimumFractionDigits: fractionDigits,
        maximumFractionDigits: fractionDigits,
      }),
    [currencyCode, locale, t]
  );

  const formatCostTick = useCallback((value: number | string) => formatCurrency(Number(value), 0), [formatCurrency]);

  const tooltipFormatter = useCallback(
    (value: number | string, name: string) => {
      if (name === t('dashboard.stats.totalCost')) {
        return [formatCurrency(Number(value), 0), name];
      }
      return [formatNumber(Number(value)), name];
    },
    [formatCurrency, t]
  );

  if (isLoading) {
    return (
      <div className='flex h-[350px] items-center justify-center'>
        <Skeleton className='h-full w-full' />
      </div>
    );
  }

  if (error) {
    return (
      <div className='flex h-[350px] items-center justify-center text-red-500'>
        {t('dashboard.charts.errorLoadingChart')} {error.message}
      </div>
    );
  }

  // Transform data for the chart
  const chartData =
    dailyStats?.map((stat) => ({
      name: new Date(stat.date).toLocaleDateString(i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US', {
        month: '2-digit',
        day: '2-digit',
      }),
      requests: stat.count,
      tokens: stat.tokens,
      cost: stat.cost,
    })) || [];

  // Calculate max values for Y-axis domains
  const maxRequests = Math.max(...chartData.map((d) => d.requests), 0);
  const maxTokens = Math.max(...chartData.map((d) => d.tokens), 0);
  const maxCost = Math.max(...chartData.map((d) => d.cost), 0);

  const requestsMax = Math.max(10, Math.ceil(maxRequests * 1.1));
  const tokensMax = Math.max(1000, Math.ceil(maxTokens * 1.1));
  const costMax = Math.max(0.1, maxCost * 1.1);

  return (
    <ResponsiveContainer width='100%' height={350}>
      <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
        <defs>
          <linearGradient id='colorRequests' x1='0' y1='0' x2='0' y2='1'>
            <stop offset='5%' stopColor='var(--primary)' stopOpacity={0.3} />
            <stop offset='95%' stopColor='var(--primary)' stopOpacity={0} />
          </linearGradient>
          <linearGradient id='colorTokens' x1='0' y1='0' x2='0' y2='1'>
            <stop offset='5%' stopColor='var(--chart-2)' stopOpacity={0.2} />
            <stop offset='95%' stopColor='var(--chart-2)' stopOpacity={0} />
          </linearGradient>
          <linearGradient id='colorCost' x1='0' y1='0' x2='0' y2='1'>
            <stop offset='5%' stopColor='var(--chart-3)' stopOpacity={0.4} />
            <stop offset='95%' stopColor='var(--chart-3)' stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
        <XAxis
          dataKey='name'
          stroke='var(--muted-foreground)'
          fontSize={12}
          tickLine={true}
          axisLine={true}
          padding={{ right: 24 }}
        // padding={{ left: 16, right: 16 }}
        />
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
          tickFormatter={formatCostTick}
          width={60}
          tickMargin={8}
        />
        <Tooltip
          formatter={tooltipFormatter}
          contentStyle={{
            backgroundColor: 'var(--background)',
            borderColor: 'var(--border)',
            borderRadius: 'var(--radius)',
            fontSize: '12px',
          }}
          itemStyle={{ padding: '2px 0' }}
        />
        <Legend verticalAlign='top' height={36} />
        <Area
          yAxisId='left'
          type='monotone'
          dataKey='requests'
          name={t('dashboard.stats.requests')}
          stroke='var(--chart-1)'
          strokeWidth={2}
          fillOpacity={1}
          fill='url(#colorRequests)'
          dot={false}
          activeDot={{ r: 5 }}
        />
        <Area
          yAxisId='tokens'
          type='monotone'
          dataKey='tokens'
          name={t('dashboard.stats.totalTokens')}
          stroke='var(--chart-2)'
          strokeWidth={2}
          fillOpacity={1}
          fill='url(#colorTokens)'
          dot={false}
          activeDot={{ r: 4 }}
        />
        <Area
          yAxisId='cost'
          type='monotone'
          dataKey='cost'
          name={t('dashboard.stats.totalCost')}
          stroke='var(--chart-3)'
          strokeWidth={2}
          fillOpacity={1}
          fill='url(#colorCost)'
          dot={false}
          activeDot={{ r: 4 }}
        />
      </AreaChart>
    </ResponsiveContainer>
  );
}
