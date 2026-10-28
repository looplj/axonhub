import { useState, useMemo, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardAction, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { ArrowUpIcon, ArrowDownIcon, ArrowUpDownIcon } from 'lucide-react';
import { formatNumber } from '@/utils/format-number';
import { formatDuration } from '@/utils/format-duration';
import { useGeneralSettings } from '@/features/system/data/system';
import { TimePeriodSelector, type TimePeriod } from '@/components/time-period-selector';
import { useChannelModelPerformanceStats, type ChannelModelPerformanceStat } from '../data/dashboard';

type SortField = 'requestCount' | 'throughput' | 'ttftMs' | 'successRate' | 'totalCost' | 'totalTokens';
type SortOrder = 'asc' | 'desc';

function formatExactNumber(value: number): string {
  return Math.round(value).toLocaleString();
}

function getSortValue(item: ChannelModelPerformanceStat, field: SortField): number {
  switch (field) {
    case 'throughput':
      return item.throughput ?? -1;
    case 'ttftMs':
      return item.ttftMs ?? -1;
    case 'successRate':
      return item.successRate;
    case 'totalCost':
      return item.totalCost;
    case 'totalTokens':
      return item.totalTokens;
    default:
      return item.requestCount;
  }
}

interface ChannelModelPerformanceTableProps {
  timePeriod?: TimePeriod;
  onTimePeriodChange?: (value: TimePeriod) => void;
}

export function ChannelModelPerformanceTable({ timePeriod = 'allTime', onTimePeriodChange }: ChannelModelPerformanceTableProps) {
  const { t, i18n } = useTranslation();
  const timeWindow = timePeriod === 'allTime' ? undefined : timePeriod;
  const { data, isLoading, error } = useChannelModelPerformanceStats(timeWindow);
  const { data: generalSettings } = useGeneralSettings();
  const [sortField, setSortField] = useState<SortField>('requestCount');
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc');

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const formatCurrency = useCallback(
    (val: number) =>
      t('currencies.format', {
        val,
        currency: currencyCode,
        locale,
        minimumFractionDigits: 4,
        maximumFractionDigits: 4,
      }),
    [currencyCode, locale, t]
  );

  const totalRequests = useMemo(
    () => (data ?? []).reduce((sum, item) => sum + item.requestCount, 0),
    [data]
  );

  const sortedData = useMemo(() => {
    if (!data) return [];
    return [...data].sort((a, b) => {
      const diff = getSortValue(a, sortField) - getSortValue(b, sortField);
      return sortOrder === 'asc' ? diff : -diff;
    });
  }, [data, sortField, sortOrder]);

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortOrder((prev) => (prev === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortField(field);
      setSortOrder('desc');
    }
  };

  const renderSortIcon = (field: SortField) => {
    if (sortField !== field) {
      return <ArrowUpDownIcon className='ml-1 inline h-3 w-3 text-muted-foreground/50' />;
    }
    return sortOrder === 'asc'
      ? <ArrowUpIcon className='ml-1 inline h-3 w-3 text-primary' />
      : <ArrowDownIcon className='ml-1 inline h-3 w-3 text-primary' />;
  };

  const description = t('dashboard.charts.channelModelPerformanceDescription', {
    count: formatNumber(totalRequests),
  });

  const renderHeaderCell = (field: SortField, label: string, align: 'left' | 'right' = 'right') => (
    <th
      className={`px-4 py-2 text-xs font-medium text-muted-foreground ${align === 'right' ? 'text-right' : 'text-left'}`}
    >
      <button
        type='button'
        onClick={() => handleSort(field)}
        className={`inline-flex items-center hover:text-foreground ${align === 'right' ? 'flex-row-reverse' : ''}`}
      >
        {label}
        {renderSortIcon(field)}
      </button>
    </th>
  );

  return (
    <Card className='hover-card'>
      <CardHeader>
        <CardTitle>{t('dashboard.charts.channelModelPerformance')}</CardTitle>
        <CardDescription>{description}</CardDescription>
        {onTimePeriodChange && (
          <CardAction>
            <TimePeriodSelector value={timePeriod} onChange={onTimePeriodChange} />
          </CardAction>
        )}
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-[300px] w-full' />
        ) : error ? (
          <div className='flex h-[200px] items-center justify-center text-sm text-red-500'>
            {t('dashboard.charts.errorLoadingChannelModelData')} {error.message}
          </div>
        ) : !data || data.length === 0 ? (
          <div className='flex h-[200px] items-center justify-center text-sm text-muted-foreground'>
            {t('dashboard.charts.noChannelModelData')}
          </div>
        ) : (
          <div className='overflow-x-auto'>
            <table className='w-full min-w-[1000px] table-fixed caption-bottom text-sm'>
              <colgroup>
                <col className='w-[18%]' />
                <col className='w-[18%]' />
                <col className='w-[12%]' />
                <col className='w-[12%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
              </colgroup>
              <thead className='[&_tr]:border-b'>
                <tr className='border-b transition-colors hover:bg-muted/50'>
                  <th className='sticky left-0 z-10 bg-card px-4 py-2 text-left text-xs font-medium text-muted-foreground'>
                    {t('dashboard.stats.channel')}
                  </th>
                  <th className='px-4 py-2 text-left text-xs font-medium text-muted-foreground'>
                    {t('dashboard.stats.model')}
                  </th>
                  {renderHeaderCell('throughput', t('dashboard.stats.throughput'))}
                  {renderHeaderCell('ttftMs', t('dashboard.stats.ttft'))}
                  {renderHeaderCell('successRate', t('dashboard.stats.successRate'))}
                  {renderHeaderCell('requestCount', t('dashboard.stats.requestCount'))}
                  {renderHeaderCell('totalTokens', t('dashboard.stats.totalTokens'))}
                  {renderHeaderCell('totalCost', t('dashboard.stats.totalCost'))}
                </tr>
              </thead>
              <tbody className='[&_tr:last-child]:border-0'>
                {sortedData.map((item) => {
                  const successRateClass =
                    item.successRate >= 95
                      ? 'text-green-600 dark:text-green-400'
                      : item.successRate >= 80
                        ? 'text-yellow-600 dark:text-yellow-400'
                        : 'text-red-600 dark:text-red-400';
                  return (
                    <tr key={`${item.channelId}-${item.modelId}`} className='border-b transition-colors hover:bg-muted/50'>
                      <td className='sticky left-0 z-10 truncate bg-card px-4 py-2 text-sm font-medium' title={item.channelName}>
                        {item.channelName}
                      </td>
                      <td className='truncate px-4 py-2 text-sm font-medium' title={item.modelId}>
                        {item.modelId}
                      </td>
                      <td className='whitespace-nowrap px-4 py-2 text-right text-sm tabular-nums'>
                        {item.throughput == null ? '—' : formatNumber(item.throughput, { digits: 1 })}
                      </td>
                      <td className='whitespace-nowrap px-4 py-2 text-right text-sm tabular-nums'>
                        {item.ttftMs == null ? '—' : formatDuration(item.ttftMs)}
                      </td>
                      <td className={`whitespace-nowrap px-4 py-2 text-right text-sm font-medium tabular-nums ${successRateClass}`}>
                        {item.successRate.toFixed(1)}%
                      </td>
                      <td className='whitespace-nowrap px-4 py-2 text-right text-sm tabular-nums'>
                        {formatExactNumber(item.requestCount)}
                      </td>
                      <td className='whitespace-nowrap px-4 py-2 text-right text-sm tabular-nums'>
                        {formatExactNumber(item.totalTokens)}
                      </td>
                      <td className='whitespace-nowrap px-4 py-2 text-right text-sm tabular-nums'>
                        {formatCurrency(item.totalCost)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}