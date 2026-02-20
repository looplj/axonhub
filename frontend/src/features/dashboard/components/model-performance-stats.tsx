'use client';

import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CartesianGrid, ResponsiveContainer, XAxis, YAxis, Tooltip, Line, LineChart, Area } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { formatDuration } from '@/utils/format-duration';
import { Skeleton } from '@/components/ui/skeleton';
import { CardDescription } from '@/components/ui/card';
import { useGeneralSettings } from '../../system/data/system';
import { useModelPerformanceStats } from '../data/dashboard';

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

  const modelData = payload
    .filter((item) => !item.dataKey.includes('-ttft') && item.value > 0)
    .map((item) => {
      const ttftItem = payload.find((p) => p.dataKey === `${item.dataKey}-ttft`);
      return {
        modelId: item.dataKey,
        throughput: item.value,
        ttft: ttftItem?.value ?? 0,
        color: item.color,
      };
    })
    .sort((a, b) => b.throughput - a.throughput);

  if (modelData.length === 0) return null;

  return (
    <div
      className='rounded-md border bg-background p-3 shadow-md'
      style={{ fontSize: '12px' }}
    >
      <div className='mb-2 font-medium text-foreground'>{label}</div>
      <div className='space-y-2'>
        {modelData.map((item) => (
          <div key={item.modelId}>
            <div className='flex items-center gap-2'>
              <span
                className='h-2 w-2 rounded-full'
                style={{ backgroundColor: item.color }}
              />
              <span className='truncate font-medium text-foreground'>
                {item.modelId}
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

export function ModelPerformanceStats() {
  const { t, i18n } = useTranslation();
  const { data: performanceStats, isLoading: isStatsLoading, error } = useModelPerformanceStats();
  const { data: generalSettings, isLoading: isSettingsLoading } = useGeneralSettings();
  const [activeSeries, setActiveSeries] = useState<string | null>(null);

  const isLoading = isStatsLoading || isSettingsLoading;

  const timezone = generalSettings?.timezone || 'UTC';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const safeStats = performanceStats ?? [];

  const { dates, topModels, legendItems } = useMemo(() => {
    const uniqueDates = [...new Set(safeStats.map((stat) => stat.date))].sort();
    const mStats = safeStats.reduce((acc, stat) => {
      if (!acc[stat.modelId]) {
        acc[stat.modelId] = { totalRequests: 0, totalThroughput: 0, count: 0 };
      }
      if (stat.throughput != null) {
        acc[stat.modelId].totalRequests += stat.requestCount;
        acc[stat.modelId].totalThroughput += stat.throughput * stat.requestCount;
        acc[stat.modelId].count += 1;
      }
      return acc;
    }, {} as Record<string, { totalRequests: number; totalThroughput: number; count: number }>);

    const tModels = Object.entries(mStats)
      .map(([modelId, stats]) => ({
        modelId,
        avgThroughput: stats.count > 0 ? stats.totalThroughput / stats.count : 0,
        totalRequests: stats.totalRequests,
      }))
      .sort((a, b) => b.avgThroughput - a.avgThroughput)
      .slice(0, 10)
      .sort((a, b) => a.modelId.localeCompare(b.modelId))
      .map((m) => m.modelId);

    const lItems = tModels.map((modelId, index) => {
      const stats = safeStats.filter((stat) => stat.modelId === modelId);
      const avgThroughput =
        stats.reduce((sum, stat) => sum + (stat.throughput ?? 0), 0) / (stats.length || 1);
      const avgTtft =
        stats.reduce((sum, stat) => sum + (stat.ttftMs ?? 0), 0) / (stats.length || 1);

      return {
        id: modelId,
        name: modelId,
        color: COLORS[index % COLORS.length],
        avgThroughput,
        avgTtft,
      };
    });

    return { dates: uniqueDates, topModels: tModels, legendItems: lItems };
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
        {t('dashboard.charts.errorLoadingModelData')} {error.message}
      </div>
    );
  }

  if (!performanceStats || performanceStats.length === 0 || topModels.length === 0) {
    return (
      <div className='flex h-[350px] items-center justify-center text-muted-foreground'>
        {t('dashboard.charts.noModelData')}
      </div>
    );
  }

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

    topModels.forEach((modelId) => {
      const stat = safeStats.find((s) => s.date === date && s.modelId === modelId);
      dataPoint[modelId] = stat?.throughput ?? 0;
      dataPoint[`${modelId}-ttft`] = stat?.ttftMs ?? 0;
    });

    return dataPoint;
  });

  const maxThroughput = Math.max(
    ...safeStats
      .filter((s) => s.throughput != null && topModels.includes(s.modelId))
      .map((s) => s.throughput!),
    0
  );
  const throughputMax = Math.max(10, Math.ceil(maxThroughput * 1.1));

  const maxTtft = Math.max(
    ...safeStats
      .filter((s) => s.ttftMs != null && topModels.includes(s.modelId))
      .map((s) => s.ttftMs!),
    0
  );
  const ttftMax = Math.max(10, Math.ceil(maxTtft * 1.1));

  const visibleModels = activeSeries ? [activeSeries] : topModels;

  return (
    <div>
      <CardDescription className='mb-1'>
        {t('dashboard.charts.performanceDescription')}
      </CardDescription>
      <ResponsiveContainer width='100%' height={350}>
        <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
          <defs>
            {topModels.map((modelId, index) => (
              <linearGradient key={`${modelId}-fill`} id={`model-throughput-${index}`} x1='0' y1='0' x2='0' y2='1'>
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
          {topModels.map((modelId, index) => {
            const color = COLORS[index % COLORS.length];
            const isActive = !activeSeries || activeSeries === modelId;
            const opacity = isActive ? 1 : 0.2;
            return (
              <Area
                key={modelId}
                type='monotone'
                dataKey={modelId}
                name={modelId}
                stroke={color}
                strokeWidth={2}
                fill={`url(#model-throughput-${index})`}
                fillOpacity={1}
                dot={false}
                activeDot={{ r: 4 }}
                connectNulls={false}
                strokeOpacity={opacity}
                hide={!visibleModels.includes(modelId)}
              />
            );
          })}
          {topModels.map((modelId, index) => {
            const color = COLORS[index % COLORS.length];
            const isActive = !activeSeries || activeSeries === modelId;
            const opacity = isActive ? 0.35 : 0.1;
            return (
              <Line
                key={`${modelId}-ttft`}
                yAxisId='ttft'
                type='monotone'
                dataKey={`${modelId}-ttft`}
                name={`${modelId} TTFT`}
                stroke={color}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 3 }}
                connectNulls={false}
                strokeOpacity={opacity}
                hide={!visibleModels.includes(modelId)}
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
