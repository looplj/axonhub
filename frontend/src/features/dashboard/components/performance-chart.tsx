import { useMemo, useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { CartesianGrid, ResponsiveContainer, XAxis, YAxis, Tooltip, AreaChart, Area } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { formatDuration } from '@/utils/format-duration';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useGeneralSettings } from '../../system/data/system';

function groupBy<T>(array: T[], key: keyof T): Record<string, T[]> {
  return array.reduce((acc, item) => {
    const k = String(item[key]);
    if (!acc[k]) acc[k] = [];
    acc[k].push(item);
    return acc;
  }, {} as Record<string, T[]>);
}

const COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
  'var(--chart-6)',
];

const MAX_CHART_THROUGHPUT = 1000;
const MAX_CHART_TTFT_MS = 60000;

export type PerformanceDisplayMode = 'throughput' | 'ttft';

export interface LegendItem {
  id: string;
  name: string;
  color: string;
  avgThroughput: number;
  avgTtft: number;
}

export interface PerformanceDataPoint {
  date: string;
  id: string;
  name?: string;
  throughput: number | null;
  ttftMs: number | null;
  requestCount: number;
}

interface PerformanceChartProps {
  data: PerformanceDataPoint[] | undefined;
  isLoading: boolean;
  error: Error | null;
  onTotalRequestsChange?: (total: number) => void;
  emptyMessage: string;
  errorMessage: string;
  idField: 'modelId' | 'channelId';
  nameField?: 'channelName';
}

interface TooltipPayloadItem {
  dataKey: string;
  value: number | null;
  name: string;
  color: string;
  payload: Record<string, string | number | null>;
}

interface PerformanceTooltipProps {
  active?: boolean;
  payload?: TooltipPayloadItem[];
  label?: string;
  displayMode: PerformanceDisplayMode;
}

function PerformanceTooltip({ active, payload, label, displayMode }: PerformanceTooltipProps) {
  const { t } = useTranslation();

  if (!active || !payload || payload.length === 0) return null;

  const dataPoint = payload[0]?.payload as Record<string, string | number | null> | undefined;
  if (!dataPoint) return null;

  const filteredPayload = displayMode === 'throughput'
    ? payload.filter((item) => item.dataKey.toString().includes('-capped') && !item.dataKey.toString().includes('-ttft') && item.value != null && item.value > 0)
    : payload.filter((item) => item.dataKey.toString().includes('-ttft-capped') && item.value != null && item.value > 0);

  const itemData = filteredPayload
    .map((item) => {
      const dataKey = item.dataKey.toString();
      // Extract base ID by removing capped suffixes
      const id = displayMode === 'throughput'
        ? dataKey.replace('-capped', '')
        : dataKey.replace('-ttft-capped', '');
      // Read actual values (not capped) from original data keys
      const throughputValue = dataPoint[id] as number ?? 0;
      const ttftValue = dataPoint[`${id}-ttft`] as number ?? 0;
      return {
        id,
        name: item.name,
        throughput: throughputValue,
        ttft: ttftValue,
        color: item.color,
      };
    })
    .sort((a, b) => displayMode === 'throughput' ? b.throughput - a.throughput : a.ttft - b.ttft);

  if (itemData.length === 0) return null;

  return (
    <div
      className='rounded-md border bg-background p-3 shadow-md'
      style={{ fontSize: '12px' }}
    >
      <div className='mb-2 font-medium text-foreground'>{label}</div>
      <div className='space-y-2'>
        {itemData.map((item) => (
          <div key={item.id}>
            <div className='flex items-center gap-2'>
              <span
                className='h-2 w-2 rounded-full'
                style={{ backgroundColor: item.color }}
              />
              <span className='truncate font-medium text-foreground'>
                {item.name}
              </span>
            </div>
            <div className='ml-4 text-muted-foreground'>
              {displayMode === 'throughput' ? (
                <>{formatNumber(item.throughput, { digits: 0 })} {t('dashboard.stats.throughput')}</>
              ) : (
                <>{t('dashboard.stats.ttft')} {formatDuration(item.ttft)}</>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export function PerformanceChart({
  data,
  isLoading,
  error,
  onTotalRequestsChange,
  emptyMessage,
  errorMessage,
  idField,
  nameField,
}: PerformanceChartProps) {
  const { t, i18n } = useTranslation();
  const { data: generalSettings, isLoading: isSettingsLoading } = useGeneralSettings();
  const [activeSeries, setActiveSeries] = useState<string | null>(null);
  const [displayMode, setDisplayMode] = useState<PerformanceDisplayMode>('throughput');

  const isLoadingData = isLoading || isSettingsLoading;

  const timezone = generalSettings?.timezone || 'UTC';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const memoizedSafeData = useMemo(() => data ?? [], [data]);

  const groupedById = useMemo(() => groupBy(memoizedSafeData, 'id'), [memoizedSafeData]);

  const { dates, topItems, legendItems, totalRequests } = useMemo(() => {
    const uniqueDates = [...new Set(memoizedSafeData.map((stat) => stat.date))].sort();

    const uniqueIds = [...new Set(memoizedSafeData.map((stat) => stat.id))].sort();

    const lItems = uniqueIds.map((id, index) => {
      const itemStatsList = groupedById[id] ?? [];
      const name = nameField && itemStatsList[0]?.name
        ? itemStatsList[0].name
        : id;
      const totalRequests = itemStatsList.reduce((sum, s) => sum + s.requestCount, 0);
      const weightedThroughput = totalRequests > 0
        ? itemStatsList.reduce((sum, s) => sum + (s.throughput ?? 0) * s.requestCount, 0) / totalRequests
        : 0;
      const weightedTtft = totalRequests > 0
        ? itemStatsList.reduce((sum, s) => sum + (s.ttftMs ?? 0) * s.requestCount, 0) / totalRequests
        : 0;

      return {
        id,
        name,
        color: COLORS[index % COLORS.length],
        avgThroughput: weightedThroughput,
        avgTtft: weightedTtft,
      };
    });

    lItems.sort((a, b) => a.name.localeCompare(b.name));

    const total = memoizedSafeData.reduce((sum, s) => sum + s.requestCount, 0);

    return { dates: uniqueDates, topItems: uniqueIds, legendItems: lItems, totalRequests: total };
  }, [memoizedSafeData, nameField, groupedById]);

  useEffect(() => {
    onTotalRequestsChange?.(totalRequests);
  }, [totalRequests, onTotalRequestsChange]);

  const statsMap = useMemo(() => {
    return memoizedSafeData.reduce((acc, stat) => {
      if (!acc[stat.date]) acc[stat.date] = {};
      acc[stat.date][stat.id] = stat;
      return acc;
    }, {} as Record<string, Record<string, typeof memoizedSafeData[0]>>);
  }, [memoizedSafeData]);

  const seriesDateRanges = useMemo(() => {
    const ranges: Record<string, {
      throughput: { first: string | null; last: string | null };
      ttft: { first: string | null; last: string | null };
    }> = {};
    topItems.forEach((id) => {
      const throughputDates = dates.filter((date) => statsMap[date]?.[id]?.throughput != null);
      const ttftDates = dates.filter((date) => statsMap[date]?.[id]?.ttftMs != null);
      ranges[id] = {
        throughput: throughputDates.length > 0
          ? { first: throughputDates[0], last: throughputDates[throughputDates.length - 1] }
          : { first: null, last: null },
        ttft: ttftDates.length > 0
          ? { first: ttftDates[0], last: ttftDates[ttftDates.length - 1] }
          : { first: null, last: null },
      };
    });
    return ranges;
  }, [dates, statsMap, topItems]);

  // Build date→index map for O(1) lookups instead of O(n) indexOf
  const dateIndexMap = useMemo(() => {
    const map = new Map<string, number>();
    dates.forEach((date, index) => map.set(date, index));
    return map;
  }, [dates]);

  if (isLoadingData) {
    return (
      <div className='flex h-[350px] items-center justify-center'>
        <Skeleton className='h-full w-full' />
      </div>
    );
  }

  if (error) {
    return (
      <div className='flex h-[350px] items-center justify-center text-red-500'>
        {errorMessage} {error.message}
      </div>
    );
  }

  if (!data || data.length === 0 || topItems.length === 0) {
    return (
      <div className='flex h-[350px] items-center justify-center text-muted-foreground'>
        {emptyMessage}
      </div>
    );
  }

  const chartData = dates.map((date) => {
    const [year, month, day] = date.split('-').map(Number);
    const dateObj = new Date(Date.UTC(year, month - 1, day));
    const dataPoint: Record<string, string | number | null> = {
      name: dateObj.toLocaleDateString(locale, {
        month: '2-digit',
        day: '2-digit',
        timeZone: 'UTC',
      }),
    };

    const dateIndex = dateIndexMap.get(date) ?? -1;

    topItems.forEach((id) => {
      const stat = statsMap[date]?.[id];
      const ranges = seriesDateRanges[id];

      const throughputRange = ranges.throughput;
      const throughputFirstIndex = throughputRange.first != null ? (dateIndexMap.get(throughputRange.first) ?? -1) : -1;
      const throughputLastIndex = throughputRange.last != null ? (dateIndexMap.get(throughputRange.last) ?? -1) : -1;
      const isThroughputOutsideRange = dateIndex < throughputFirstIndex || dateIndex > throughputLastIndex;

      const ttftRange = ranges.ttft;
      const ttftFirstIndex = ttftRange.first != null ? (dateIndexMap.get(ttftRange.first) ?? -1) : -1;
      const ttftLastIndex = ttftRange.last != null ? (dateIndexMap.get(ttftRange.last) ?? -1) : -1;
      const isTtftOutsideRange = dateIndex < ttftFirstIndex || dateIndex > ttftLastIndex;

      dataPoint[id] = stat?.throughput ?? (isThroughputOutsideRange ? 0 : null);
      dataPoint[`${id}-ttft`] = stat?.ttftMs ?? (isTtftOutsideRange ? 0 : null);
      dataPoint[`${id}-capped`] = Math.min(stat?.throughput ?? 0, MAX_CHART_THROUGHPUT);
      dataPoint[`${id}-ttft-capped`] = Math.min(stat?.ttftMs ?? 0, MAX_CHART_TTFT_MS);
    });

    return dataPoint;
  });

  const throughputValues = memoizedSafeData
    .filter((s) => s.throughput != null && topItems.includes(s.id))
    .map((s) => s.throughput!)
    .sort((a, b) => a - b);

  const maxThroughput = throughputValues.length > 10
    ? throughputValues[Math.floor(throughputValues.length * 0.9)] || throughputValues[throughputValues.length - 1]
    : throughputValues.length > 0
      ? throughputValues[throughputValues.length - 1]
      : 0;
  const throughputMax = Math.max(10, Math.ceil(maxThroughput * 1.1));

  const maxTtft = memoizedSafeData
    .filter((s) => s.ttftMs != null && s.ttftMs > 0 && topItems.includes(s.id))
    .reduce((max, s) => Math.max(max, s.ttftMs!), 0);
  const ttftMax = Math.max(100, Math.ceil(maxTtft * 1.1));

  const visibleItems = activeSeries ? [activeSeries] : topItems;

  // Dynamic Y-axis: use actual max if below cap, otherwise use cap
  const yAxisDomain = displayMode === 'throughput'
    ? [0, Math.min(throughputMax, MAX_CHART_THROUGHPUT)]
    : [0, Math.min(ttftMax, MAX_CHART_TTFT_MS)];
  const yAxisTickFormatter = displayMode === 'throughput'
    ? (value: number) => formatNumber(value, { digits: 0 })
    : (value: number) => formatDuration(value);

  const gradientPrefix = idField === 'modelId' ? 'model' : 'channel';

  return (
    <div>
      <div className='mb-3 flex items-center justify-end'>
        <Tabs value={displayMode} onValueChange={(v) => setDisplayMode(v as PerformanceDisplayMode)}>
          <TabsList className='h-7 p-0.5'>
            <TabsTrigger value='throughput' className='h-6 px-2.5 text-xs'>
              {t('dashboard.stats.throughput')}
            </TabsTrigger>
            <TabsTrigger value='ttft' className='h-6 px-2.5 text-xs'>
              {t('dashboard.stats.ttft')}
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <ResponsiveContainer width='100%' height={350}>
        <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
          <defs>
            {topItems.map((id, index) => (
              <linearGradient key={`${id}-fill`} id={`${gradientPrefix}-${displayMode}-${id}`} x1='0' y1='0' x2='0' y2='1'>
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
            width={50}
            tickMargin={8}
            tickCount={6}
          />
          <Tooltip content={<PerformanceTooltip displayMode={displayMode} />} />
          {topItems.map((id, index) => {
            const color = COLORS[index % COLORS.length];
            const isActive = !activeSeries || activeSeries === id;
            const opacity = isActive ? 1 : 0.2;
            const itemName = legendItems.find((item) => item.id === id)?.name || id;
            const dataKey = displayMode === 'throughput' ? `${id}-capped` : `${id}-ttft-capped`;
            return (
              <Area
                key={id}
                type='monotone'
                dataKey={dataKey}
                name={itemName}
                stroke={color}
                strokeWidth={2}
                fill={`url(#${gradientPrefix}-${displayMode}-${id})`}
                fillOpacity={1}
                dot={false}
                activeDot={{ r: 4 }}
                connectNulls={true}
                strokeOpacity={opacity}
                hide={!visibleItems.includes(id)}
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
                {formatNumber(item.avgThroughput, { digits: 0 })} {t('dashboard.stats.throughput')} · {t('dashboard.stats.ttft')} {formatDuration(item.avgTtft)}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
