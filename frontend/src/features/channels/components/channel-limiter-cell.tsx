import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ChannelLimiterStats } from '../data/schema';

interface ChannelLimiterCellProps {
  stats: ChannelLimiterStats | null | undefined;
}

/**
 * ChannelLimiterCell renders a compact text snapshot of the per-channel
 * concurrency limiter: `inFlight/capacity` for in-flight load, plus
 * `Q waiting/queueSize` when a queue is configured.
 *
 * Returns null when the channel has no MaxConcurrent configured (no admission
 * control); the parent column should render nothing in that case.
 */
export const ChannelLimiterCell = memo(({ stats }: ChannelLimiterCellProps) => {
  const { t } = useTranslation();

  if (!stats || stats.capacity <= 0) {
    return null;
  }

  const { inFlight, waiting, capacity, queueSize } = stats;

  const utilization = inFlight / capacity;
  const isFull = inFlight >= capacity;
  const isWarn = !isFull && utilization >= 0.5;
  const queueIsFull = queueSize > 0 && waiting >= queueSize;
  const queueIsBusy = queueSize > 0 && waiting > 0 && !queueIsFull;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className='flex cursor-help items-center gap-1'>
          <span
            className={cn(
              'text-xs tabular-nums',
              isFull && 'text-red-500',
              isWarn && 'text-yellow-600',
              !isFull && !isWarn && 'text-blue-500'
            )}
          >
            {inFlight}/{capacity}
          </span>
          {queueSize > 0 && (
            <span
              className={cn(
                'text-muted-foreground text-[10px] tabular-nums',
                queueIsFull && 'text-red-500',
                queueIsBusy && 'text-yellow-600'
              )}
            >
              Q {waiting}/{queueSize}
            </span>
          )}
        </div>
      </TooltipTrigger>
      <TooltipContent>
        <div className='space-y-1 text-xs'>
          <div>
            {t('channels.columns.limiterTooltip.inFlight')}: {inFlight}/{capacity}
          </div>
          {queueSize > 0 ? (
            <div>
              {t('channels.columns.limiterTooltip.waiting')}: {waiting}/{queueSize}
            </div>
          ) : (
            <div className='text-muted-foreground'>
              {t('channels.columns.limiterTooltip.softMode')}
            </div>
          )}
        </div>
      </TooltipContent>
    </Tooltip>
  );
});

ChannelLimiterCell.displayName = 'ChannelLimiterCell';
