import { memo } from 'react';
import { format } from 'date-fns';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ChannelProbePoint } from '../data/schema';

interface ChannelHealthCellProps {
  points: ChannelProbePoint[];
}

export const ChannelHealthCell = memo(({ points }: ChannelHealthCellProps) => {
  const { t } = useTranslation();

  if (!points || points.length === 0) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  const maxBars = 15;
  const displayPoints = points.slice(-maxBars);

  return (
    <div className='flex items-center gap-0.5'>
      {displayPoints.map((point, index) => {
        const hasRequests = point.totalRequestCount > 0;
        const successRate = hasRequests
          ? point.successRequestCount / point.totalRequestCount
          : 0;

        const isHealthy = hasRequests && successRate >= 0.9;
        const isWarning = hasRequests && successRate >= 0.5 && successRate < 0.9;
        const isError = hasRequests && successRate < 0.5;
        const isIdle = !hasRequests;

        const probeTime = format(new Date(point.timestamp * 1000), 'MM-dd HH:mm');

        return (
          <Tooltip key={`${point.timestamp}-${index}`}>
            <TooltipTrigger asChild>
              <div
                className={cn(
                  'h-8 w-1.5 cursor-help rounded-sm',
                  isHealthy && 'bg-green-500',
                  isWarning && 'bg-yellow-500',
                  isError && 'bg-red-500',
                  isIdle && 'bg-gray-200'
                )}
              />
            </TooltipTrigger>
            <TooltipContent>
              <div className='space-y-1 text-xs'>
                <div>{t('channels.columns.healthTooltip.probeTime')}: {probeTime}</div>
                <div>{t('channels.columns.healthTooltip.successRate')}: {point.successRequestCount}/{point.totalRequestCount}</div>
              </div>
            </TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
});

ChannelHealthCell.displayName = 'ChannelHealthCell';
