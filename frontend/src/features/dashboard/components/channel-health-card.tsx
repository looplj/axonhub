import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, ActivityIcon } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import { useChannelSuccessRates, type ChannelSuccessRate } from '../data/dashboard';
import { coarseWindowLabelKey, inclusiveCalendarDays, type CoarseTimeWindow } from '../utils/time-window';

const UNHEALTHY_RATE = 90;
const CHANNEL_LIMIT = 12;

// channelSuccessRates only understands coarse calendar windows, so the global
// date range is mapped onto the closest supported window.
function toCoarseTimeWindow(startTime: string | null, endTime: string | null): CoarseTimeWindow {
  if (!startTime) return 'allTime';

  const days = inclusiveCalendarDays(startTime, endTime);

  if (days <= 1) return 'day';
  if (days <= 7) return 'week';
  if (days <= 31) return 'month';
  return 'allTime';
}

interface ChannelHealthCardProps {
  startTime: string | null;
  endTime: string | null;
}

export function ChannelHealthCard({ startTime, endTime }: ChannelHealthCardProps) {
  const { t } = useTranslation();
  const timeWindow = toCoarseTimeWindow(startTime, endTime);
  const { data: channels, isLoading, error } = useChannelSuccessRates(CHANNEL_LIMIT, timeWindow);

  const rows = useMemo(() => {
    const sorted = [...(channels || [])].sort((a, b) => {
      const aUnhealthy = a.successRate < UNHEALTHY_RATE;
      const bUnhealthy = b.successRate < UNHEALTHY_RATE;
      if (aUnhealthy !== bUnhealthy) return aUnhealthy ? -1 : 1;
      if (aUnhealthy && bUnhealthy) return a.successRate - b.successRate;
      return b.totalCount - a.totalCount;
    });
    return sorted;
  }, [channels]);

  const unhealthyCount = rows.filter((channel) => channel.successRate < UNHEALTHY_RATE).length;

  const renderRow = (channel: ChannelSuccessRate) => {
    const unhealthy = channel.successRate < UNHEALTHY_RATE;
    const total = channel.totalCount || 1;

    return (
      <div key={channel.channelId} className='space-y-1.5'>
        <div className='flex items-center gap-2 text-sm'>
          <div
            className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-md ${
              unhealthy ? 'bg-red-500/10 text-red-500' : 'bg-primary/10 text-primary'
            }`}
          >
            {unhealthy ? <AlertTriangle className='h-3.5 w-3.5' /> : <ActivityIcon className='h-3.5 w-3.5' />}
          </div>
          <span className='min-w-0 flex-1 truncate font-medium' title={channel.channelName}>
            {channel.channelName || '-'}
          </span>
          <span className={`shrink-0 font-mono text-xs font-medium ${unhealthy ? 'text-red-500' : ''}`}>
            {channel.successRate.toFixed(1)}%
          </span>
        </div>
        <div className='flex h-1.5 overflow-hidden rounded-full bg-muted'>
          <div className='bg-green-500' style={{ width: `${(channel.successCount / total) * 100}%` }} />
          <div className='bg-red-500' style={{ width: `${(channel.failedCount / total) * 100}%` }} />
        </div>
        <div className='text-muted-foreground flex gap-3 text-xs'>
          <span className='flex items-center gap-1 text-green-600 dark:text-green-500'>
            {formatNumber(channel.successCount)} {t('dashboard.stats.requests')}
          </span>
          <span className='flex items-center gap-1 text-red-500'>
            {formatNumber(channel.failedCount)} {t('dashboard.stats.failedRequests')}
          </span>
        </div>
      </div>
    );
  };

  return (
    <Card className='hover-card flex flex-col'>
      <CardHeader>
        <CardTitle>{t('dashboard.charts.channelHealth')}</CardTitle>
        <CardDescription>
          {unhealthyCount > 0
            ? t('dashboard.charts.channelHealthUnhealthy', { count: unhealthyCount })
            : t('dashboard.charts.channelSuccessRateDescription')}
        </CardDescription>
        <CardAction>
          <Badge variant='outline' className='text-muted-foreground font-normal'>
            {t(coarseWindowLabelKey(timeWindow))}
          </Badge>
        </CardAction>
      </CardHeader>
      <CardContent className='flex-1'>
        {isLoading ? (
          <div className='space-y-4'>
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className='h-12 w-full' />
            ))}
          </div>
        ) : error ? (
          <div className='text-sm text-red-500'>
            {t('dashboard.charts.errorLoadingChannelSuccessRate')} {error.message}
          </div>
        ) : rows.length === 0 ? (
          <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noChannelData')}</div>
        ) : (
          <div className='max-h-[300px] space-y-4 overflow-y-auto pr-1 [scrollbar-gutter:stable]'>
            {rows.map(renderRow)}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
