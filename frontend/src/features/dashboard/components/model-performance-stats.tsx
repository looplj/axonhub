'use client';

import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CartesianGrid, ResponsiveContainer, XAxis, YAxis, Tooltip, AreaChart, Area } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { formatDuration } from '@/utils/format-duration';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
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

export type PerformanceDisplayMode = 'throughput' | 'ttft';

interface TooltipProps {
  active?: boolean;
  payload?: Array<{
    dataKey: string;
    value: number;
    name: string;
    color: string;
    payload: Record<string, string | number>;
  }>;
  label?: string;
  displayMode: PerformanceDisplayMode;
}

function PerformanceTooltip({ active, payload, label, displayMode }: TooltipProps) {
  const { t } = useTranslation();

  if (!active || !payload || payload.length === 0) return null;

  const dataPoint = payload[0]?.payload as Record<string, string | number> | undefined;
  if (!dataPoint) return null;

  const filteredPayload = displayMode === 'throughput'
    ? payload.filter((item) => !item.dataKey.toString().includes('-ttft') && item.value > 0)
    : payload.filter((item) => item.dataKey.toString().includes('-ttft') && item.value > 0);

  const modelData = filteredPayload
    .map((item) => {
      const dataKey = item.dataKey.toString();
      const modelId = displayMode === 'throughput' ? dataKey : dataKey.replace('-ttft', '');
      const throughputValue = dataPoint[modelId] as number ?? 0;
      const ttftValue = dataPoint[`${modelId}-ttft`] as number ?? 0;
      return {
        modelId: modelId,
        throughput: throughputValue,
        ttft: ttftValue,
        color: item.color,
      };
    })
    .sort((a, b) => displayMode === 'throughput' ? b.throughput - a.throughput : a.ttft - b.ttft);

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
              {displayMode === 'throughput' ? (
                <>{formatNumber(item.throughput, { digits: 0 })} {t('dashboard.stats.throughput')}</>
              ) : (
                <>TTFT {formatDuration(item.ttft)}</>
              )}
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
  const [displayMode, setDisplayMode] = useState<PerformanceDisplayMode>('throughput');

  const isLoading = isStatsLoading || isSettingsLoading;

  const timezone = generalSettings?.timezone || 'UTC';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const safeStats = performanceStats ?? [];

  const { dates, topModels, legendItems } = useMemo(() => {
    const uniqueDates = [...new Set(safeStats.map((stat) => stat.date))].sort();
    
    const uniqueModels = [...new Set(safeStats.map((stat) => stat.modelId))].sort();
    
    const lItems = uniqueModels.map((modelId, index) => {
      const modelStatsList = safeStats.filter((s) => s.modelId === modelId);
      const totalRequests = modelStatsList.reduce((sum, s) => sum + s.requestCount, 0);
      const weightedThroughput = totalRequests > 0
        ? modelStatsList.reduce((sum, s) => sum + (s.throughput ?? 0) * s.requestCount, 0) / totalRequests
        : 0;
      const weightedTtft = totalRequests > 0
        ? modelStatsList.reduce((sum, s) => sum + (s.ttftMs ?? 0) * s.requestCount, 0) / totalRequests
        : 0;

      return {
        id: modelId,
        name: modelId,
        color: COLORS[index % COLORS.length],
        avgThroughput: weightedThroughput,
        avgTtft: weightedTtft,
      };
    });

    return { dates: uniqueDates, topModels: uniqueModels, legendItems: lItems };
  }, [safeStats]);

  const statsMap = useMemo(() => {
    return safeStats.reduce((acc, stat) => {
      if (!acc[stat.date]) acc[stat.date] = {};
      acc[stat.date][stat.modelId] = stat;
      return acc;
    }, {} as Record<string, Record<string, typeof safeStats[0]>>);
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
    const dateObj = new Date(Date.UTC(year, month - 1, day));
    const dataPoint: Record<string, string | number> = {
      name: dateObj.toLocaleDateString(locale, {
        month: '2-digit',
        day: '2-digit',
        timeZone: timezone,
      }),
    };

    topModels.forEach((modelId) => {
      const stat = statsMap[date]?.[modelId];
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
      .filter((s) => s.ttftMs != null && s.ttftMs > 0 && topModels.includes(s.modelId))
      .map((s) => s.ttftMs!),
    0
  );
  const ttftMax = Math.max(100, Math.ceil(maxTtft * 1.1));

  const visibleModels = activeSeries ? [activeSeries] : topModels;

  const yAxisDomain = displayMode === 'throughput' ? [0, throughputMax] : [0, ttftMax];
  const yAxisTickFormatter = displayMode === 'throughput'
    ? (value: number) => formatNumber(value)
    : (value: number) => formatDuration(value);

  return (
    <div>
      <div className='mb-3 flex items-center justify-end'>
        <Tabs value={displayMode} onValueChange={(v) => setDisplayMode(v as PerformanceDisplayMode)}>
          <TabsList className='h-7 p-0.5'>
            <TabsTrigger value='throughput' className='h-6 px-2.5 text-xs'>
              {t('dashboard.stats.throughput')}
            </TabsTrigger>
            <TabsTrigger value='ttft' className='h-6 px-2.5 text-xs'>
              TTFT
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <ResponsiveContainer width='100%' height={350}>
        <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
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
            domain={yAxisDomain}
            tickFormatter={yAxisTickFormatter}
            width={40}
            tickMargin={8}
          />
          <Tooltip content={<PerformanceTooltip displayMode={displayMode} />} />
          {topModels.map((modelId, index) => {
            const color = COLORS[index % COLORS.length];
            const isActive = !activeSeries || activeSeries === modelId;
            const opacity = isActive ? 1 : 0.2;
            const dataKey = displayMode === 'throughput' ? modelId : `${modelId}-ttft`;
            return (
              <Area
                key={modelId}
                type='monotone'
                dataKey={dataKey}
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
          </AreaChart>
      </ResponsiveContainer>
      <div className='mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
        {legendItems.map((item) => {
          const isActive = !activeSeries || activeSeries === item.id;
          return (
            <button
              type='button'
              key={item.id}
              onClick={() => setActiveSeries((current) => (current === item.id ? null : item.id))}
              className={`flex flex-col gap-1 rounded-md border px-2 py-1.5 text-left text-sm transition 2xl:flex-row 2xl:items-center 2xl:justify-between ${
                isActive ? 'border-primary/40 bg-primary/5 text-foreground' : 'border-border text-muted-foreground'
              }`}
            >
              <span className='flex min-w-0 items-center gap-2'>
                <span className='h-2.5 w-2.5 rounded-full' style={{ backgroundColor: item.color }} />
                <span className='font-medium'>{item.name}</span>
              </span>
              <span className='text-xs text-muted-foreground tabular-nums 2xl:text-right'>
                {formatNumber(item.avgThroughput, { digits: 0 })} {t('dashboard.stats.throughput')} · TTFT {formatDuration(item.avgTtft)}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
