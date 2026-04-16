import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ChannelRateLimitStatus } from '../data/schema';

interface ChannelRateLimitCellProps {
  status: ChannelRateLimitStatus | null | undefined;
}

function formatTimeRemaining(resetAt: string | null | undefined): string {
  if (!resetAt) return '';
  const reset = new Date(resetAt).getTime();
  const now = Date.now();
  const diffMs = reset - now;
  if (diffMs <= 0) return '';
  const totalSeconds = Math.floor(diffMs / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  if (hours > 0) return `${hours}h${minutes}m`;
  if (minutes > 0) return `${minutes}m`;
  return '<1m';
}

function formatResetTime(resetAt: string | null | undefined): string {
  if (!resetAt) return '';
  const d = new Date(resetAt);
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`;
}

export const ChannelRateLimitCell = memo(({ status }: ChannelRateLimitCellProps) => {
  const { t } = useTranslation();

  if (!status) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  const rpmRatio = status.rpmLimit != null && status.rpmCurrent != null && status.rpmLimit > 0
    ? status.rpmCurrent / status.rpmLimit : -1;
  const tpmRatio = status.tpmLimit != null && status.tpmCurrent != null && status.tpmLimit > 0
    ? status.tpmCurrent / status.tpmLimit : -1;

  const maxRatio = Math.max(rpmRatio, tpmRatio);
  const isCritical = maxRatio >= 1;
  const isHigh = maxRatio >= 0.8 && maxRatio < 1;

  const rateLimitSegments: { label: string; current: number; limit: number; resetAt?: string | null }[] = [];
  if (status.rpmCurrent != null && status.rpmLimit != null) {
    rateLimitSegments.push({ label: t('channels.expandedRow.rateLimit.requests'), current: status.rpmCurrent, limit: status.rpmLimit, resetAt: status.rpmResetAt });
  }
  if (status.tpmCurrent != null && status.tpmLimit != null) {
    rateLimitSegments.push({ label: t('channels.expandedRow.rateLimit.tokens'), current: status.tpmCurrent, limit: status.tpmLimit, resetAt: status.tpmResetAt });
  }

  const hasConcurrent = status.concurrentCurrent != null && status.concurrentLimit != null;
  const hasRateLimits = rateLimitSegments.length > 0;

  if (!hasRateLimits && !hasConcurrent) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  const primary = rateLimitSegments.reduce((a, b) => {
    const ra = a.limit > 0 ? a.current / a.limit : 0;
    const rb = b.limit > 0 ? b.current / b.limit : 0;
    return ra > rb ? a : b;
  }, rateLimitSegments[0]);

  const pct = hasRateLimits && primary.limit > 0 ? Math.round((primary.current / primary.limit) * 100) : 0;
  const concurrentPct = hasConcurrent && status.concurrentLimit! > 0
    ? Math.round((status.concurrentCurrent! / status.concurrentLimit!) * 100) : 0;
  const concurrentFull = hasConcurrent && status.concurrentCurrent! >= status.concurrentLimit!;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className='flex flex-col items-center gap-0.5'>
          {hasRateLimits ? (
            <div className='flex items-center gap-1'>
              <div className='bg-muted h-1.5 w-10 overflow-hidden rounded-full'>
                <div
                  className={`h-full rounded-full transition-all ${isCritical ? 'bg-destructive' : isHigh ? 'bg-yellow-500' : 'bg-primary'}`}
                  style={{ width: `${Math.min(pct, 100)}%` }}
                />
              </div>
              <span className={`font-mono text-xs ${isCritical ? 'text-destructive font-semibold' : isHigh ? 'text-yellow-600 font-semibold' : 'text-muted-foreground'}`}>
                {pct}%
              </span>
            </div>
          ) : (
            <div className='h-3' />
          )}
          {hasConcurrent && (
            <div className='flex items-center gap-1'>
              <div className='bg-muted h-1 w-10 overflow-hidden rounded-full'>
                <div
                  className={`h-full rounded-full transition-all ${concurrentFull ? 'bg-destructive' : concurrentPct >= 80 ? 'bg-yellow-500' : 'bg-blue-400'}`}
                  style={{ width: `${Math.min(concurrentPct, 100)}%` }}
                />
              </div>
              <span className={`font-mono text-[10px] ${concurrentFull ? 'text-destructive font-semibold' : 'text-muted-foreground'}`}>
                {status.concurrentCurrent}/{status.concurrentLimit}
              </span>
            </div>
          )}
        </div>
      </TooltipTrigger>
      <TooltipContent>
        <div className='space-y-1 text-xs'>
          {rateLimitSegments.map((s) => (
            <div key={s.label} className='flex justify-between gap-4'>
              <span className='text-muted-foreground'>{s.label}:</span>
              <span className='font-mono'>{s.current}/{s.limit}</span>
            </div>
          ))}
          {hasConcurrent && (
            <div className='flex justify-between gap-4'>
              <span className='text-muted-foreground'>{t('channels.expandedRow.rateLimit.concurrent')}:</span>
              <span className='font-mono'>{status.concurrentCurrent}/{status.concurrentLimit}</span>
            </div>
          )}
          {rateLimitSegments.some((s) => s.resetAt) && (
            <div className='border-t pt-1 mt-1'>
              {rateLimitSegments.filter((s) => s.resetAt).map((s) => (
                <div key={s.label} className='flex justify-between gap-4'>
                  <span className='text-muted-foreground'>{s.label} {t('channels.expandedRow.rateLimit.resets')}:</span>
                  <span className='font-mono'>{formatTimeRemaining(s.resetAt)} ({formatResetTime(s.resetAt)})</span>
                </div>
              ))}
            </div>
          )}
          {status.isCoolingDown && status.cooldownUntil && (
            <div className='text-destructive font-semibold'>
              {t('channels.expandedRow.rateLimit.cooldown')}: {formatTimeRemaining(status.cooldownUntil)}
            </div>
          )}
        </div>
      </TooltipContent>
    </Tooltip>
  );
});

ChannelRateLimitCell.displayName = 'ChannelRateLimitCell';
