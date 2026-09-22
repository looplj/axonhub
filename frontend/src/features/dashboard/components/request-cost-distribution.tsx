import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatNumber } from '@/utils/format-number';
import { formatCurrencySimple } from '@/features/analytics/utils/format-currency';
import {
  useCostByAPIKey,
  useCostByChannel,
  useCostByModel,
  useRequestsByAPIKey,
  useRequestsByChannel,
  useRequestsByModel,
} from '../data/dashboard';
import { coarseWindowFromRange, type CoarseTimeWindow } from '../utils/time-window';
import { RangeBadge } from './range-badge';

const MAX_ROWS = 10;
const COLOR_REQUESTS = 'var(--chart-4)';
const COLOR_COST = 'var(--chart-5)';

type DistributionDimension = 'channel' | 'model' | 'apiKey';

interface DistributionRow {
  key: string;
  name: string;
  requests: number;
  cost: number;
}

interface DistributionRowProps {
  rank: string;
  name: string;
  requests: number;
  requestShare: number;
  cost: number;
  costShare: number;
  currencyCode: string;
}

function DistributionRowItem({ rank, name, requests, requestShare, cost, costShare, currencyCode }: DistributionRowProps) {
  return (
    <div className='grid grid-cols-[1.5rem_minmax(0,1fr)_minmax(0,1.1fr)_auto_minmax(0,1.1fr)] items-center gap-3 text-sm'>
      <span className='text-muted-foreground text-right text-xs tabular-nums'>{rank}</span>
      <span className='truncate font-medium' title={name}>
        {name}
      </span>
      <div className='flex min-w-0 items-center gap-2'>
        <div className='h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-muted'>
          <div className='h-full rounded-full' style={{ width: `${requestShare}%`, backgroundColor: COLOR_REQUESTS }} />
        </div>
        <span className='w-12 text-right text-xs tabular-nums'>{formatNumber(requests)}</span>
      </div>
      <span className='w-24 text-right font-mono text-xs tabular-nums'>{formatCurrencySimple(cost, currencyCode)}</span>
      <div className='flex min-w-0 items-center gap-2'>
        <div className='h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-muted'>
          <div className='h-full rounded-full' style={{ width: `${costShare}%`, backgroundColor: COLOR_COST }} />
        </div>
        <span className='text-muted-foreground w-10 text-right text-xs tabular-nums'>{costShare.toFixed(0)}%</span>
      </div>
    </div>
  );
}

interface RequestCostDistributionProps {
  startTime: string | null;
  endTime: string | null;
  currencyCode: string;
}

/** Ranked request and cost shares per channel, model or API key, with the long tail
 * aggregated into a single row instead of being hidden. */
export function RequestCostDistribution({ startTime, endTime, currencyCode }: RequestCostDistributionProps) {
  const { t } = useTranslation();
  const [dimension, setDimension] = useState<DistributionDimension>('channel');
  const timeWindow: CoarseTimeWindow = coarseWindowFromRange(startTime, endTime);

  const channelsRequests = useRequestsByChannel(timeWindow);
  const channelsCost = useCostByChannel(timeWindow);
  const modelsRequests = useRequestsByModel(timeWindow);
  const modelsCost = useCostByModel(timeWindow);
  const apiKeysRequests = useRequestsByAPIKey(timeWindow);
  const apiKeysCost = useCostByAPIKey(timeWindow);

  const activeRequests = dimension === 'channel' ? channelsRequests : dimension === 'model' ? modelsRequests : apiKeysRequests;

  const rows = useMemo<DistributionRow[]>(() => {
    const byKey = new Map<string, DistributionRow>();

    const addRequests = (key: string, name: string | undefined, count: number) => {
      const row = byKey.get(key) ?? { key, name: name || key, requests: 0, cost: 0 };
      row.requests += count;
      byKey.set(key, row);
    };
    const addCost = (key: string, cost: number) => {
      const row = byKey.get(key) ?? { key, name: key, requests: 0, cost: 0 };
      row.cost += cost;
      byKey.set(key, row);
    };

    if (dimension === 'channel') {
      for (const item of channelsRequests.data ?? []) addRequests(item.channelName, item.channelName, item.count);
      for (const item of channelsCost.data ?? []) addCost(item.channelName, item.cost);
    } else if (dimension === 'model') {
      for (const item of modelsRequests.data ?? []) addRequests(item.modelId, item.modelId, item.count);
      for (const item of modelsCost.data ?? []) addCost(item.modelId, item.cost);
    } else {
      for (const item of apiKeysRequests.data ?? []) addRequests(item.apiKeyId, item.apiKeyName, item.count);
      for (const item of apiKeysCost.data ?? []) addCost(item.apiKeyId, item.cost);
    }

    return [...byKey.values()].sort((a, b) => b.requests - a.requests);
  }, [dimension, channelsRequests.data, channelsCost.data, modelsRequests.data, modelsCost.data, apiKeysRequests.data, apiKeysCost.data]);

  const topRows = rows.slice(0, MAX_ROWS);
  const tail = rows.slice(MAX_ROWS);
  const totalRequests = rows.reduce((sum, row) => sum + row.requests, 0);
  const totalCost = rows.reduce((sum, row) => sum + row.cost, 0);

  const share = (value: number, total: number) => (total > 0 ? (value / total) * 100 : 0);

  return (
    <Card className='hover-card'>
      <CardHeader>
        <div className='space-y-1'>
          <CardTitle>{t('dashboard.charts.requestsCost')}</CardTitle>
          <CardDescription>{t('dashboard.charts.requestsCostDescription')}</CardDescription>
        </div>
        <CardAction className='flex items-center gap-2'>
          <RangeBadge timeWindow={timeWindow} />
          <Tabs value={dimension} onValueChange={(value) => setDimension(value as DistributionDimension)}>
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
            </TabsList>
          </Tabs>
        </CardAction>
      </CardHeader>
      <CardContent>
        {activeRequests.isLoading && rows.length === 0 ? (
          <div className='space-y-3'>
            {Array.from({ length: 6 }).map((_, index) => (
              <Skeleton key={index} className='h-6 w-full' />
            ))}
          </div>
        ) : rows.length === 0 ? (
          <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noChannelData')}</div>
        ) : (
          <div className='space-y-3'>
            <div className='text-muted-foreground grid grid-cols-[1.5rem_minmax(0,1fr)_minmax(0,1.1fr)_auto_minmax(0,1.1fr)] items-center gap-3 text-xs font-medium'>
              <span />
              <span>{t('analytics.table.name')}</span>
              <span>{t('analytics.table.requests')}</span>
              <span className='w-24 text-right'>{t('analytics.table.cost')}</span>
              <span>{t('dashboard.charts.costShare')}</span>
            </div>
            {topRows.map((row, index) => (
              <DistributionRowItem
                key={row.key}
                rank={String(index + 1).padStart(2, '0')}
                name={row.name}
                requests={row.requests}
                requestShare={share(row.requests, totalRequests)}
                cost={row.cost}
                costShare={share(row.cost, totalCost)}
                currencyCode={currencyCode}
              />
            ))}
            {tail.length > 0 && (
              <DistributionRowItem
                rank='—'
                name={t('dashboard.charts.other')}
                requests={tail.reduce((sum, row) => sum + row.requests, 0)}
                requestShare={share(
                  tail.reduce((sum, row) => sum + row.requests, 0),
                  totalRequests
                )}
                cost={tail.reduce((sum, row) => sum + row.cost, 0)}
                costShare={share(
                  tail.reduce((sum, row) => sum + row.cost, 0),
                  totalCost
                )}
                currencyCode={currencyCode}
              />
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
