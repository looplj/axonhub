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

export const ChannelRateLimitCell = memo(({ status }: ChannelRateLimitCellProps) => {
  const { t } = useTranslation();

  if (!status) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  const rpmRatio = status.rpmLimit != null && status.rpmCurrent != null && status.rpmLimit > 0
    ? status.rpmCurrent / status.rpmLimit : -1;
  const tpmRatio = status.tpmLimit != null && status.tpmCurrent != null && status.tpmLimit > 0
    ? status.tpmCurrent / status.tpmLimit : -1;
  const concurrentRatio = status.concurrentLimit != null && status.concurrentCurrent != null && status.concurrentLimit > 0
    ? status.concurrentCurrent / status.concurrentLimit : -1;

  const maxRatio = Math.max(rpmRatio, tpmRatio, concurrentRatio);
  const isCritical = maxRatio >= 1;
  const isHigh = maxRatio >= 0.8 && maxRatio < 1;

  const segments: { label: string; current: number; limit: number; resetAt?: string | null }[] = [];
  if (status.rpmCurrent != null && status.rpmLimit != null) {
    segments.push({ label: t('channels.expandedRow.rateLimit.requests'), current: status.rpmCurrent, limit: status.rpmLimit, resetAt: status.rpmResetAt });
  }
  if (status.tpmCurrent != null && status.tpmLimit != null) {
    segments.push({ label: t('channels.expandedRow.rateLimit.tokens'), current: status.tpmCurrent, limit: status.tpmLimit, resetAt: status.tpmResetAt });
  }
  if (status.concurrentCurrent != null && status.concurrentLimit != null) {
    segments.push({ label: t('channels.expandedRow.rateLimit.concurrent'), current: status.concurrentCurrent, limit: status.concurrentLimit });
  }

  if (segments.length === 0) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  const primary = segments.reduce((a, b) => {
    const ra = a.limit > 0 ? a.current / a.limit : 0;
    const rb = b.limit > 0 ? b.current / b.limit : 0;
    return ra > rb ? a : b;
  });

  const pct = primary.limit > 0 ? Math.round((primary.current / primary.limit) * 100) : 0;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className='flex items-center justify-center gap-1'>
          <div className='bg-muted h-1.5 w-12 overflow-hidden rounded-full'>
            <div
              className={`h-full rounded-full transition-all ${isCritical ? 'bg-destructive' : isHigh ? 'bg-yellow-500' : 'bg-primary'}`}
              style={{ width: `${Math.min(pct, 100)}%` }}
            />
          </div>
          <span className={`font-mono text-xs ${isCritical ? 'text-destructive font-semibold' : isHigh ? 'text-yellow-600 font-semibold' : 'text-muted-foreground'}`}>
            {pct}%
          </span>
          {status.isCoolingDown && (
            <span className='text-destructive text-xs'>⏳</span>
          )}
        </div>
      </TooltipTrigger>
      <TooltipContent>
        <div className='space-y-1 text-xs'>
          {segments.map((s) => (
            <div key={s.label} className='flex justify-between gap-3'>
              <span className='text-muted-foreground'>{s.label}:</span>
              <span className='font-mono'>{s.current}/{s.limit}{s.resetAt ? ` (${formatTimeRemaining(s.resetAt)})` : ''}</span>
            </div>
          ))}
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
