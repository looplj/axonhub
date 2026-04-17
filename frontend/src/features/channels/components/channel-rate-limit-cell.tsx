import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { format, parseISO } from 'date-fns';

function formatUTCShort(iso: string): string {
  const d = parseISO(iso.endsWith('Z') || /[+-]\d{2}:?\d{2}$/.test(iso) ? iso : iso + 'Z');
  const yyyy = d.getUTCFullYear();
  const MM = String(d.getUTCMonth() + 1).padStart(2, '0');
  const dd = String(d.getUTCDate()).padStart(2, '0');
  const HH = String(d.getUTCHours()).padStart(2, '0');
  const mm = String(d.getUTCMinutes()).padStart(2, '0');
  return `${yyyy}-${MM}-${dd} ${HH}:${mm}`;
}
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

function formatCostValue(value: number): string {
  return value.toFixed(2);
}

function getBarColor(pct: number): string {
  if (pct >= 100) return 'bg-destructive';
  if (pct >= 80) return 'bg-yellow-500';
  return 'bg-primary';
}

function getTextColor(pct: number): string {
  if (pct >= 100) return 'text-destructive font-bold';
  if (pct >= 80) return 'text-yellow-600 font-semibold';
  return 'text-foreground';
}

export const ChannelRateLimitCell = memo(({ status }: ChannelRateLimitCellProps) => {
  const { t } = useTranslation();

  if (!status) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  const rateLimitSegments: { type: string; label: string; shortLabel: string; current: number; limit: number; resetAt?: string | null; isCost?: boolean; anchor?: string | null }[] = [];
  if (status.rpmCurrent != null && status.rpmLimit != null) {
    rateLimitSegments.push({ type: 'rpm', label: t('channels.expandedRow.rateLimit.requests'), shortLabel: 'Req', current: status.rpmCurrent, limit: status.rpmLimit, resetAt: status.rpmResetAt, anchor: status.rpmWindowAnchor });
  }
  if (status.tpmCurrent != null && status.tpmLimit != null) {
    rateLimitSegments.push({ type: 'tpm', label: t('channels.expandedRow.rateLimit.tokens'), shortLabel: 'Tok', current: status.tpmCurrent, limit: status.tpmLimit, resetAt: status.tpmResetAt, anchor: status.tpmWindowAnchor });
  }
  if (status.costCurrent != null && status.costLimit != null) {
    rateLimitSegments.push({ type: 'cost', label: t('channels.expandedRow.rateLimit.cost'), shortLabel: '$$$', current: status.costCurrent, limit: status.costLimit, resetAt: status.costResetAt, isCost: true, anchor: status.costWindowAnchor });
  }

  const hasConcurrent = status.concurrentCurrent != null && status.concurrentLimit != null;
  const hasRateLimits = rateLimitSegments.length > 0;

  if (!hasRateLimits && !hasConcurrent) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  const concurrentPct = hasConcurrent && status.concurrentLimit! > 0
    ? Math.floor((status.concurrentCurrent! / status.concurrentLimit!) * 100) : 0;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className='flex flex-col items-center gap-0.5'>
          {hasRateLimits ? (
            <div className='flex flex-col gap-0.5'>
              {rateLimitSegments.map((s) => {
                const pct = s.limit > 0 ? Math.floor((s.current / s.limit) * 100) : 0;
                return (
                  <div key={s.type} className='flex items-center gap-1'>
                    <span className='text-muted-foreground text-xs w-7 text-right shrink-0'>{s.shortLabel}</span>
                    <div className='bg-muted h-1.5 w-10 overflow-hidden rounded-full shrink-0'>
                      <div
                        className={`h-full rounded-full transition-all ${getBarColor(pct)}`}
                        style={{ width: `${Math.min(pct, 100)}%` }}
                      />
                    </div>
                    <span className={`font-mono text-xs ${getTextColor(pct)}`}>
                      {pct}%
                    </span>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className='h-4' />
          )}
          {hasConcurrent && (
            <div className='flex items-center gap-1'>
              <span className='text-muted-foreground text-xs w-7 text-right shrink-0'>Con</span>
              <div className='bg-muted h-1.5 w-10 overflow-hidden rounded-full shrink-0'>
                <div
                  className={`h-full rounded-full transition-all ${concurrentPct >= 100 ? 'bg-destructive' : concurrentPct >= 80 ? 'bg-yellow-500' : 'bg-blue-400'}`}
                  style={{ width: `${Math.min(concurrentPct, 100)}%` }}
                />
              </div>
              <span className={`font-mono text-xs ${getTextColor(concurrentPct)}`}>
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
              <span className='opacity-70'>{s.label}:</span>
              <span className='font-mono'>{s.isCost ? `${formatCostValue(s.current)}/${formatCostValue(s.limit)}` : `${s.current}/${s.limit}`}</span>
            </div>
          ))}
          {hasConcurrent && (
            <div className='flex justify-between gap-4'>
              <span className='opacity-70'>{t('channels.expandedRow.rateLimit.concurrent')}:</span>
              <span className='font-mono'>{status.concurrentCurrent}/{status.concurrentLimit}</span>
            </div>
          )}
          {rateLimitSegments.some((s) => s.resetAt) && (
            <div className='border-t pt-1 mt-1'>
              {rateLimitSegments.filter((s) => s.resetAt).map((s) => (
                <div key={s.label} className='flex justify-between gap-4'>
                  <span className='opacity-70'>{s.label} {t('channels.expandedRow.rateLimit.resets')}:</span>
                  <span className='font-mono'>{formatTimeRemaining(s.resetAt)} ({formatResetTime(s.resetAt)})</span>
                </div>
              ))}
            </div>
          )}
          {rateLimitSegments.some((s) => s.anchor) && (
            <div className='border-t pt-1 mt-1'>
              {rateLimitSegments.filter((s) => s.anchor).map((s) => (
                <div key={s.label} className='flex justify-between gap-4'>
                  <span className='opacity-70'>{s.label} {t('channels.expandedRow.rateLimit.anchor')}:</span>
                  <span className='font-mono'>{formatUTCShort(s.anchor!)} UTC</span>
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
