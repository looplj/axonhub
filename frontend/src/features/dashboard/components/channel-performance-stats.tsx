'use client';

import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { CartesianGrid, ResponsiveContainer, XAxis, YAxis, Tooltip, Line, LineChart, Legend } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { Skeleton } from '@/components/ui/skeleton';
import { useGeneralSettings } from '../../system/data/system';
import { useChannelPerformanceStats } from '../data/dashboard';

const COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
];

export function ChannelPerformanceStats() {
  const { t, i18n } = useTranslation();
  const { data: performanceStats, isLoading: isStatsLoading, error } = useChannelPerformanceStats();
  const { data: generalSettings, isLoading: isSettingsLoading } = useGeneralSettings();

  const isLoading = isStatsLoading || isSettingsLoading;

  const timezone = generalSettings?.timezone || 'UTC';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const tooltipFormatter = useCallback(
    (value: number | string, name: string) => {
      return [formatNumber(Number(value)), name];
    },
    []
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
        {t('dashboard.charts.errorLoadingChannelData')} {error.message}
      </div>
    );
  }

  if (!performanceStats || performanceStats.length === 0) {
    return (
      <div className='flex h-[350px] items-center justify-center text-muted-foreground'>
        {t('dashboard.charts.noChannelData')}
      </div>
    );
  }

  const dates = [...new Set(performanceStats.map((stat) => stat.date))].sort();
  const channelStats = performanceStats.reduce((acc, stat) => {
    if (!acc[stat.channelId]) {
      acc[stat.channelId] = { totalRequests: 0, totalThroughput: 0, count: 0 };
    }
    if (stat.throughput != null) {
      acc[stat.channelId].totalRequests += stat.requestCount;
      acc[stat.channelId].totalThroughput += stat.throughput * stat.requestCount;
      acc[stat.channelId].count += 1;
    }
    return acc;
  }, {} as Record<string, { totalRequests: number; totalThroughput: number; count: number }>);

  const topChannels = Object.entries(channelStats)
    .map(([channelId, stats]) => ({
      channelId,
      avgThroughput: stats.count > 0 ? stats.totalThroughput / stats.count : 0,
      totalRequests: stats.totalRequests,
    }))
    .sort((a, b) => b.totalRequests - a.totalRequests)
    .slice(0, 10)
    .map((c) => c.channelId);

  if (topChannels.length === 0) {
    return (
      <div className='flex h-[350px] items-center justify-center text-muted-foreground'>
        {t('dashboard.charts.noChannelData')}
      </div>
    );
  }

  // Transform data for the chart
  const chartData = dates.map((date) => {
    const [year, month, day] = date.split('-').map(Number);
    const dateObj = new Date(year, month - 1, day);
    const dataPoint: Record<string, string | number> = {
      name: dateObj.toLocaleDateString(locale, {
        month: '2-digit',
        day: '2-digit',
        timeZone: timezone,
      }),
    };

    topChannels.forEach((channelId) => {
      const stat = performanceStats.find((s) => s.date === date && s.channelId === channelId);
      dataPoint[channelId] = stat?.throughput ?? null;
    });

    return dataPoint;
  });

  // Calculate max throughput for Y-axis domain
  const maxThroughput = Math.max(
    ...performanceStats
      .filter((s) => s.throughput != null && topChannels.includes(s.channelId))
      .map((s) => s.throughput!),
    0
  );
  const throughputMax = Math.max(10, Math.ceil(maxThroughput * 1.1));

  return (
    <ResponsiveContainer width='100%' height={350}>
      <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
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
          stroke='var(--muted-foreground)'
          fontSize={12}
          tickLine={true}
          axisLine={true}
          domain={[0, throughputMax]}
          tickFormatter={(value) => formatNumber(value)}
          width={40}
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
        {topChannels.map((channelId, index) => (
          <Line
            key={channelId}
            type='monotone'
            dataKey={channelId}
            name={channelId}
            stroke={COLORS[index % 5]}
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4 }}
            connectNulls={false}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}
