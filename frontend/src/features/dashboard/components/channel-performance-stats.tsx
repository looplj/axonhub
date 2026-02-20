'use client';

import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CartesianGrid, ResponsiveContainer, XAxis, YAxis, Tooltip, Line, LineChart, Area } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { formatDuration } from '@/utils/format-duration';
import { Skeleton } from '@/components/ui/skeleton';
import { CardDescription } from '@/components/ui/card';
import { useGeneralSettings } from '../../system/data/system';
import { useChannelPerformanceStats } from '../data/dashboard';

const COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
  'var(--chart-6)',
  'var(--chart-7)',
  'var(--chart-8)',
  'var(--chart-9)',
  'var(--chart-10)',
];

interface TooltipProps {
  active?: boolean;
  payload?: Array<{
    dataKey: string;
    value: number;
    name: string;
    color: string;
  }>;
  label?: string;
}

function PerformanceTooltip({ active, payload, label }: TooltipProps) {
  const { t } = useTranslation();

  if (!active || !payload || payload.length === 0) return null;

  const channelData = payload
    .filter((item) => !item.dataKey.includes('-ttft') && item.value > 0)
    .map((item) => {
      const ttftItem = payload.find((p) => p.dataKey === `${item.dataKey}-ttft`);
      return {
        channelId: item.dataKey,
        throughput: item.value,
        ttft: ttftItem?.value ?? 0,
        color: item.color,
      };
    })
    .sort((a, b) => a.channelId.localeCompare(b.channelId));

  if (channelData.length === 0) return null;

  return (
    <div
      className='rounded-md border bg-background p-3 shadow-md'
      style={{ fontSize: '12px' }}
    >
      <div className='mb-2 font-medium text-foreground'>{label}</div>
      <div className='space-y-2'>
        {channelData.map((item) => (
          <div key={item.channelId}>
            <div className='flex items-center gap-2'>
              <span
                className='h-2 w-2 rounded-full'
                style={{ backgroundColor: item.color }}
              />
              <span className='truncate font-medium text-foreground'>
                {item.channelId}
              </span>
            </div>
            <div className='ml-4 text-muted-foreground'>
              {formatNumber(item.throughput)} {t('dashboard.stats.throughput')} · TTFT{' '}
              {formatDuration(item.ttft)}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export function ChannelPerformanceStats() {
  const { t, i18n } = useTranslation();
  const { data: performanceStats, isLoading: isStatsLoading, error } = useChannelPerformanceStats();
  const { data: generalSettings, isLoading: isSettingsLoading } = useGeneralSettings();
  const [activeSeries, setActiveSeries] = useState<string | null>(null);

  const isLoading = isStatsLoading || isSettingsLoading;

  const timezone = generalSettings?.timezone || 'UTC';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const safeStats = performanceStats ?? [];

  const { dates, topChannels, legendItems } = useMemo(() => {
    const uniqueDates = [...new Set(safeStats.map((stat) => stat.date))].sort();
    const cStats = safeStats.reduce((acc, stat) => {
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

    const tChannels = Object.entries(cStats)
      .map(([channelId, stats]) => ({
        channelId,
        avgThroughput: stats.count > 0 ? stats.totalThroughput / stats.count : 0,
        totalRequests: stats.totalRequests,
      }))
      .sort((a, b) => b.avgThroughput - a.avgThroughput)
      .slice(0, 10)
      .sort((a, b) => a.channelId.localeCompare(b.channelId))
      .map((c) => c.channelId);

    const lItems = tChannels.map((channelId, index) => {
      const stats = safeStats.filter((stat) => stat.channelId === channelId);
      const avgThroughput =
        stats.reduce((sum, stat) => sum + (stat.throughput ?? 0), 0) / (stats.length || 1);
      const avgTtft =
        stats.reduce((sum, stat) => sum + (stat.ttftMs ?? 0), 0) / (stats.length || 1);

      return {
        id: channelId,
        name: channelId,
        color: COLORS[index % COLORS.length],
        avgThroughput,
        avgTtft,
      };
    });

    return { dates: uniqueDates, topChannels: tChannels, legendItems: lItems };
  }, [safeStats]);

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

  if (!performanceStats || performanceStats.length === 0 || topChannels.length === 0) {
    return (
      <div className='flex h-[350px] items-center justify-center text-muted-foreground'>
        {t('dashboard.charts.noChannelData')}
      </div>
    );
  }

  // Transform data for the chart
  const chartData = dates.map((date) => {
    const [year, month, day] = date.split('-').map(Number);
    // Use Date.UTC to avoid local timezone shifts when formatting
    const dateObj = new Date(Date.UTC(year, month - 1, day));
    const dataPoint: Record<string, string | number> = {
      name: dateObj.toLocaleDateString(locale, {
        month: '2-digit',
        day: '2-digit',
        timeZone: timezone,
      }),
    };

    topChannels.forEach((channelId) => {
      const stat = safeStats.find((s) => s.date === date && s.channelId === channelId);
      dataPoint[channelId] = stat?.throughput ?? 0;
      dataPoint[`${channelId}-ttft`] = stat?.ttftMs ?? 0;
    });

    return dataPoint;
  });

  // Calculate max throughput for Y-axis domain
  const maxThroughput = Math.max(
    ...safeStats
      .filter((s) => s.throughput != null && topChannels.includes(s.channelId))
      .map((s) => s.throughput!),
    0
  );
  const throughputMax = Math.max(10, Math.ceil(maxThroughput * 1.1));

  const maxTtft = Math.max(
    ...safeStats
      .filter((s) => s.ttftMs != null && topChannels.includes(s.channelId))
      .map((s) => s.ttftMs!),
    0
  );
  const ttftMax = Math.max(10, Math.ceil(maxTtft * 1.1));

  const visibleChannels = activeSeries ? [activeSeries] : topChannels;

  return (
    <div>
      <CardDescription className='mb-1'>
        {t('dashboard.charts.performanceDescription')}
      </CardDescription>
      <ResponsiveContainer width='100%' height={350}>
        <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
          <defs>
            {topChannels.map((channelId, index) => (
              <linearGradient
                key={`${channelId}-fill`}
                id={`channel-throughput-${index}`}
                x1='0'
                y1='0'
                x2='0'
                y2='1'
              >
                <stop offset='5%' stopColor={COLORS[index % COLORS.length]} stopOpacity={0.3} />
                <stop offset='95%' stopColor={COLORS[index % COLORS.length]} stopOpacity={0} />
              </linearGradient>
            ))}
          </defs>
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
          <YAxis
            yAxisId='ttft'
            orientation='right'
            stroke='var(--chart-3)'
            fontSize={12}
            tickLine={true}
            axisLine={true}
            domain={[0, ttftMax]}
            tickFormatter={(value) => formatNumber(value)}
            width={50}
            tickMargin={8}
          />
          <Tooltip content={<PerformanceTooltip />} />
          {topChannels.map((channelId, index) => {
            const color = COLORS[index % COLORS.length];
            const isActive = !activeSeries || activeSeries === channelId;
            const opacity = isActive ? 1 : 0.2;
            return (
              <Area
                key={channelId}
                type='monotone'
                dataKey={channelId}
                name={channelId}
                stroke={color}
                strokeWidth={2}
                fill={`url(#channel-throughput-${index})`}
                fillOpacity={1}
                dot={false}
                activeDot={{ r: 4 }}
                connectNulls={false}
                strokeOpacity={opacity}
                hide={!visibleChannels.includes(channelId)}
              />
            );
          })}
          {topChannels.map((channelId, index) => {
            const color = COLORS[index % COLORS.length];
            const isActive = !activeSeries || activeSeries === channelId;
            const opacity = isActive ? 0.35 : 0.1;
            return (
              <Line
                key={`${channelId}-ttft`}
                yAxisId='ttft'
                type='monotone'
                dataKey={`${channelId}-ttft`}
                name={`${channelId} TTFT`}
                stroke={color}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 3 }}
                connectNulls={false}
                strokeOpacity={opacity}
                hide={!visibleChannels.includes(channelId)}
                isAnimationActive={false}
              />
            );
          })}
        </LineChart>
      </ResponsiveContainer>
      <div className='mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
        {legendItems.map((item) => {
          const isActive = !activeSeries || activeSeries === item.id;
          return (
            <button
              type='button'
              key={item.id}
              onClick={() => setActiveSeries((current) => (current === item.id ? null : item.id))}
              className={`flex items-center justify-between gap-2 rounded-md border px-2 py-1.5 text-left text-sm transition ${
                isActive ? 'border-primary/40 bg-primary/5 text-foreground' : 'border-border text-muted-foreground'
              }`}
            >
              <span className='flex min-w-0 items-center gap-2'>
                <span className='h-2.5 w-2.5 rounded-full' style={{ backgroundColor: item.color }} />
                <span className='truncate font-medium'>{item.name}</span>
              </span>
              <span className='text-xs text-muted-foreground tabular-nums'>
                {formatNumber(item.avgThroughput)} {t('dashboard.stats.throughput')} · TTFT {formatDuration(item.avgTtft)}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
