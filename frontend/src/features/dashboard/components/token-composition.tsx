import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatNumber } from '@/utils/format-number';
import { formatCurrencySimple } from '@/features/analytics/utils/format-currency';
import { useTokensByAPIKey, useTokensByChannel, useTokensByModel, useUsageStatsByUser } from '../data/dashboard';
import { coarseWindowFromRange, type CoarseTimeWindow, type RelativeTimeWindow } from '../utils/time-window';
import { RangeBadge } from './range-badge';

const MAX_ROWS = 10;
const COLOR_CACHED = 'var(--chart-1)';
const COLOR_INPUT = 'var(--chart-2)';
const COLOR_OUTPUT = 'var(--chart-3)';

type TokenDimension = 'channel' | 'model' | 'apiKey' | 'user';

interface TokenRow {
  key: string;
  name: string;
  cached: number | null;
  uncached: number | null;
  output: number | null;
  totalTokens: number;
  requestCount?: number;
  cost?: number;
}

interface TokenRowProps {
  rank: string;
  row: TokenRow;
  barWidth: number;
  currencyCode: string;
}

function TokenRowItem({ rank, row, barWidth, currencyCode }: TokenRowProps) {
  const { t } = useTranslation();

  const hasBreakdown = row.cached != null || row.uncached != null || row.output != null;
  const breakdownTotal = (row.cached ?? 0) + (row.uncached ?? 0) + (row.output ?? 0);
  const inputTotal = (row.uncached ?? 0) + (row.cached ?? 0);
  const cacheHitRate = inputTotal > 0 ? ((row.cached ?? 0) / inputTotal) * 100 : null;

  const segments = hasBreakdown
    ? [
        { key: 'cached', value: row.cached ?? 0, color: COLOR_CACHED },
        { key: 'input', value: row.uncached ?? 0, color: COLOR_INPUT },
        { key: 'output', value: row.output ?? 0, color: COLOR_OUTPUT },
      ].filter((segment) => segment.value > 0)
    : [{ key: 'total', value: 1, color: COLOR_OUTPUT }];

  return (
    <div className='grid grid-cols-[1.5rem_minmax(0,1fr)_minmax(0,1.6fr)_auto] items-center gap-3 text-sm'>
      <span className='text-muted-foreground text-right text-xs tabular-nums'>{rank}</span>
      <span className='truncate font-medium' title={row.name}>
        {row.name}
      </span>
      <div className='flex min-w-0 items-center gap-2'>
        <div className='flex h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-muted'>
          {segments.map((segment) => (
            <div
              key={segment.key}
              style={{
                width: `${hasBreakdown ? (segment.value / breakdownTotal) * 100 : barWidth}%`,
                backgroundColor: segment.color,
              }}
            />
          ))}
        </div>
        {cacheHitRate != null && (
          <span className='text-muted-foreground w-12 text-right text-xs tabular-nums'>{cacheHitRate.toFixed(0)}%</span>
        )}
      </div>
      <div className='text-right'>
        <div className='font-mono text-xs font-medium tabular-nums'>{formatNumber(row.totalTokens)}</div>
        <div className='text-muted-foreground text-xs tabular-nums'>
          {row.requestCount != null && row.cost != null
            ? `${formatNumber(row.requestCount)} · ${formatCurrencySimple(row.cost, currencyCode)}`
            : cacheHitRate != null
              ? `${cacheHitRate.toFixed(0)}% ${t('dashboard.stats.cached')}`
              : ''}
        </div>
      </div>
    </div>
  );
}

interface TokenCompositionProps {
  startTime: string | null;
  endTime: string | null;
  timeWindow?: RelativeTimeWindow;
  currencyCode: string;
  isProjectOwner: boolean;
}

/** Token composition per channel, model, API key or user: cached and uncached input
 * plus output stacked in one bar, with the total and cache hit rate beside it. */
export function TokenComposition({ startTime, endTime, timeWindow, currencyCode, isProjectOwner }: TokenCompositionProps) {
  const { t } = useTranslation();
  const [dimension, setDimension] = useState<TokenDimension>('channel');
  const coarseWindow: CoarseTimeWindow = coarseWindowFromRange(startTime, endTime, timeWindow);

  const channels = useTokensByChannel(coarseWindow);
  const models = useTokensByModel(coarseWindow);
  const apiKeys = useTokensByAPIKey(coarseWindow);
  const users = useUsageStatsByUser(coarseWindow);

  const rows = useMemo<TokenRow[]>(() => {
    if (dimension === 'user') {
      return (users.data ?? []).map((item) => ({
        key: item.userId,
        name: item.userName || item.userId,
        cached: null,
        uncached: null,
        output: null,
        totalTokens: item.totalTokens,
        requestCount: item.requestCount,
        cost: item.totalCost,
      }));
    }

    if (dimension === 'channel') {
      return (channels.data ?? []).map((item) => ({
        key: item.channelId,
        name: item.channelName || item.channelId,
        cached: item.cachedTokens,
        uncached: Math.max(item.inputTokens - item.cachedTokens, 0),
        output: item.outputTokens,
        totalTokens: item.inputTokens + item.outputTokens,
      }));
    }

    if (dimension === 'model') {
      return (models.data ?? []).map((item) => ({
        key: item.modelId,
        name: item.modelId,
        cached: item.cachedTokens,
        uncached: Math.max(item.inputTokens - item.cachedTokens, 0),
        output: item.outputTokens,
        totalTokens: item.inputTokens + item.outputTokens,
      }));
    }

    return (apiKeys.data ?? []).map((item) => ({
      key: item.apiKeyId,
      name: item.apiKeyName || item.apiKeyId,
      cached: item.cachedTokens,
      uncached: Math.max(item.inputTokens - item.cachedTokens, 0),
      output: item.outputTokens,
      totalTokens: item.inputTokens + item.outputTokens,
    }));
  }, [dimension, channels.data, models.data, apiKeys.data, users.data]);

  const activeQuery = dimension === 'user' ? users : dimension === 'channel' ? channels : dimension === 'model' ? models : apiKeys;
  const sorted = [...rows].sort((a, b) => b.totalTokens - a.totalTokens);
  const topRows = sorted.slice(0, MAX_ROWS);
  const tail = sorted.slice(MAX_ROWS);
  const maxTokens = topRows[0]?.totalTokens || 0;

  return (
    <Card className='hover-card'>
      <CardHeader>
        <div className='space-y-1'>
          <CardTitle>{t('dashboard.charts.tokenComposition')}</CardTitle>
          <CardDescription>{t('dashboard.charts.tokenCompositionDescription')}</CardDescription>
        </div>
        <CardAction className='flex items-center gap-2'>
          <RangeBadge timeWindow={coarseWindow} />
          <Tabs value={dimension} onValueChange={(value) => setDimension(value as TokenDimension)}>
            <TabsList className='h-8'>
              <TabsTrigger value='channel' className='text-xs'>
                {t('dashboard.stats.channel')}
              </TabsTrigger>
              <TabsTrigger value='model' className='text-xs'>
                {t('dashboard.stats.model')}
              </TabsTrigger>
              <TabsTrigger value='apiKey' className='text-xs'>
                {t('dashboard.stats.apiKey')}
              </TabsTrigger>
              {isProjectOwner && (
                <TabsTrigger value='user' className='text-xs'>
                  {t('dashboard.stats.user')}
                </TabsTrigger>
              )}
            </TabsList>
          </Tabs>
        </CardAction>
      </CardHeader>
      <CardContent>
        {activeQuery.isLoading && rows.length === 0 ? (
          <div className='space-y-3'>
            {Array.from({ length: 6 }).map((_, index) => (
              <Skeleton key={index} className='h-8 w-full' />
            ))}
          </div>
        ) : rows.length === 0 ? (
          <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noTokenData')}</div>
        ) : (
          <div className='space-y-3'>
            <div className='text-muted-foreground grid grid-cols-[1.5rem_minmax(0,1fr)_minmax(0,1.6fr)_auto] items-center gap-3 text-xs font-medium'>
              <span />
              <span>{t('analytics.table.name')}</span>
              <span>
                {t('dashboard.stats.cached')} / {t('dashboard.stats.input')} / {t('dashboard.stats.output')}
              </span>
              <span>{t('analytics.table.totalTokens')}</span>
            </div>
            {topRows.map((row, index) => (
              <TokenRowItem
                key={row.key}
                rank={String(index + 1).padStart(2, '0')}
                row={row}
                barWidth={maxTokens > 0 ? (row.totalTokens / maxTokens) * 100 : 0}
                currencyCode={currencyCode}
              />
            ))}
            {tail.length > 0 && (
              <TokenRowItem
                rank='—'
                row={{
                  key: 'other',
                  name: t('dashboard.charts.other'),
                  cached: dimension === 'user' ? null : tail.reduce((sum, row) => sum + (row.cached ?? 0), 0),
                  uncached: dimension === 'user' ? null : tail.reduce((sum, row) => sum + (row.uncached ?? 0), 0),
                  output: dimension === 'user' ? null : tail.reduce((sum, row) => sum + (row.output ?? 0), 0),
                  totalTokens: tail.reduce((sum, row) => sum + row.totalTokens, 0),
                }}
                barWidth={maxTokens > 0 ? (tail.reduce((sum, row) => sum + row.totalTokens, 0) / maxTokens) * 100 : 0}
                currencyCode={currencyCode}
              />
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
