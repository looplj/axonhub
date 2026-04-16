import { memo } from 'react';
import { format } from 'date-fns';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { CHANNEL_CONFIGS } from '../data/config_channels';
import { Channel, ChannelRateLimitStatus } from '../data/schema';

function formatTimeRemaining(resetAt: string | null | undefined): string {
  if (!resetAt) return '';
  const reset = new Date(resetAt).getTime();
  const now = Date.now();
  const diffMs = reset - now;
  if (diffMs <= 0) return '';

  const totalSeconds = Math.floor(diffMs / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  if (minutes > 0) {
    return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
  }
  return `${seconds}s`;
}

function formatResetTime(resetAt: string | null | undefined): string {
  if (!resetAt) return '';
  const d = new Date(resetAt);
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`;
}

interface RateLimitMetricProps {
  label: string;
  current: number;
  limit: number | null | undefined;
  resetAt: string | null | undefined;
  windowDuration: string;
}

function RateLimitMetric({ label, current, limit, resetAt, windowDuration }: RateLimitMetricProps) {
  const { t } = useTranslation();
  const timeRemaining = formatTimeRemaining(resetAt);
  const usageRatio = limit != null && limit > 0 ? current / limit : 0;
  const isHigh = usageRatio >= 0.8;
  const isCritical = usageRatio >= 1;

  return (
    <div className='flex items-center justify-between gap-2 text-sm'>
      <span className='text-muted-foreground shrink-0'>{label}:</span>
      <div className='flex items-center gap-2'>
        <span className={`font-mono text-xs ${isCritical ? 'text-destructive font-semibold' : isHigh ? 'text-yellow-600 font-semibold' : ''}`}>
          {current}{limit != null ? `/${limit}` : ''}
        </span>
        {timeRemaining && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className='text-muted-foreground font-mono text-xs'>
                {timeRemaining} / {windowDuration || '?'}
              </span>
            </TooltipTrigger>
            <TooltipContent>
              <div className='space-y-1 text-xs'>
                <div className='flex justify-between gap-3'>
                  <span className='text-muted-foreground'>{t('channels.expandedRow.rateLimit.resets')}:</span>
                  <span className='font-mono'>{formatResetTime(resetAt)}</span>
                </div>
                <div className='text-muted-foreground'>
                  {t('channels.expandedRow.rateLimit.windowAligned')}
                </div>
              </div>
            </TooltipContent>
          </Tooltip>
        )}
      </div>
    </div>
  );
}

interface ConcurrentMetricProps {
  current: number;
  limit: number | null | undefined;
}

function ConcurrentMetric({ current, limit }: ConcurrentMetricProps) {
  const { t } = useTranslation();
  const isFull = limit != null && limit > 0 && current >= limit;
  const isHigh = limit != null && limit > 0 && current / limit >= 0.8 && !isFull;

  return (
    <div className='flex items-center justify-between gap-2 text-sm'>
      <span className='text-muted-foreground shrink-0'>{t('channels.expandedRow.rateLimit.concurrent')}:</span>
      <div className='flex items-center gap-2'>
        <div className='bg-muted h-1.5 w-16 overflow-hidden rounded-full'>
          <div
            className={`h-full rounded-full transition-all ${isFull ? 'bg-destructive' : isHigh ? 'bg-yellow-500' : 'bg-blue-400'}`}
            style={{ width: `${limit != null && limit > 0 ? Math.min((current / limit) * 100, 100) : 0}%` }}
          />
        </div>
        <span className={`font-mono text-xs ${isFull ? 'text-destructive font-semibold' : isHigh ? 'text-yellow-600 font-semibold' : ''}`}>
          {current}{limit != null ? `/${limit}` : ''}
        </span>
      </div>
    </div>
  );
}

interface RateLimitStatusSectionProps {
  status: ChannelRateLimitStatus;
  rpmDuration: string | null | undefined;
  tpmDuration: string | null | undefined;
  costDuration: string | null | undefined;
}

function RateLimitStatusSection({ status, rpmDuration, tpmDuration, costDuration }: RateLimitStatusSectionProps) {
  const { t } = useTranslation();

  const durationKeyMap: Record<string, string> = {
    ONE_MIN: 'channels.dialogs.rateLimit.durations.1min',
    ONE_HOUR: 'channels.dialogs.rateLimit.durations.1hr',
    FIVE_HOUR: 'channels.dialogs.rateLimit.durations.5hr',
    ONE_WEEK: 'channels.dialogs.rateLimit.durations.1wk',
    ONE_MONTH: 'channels.dialogs.rateLimit.durations.1mo',
  };

  const formatWindowDuration = (d: string | null | undefined) => d ? t(durationKeyMap[d] ?? d) : '';
  const hasRpm = status.rpmCurrent != null;
  const hasTpm = status.tpmCurrent != null;
  const hasConcurrent = status.concurrentCurrent != null;
  const hasCost = status.costCurrent != null;
  const hasRateLimits = hasRpm || hasTpm || hasCost;

  return (
    <div className='space-y-2'>
      {hasRpm && (
        <RateLimitMetric
          label={t('channels.expandedRow.rateLimit.requests')}
          current={status.rpmCurrent}
          limit={status.rpmLimit}
          resetAt={status.rpmResetAt}
          windowDuration={formatWindowDuration(rpmDuration)}
        />
      )}
      {hasTpm && (
        <RateLimitMetric
          label={t('channels.expandedRow.rateLimit.tokens')}
          current={status.tpmCurrent}
          limit={status.tpmLimit}
          resetAt={status.tpmResetAt}
          windowDuration={formatWindowDuration(tpmDuration)}
        />
      )}
      {hasCost && (
        <RateLimitMetric
          label={t('channels.expandedRow.rateLimit.cost')}
          current={status.costCurrent}
          limit={status.costLimit}
          resetAt={status.costResetAt}
          windowDuration={formatWindowDuration(costDuration)}
        />
      )}
      {hasConcurrent && (
        <ConcurrentMetric
          current={status.concurrentCurrent}
          limit={status.concurrentLimit}
        />
      )}
      {status.isCoolingDown && status.cooldownUntil && (
        <div className='flex items-center justify-between text-sm'>
          <span className='text-muted-foreground'>{t('channels.expandedRow.rateLimit.cooldown')}:</span>
          <span className='text-destructive font-mono text-xs font-semibold'>
            {formatTimeRemaining(status.cooldownUntil)}
          </span>
        </div>
      )}
    </div>
  );
}

interface ChannelExpandedRowProps {
  channel: Channel;
  columnsLength: number;
  getApiFormatLabel: (apiFormat?: string) => string;
}

export const ChannelExpandedRow = memo(({ channel, columnsLength, getApiFormatLabel }: ChannelExpandedRowProps) => {
  const { t } = useTranslation();
  const config = CHANNEL_CONFIGS[channel.type];

  return (
    <div className='bg-muted/30 p-6 hover:bg-muted/50'>
      <div className='space-y-6'>
        <div className='grid grid-cols-1 gap-6 md:grid-cols-2'>
          <div className='space-y-3'>
            <h4 className='text-sm font-semibold'>{t('channels.expandedRow.basic')}</h4>
            <div className='space-y-2 text-sm'>
              <div className='flex items-start gap-2'>
                <span className='text-muted-foreground shrink-0'>{t('channels.columns.baseURL')}:</span>
                <span className='min-w-0 flex-1 text-right font-mono text-xs break-all'>{channel.baseURL}</span>
              </div>
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground'>{t('channels.columns.type')}:</span>
                <Badge variant='outline' className={config?.color}>
                  {t(`channels.types.${channel.type}`)}
                </Badge>
              </div>
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground'>{t('channels.expandedRow.apiFormat')}:</span>
                <span className='font-mono text-xs'>{getApiFormatLabel(config?.apiFormat)}</span>
              </div>
              <div className='flex justify-between'>
                <span className='text-muted-foreground'>{t('common.columns.createdAt')}:</span>
                <span>{format(channel.createdAt, 'yyyy-MM-dd HH:mm')}</span>
              </div>
              <div className='flex justify-between'>
                <span className='text-muted-foreground'>{t('common.columns.updatedAt')}:</span>
                <span>{format(channel.updatedAt, 'yyyy-MM-dd HH:mm')}</span>
              </div>
            </div>
          </div>

          <div className='space-y-6'>
            <div className='space-y-3'>
              <h4 className='text-sm font-semibold'>{t('channels.expandedRow.additional')}</h4>
              <div className='space-y-2 text-sm'>
                <div className='flex items-center justify-between'>
                  <span className='text-muted-foreground'>{t('channels.columns.orderingWeight')}:</span>
                  <span className='font-mono text-xs'>{channel.orderingWeight ?? 0}</span>
                </div>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('channels.expandedRow.remark')}:</span>
                  <span className='max-w-[200px] truncate text-right' title={channel.remark || undefined}>
                    {channel.remark || '-'}
                  </span>
                </div>
                <div className='flex items-start justify-between'>
                  <span className='text-muted-foreground shrink-0'>{t('channels.expandedRow.tags')}:</span>
                  <div className='flex max-w-[200px] flex-wrap justify-end gap-1'>
                    {channel.tags && channel.tags.length > 0 ? (
                      channel.tags.map((tag) => (
                        <Badge key={tag} variant='outline' className='text-xs'>
                          {tag}
                        </Badge>
                      ))
                    ) : (
                      <span>-</span>
                    )}
                  </div>
                </div>
              </div>
            </div>

          </div>
        </div>

        {channel.rateLimitStatus && (
          <div className='space-y-3'>
            <h4 className='text-sm font-semibold'>{t('channels.expandedRow.rateLimit.title')}</h4>
            <RateLimitStatusSection
              status={channel.rateLimitStatus}
              rpmDuration={channel.settings?.rateLimit?.rpmDuration}
              tpmDuration={channel.settings?.rateLimit?.tpmDuration}
              costDuration={channel.settings?.rateLimit?.costDuration}
            />
          </div>
        )}

        {channel.supportedModels && channel.supportedModels.length > 0 && (
          <div className='space-y-3'>
            <h4 className='text-sm font-semibold'>{t('channels.expandedRow.supportedModels')}</h4>
            <div className='flex flex-wrap gap-2'>
              {channel.supportedModels.slice(0, 5).map((model) => (
                <Badge key={model} variant='secondary' className='font-mono text-xs'>
                  {model}
                </Badge>
              ))}
              {channel.supportedModels.length > 5 && (
                <span className='text-muted-foreground flex items-center text-xs italic'>
                  {t('channels.expandedRow.moreModels', { count: channel.supportedModels.length - 5 })}
                </span>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
});

ChannelExpandedRow.displayName = 'ChannelExpandedRow';
