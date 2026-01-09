import { memo } from 'react';
import { cn } from '@/lib/utils';
import { ChannelProbePoint } from '../data/schema';

interface ChannelHealthCellProps {
  points: ChannelProbePoint[];
}

export const ChannelHealthCell = memo(({ points }: ChannelHealthCellProps) => {
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

        return (
          <div
            key={`${point.timestamp}-${index}`}
            className={cn(
              'h-8 w-1.5 rounded-sm',
              isHealthy && 'bg-green-500',
              isWarning && 'bg-yellow-500',
              isError && 'bg-red-500',
              isIdle && 'bg-gray-200'
            )}
            title={`${new Date(point.timestamp * 1000).toLocaleString()}\nSuccess: ${point.successRequestCount}/${point.totalRequestCount}`}
          />
        );
      })}
    </div>
  );
});

ChannelHealthCell.displayName = 'ChannelHealthCell';
