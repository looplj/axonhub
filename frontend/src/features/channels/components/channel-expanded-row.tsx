import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useGeneralSettings } from '@/features/system/data/system';
import { CHANNEL_CONFIGS } from '../data/config_channels';
import { Channel, ChannelRateLimitStatus } from '../data/schema';
import { formatNumber } from '@/utils/format-number';
import { formatInTz, getTimezoneAbbrev } from '../utils/timezone';
import { formatTimeRemaining } from '../utils/format-time-remaining';

interface RateLimitMetricProps {
  label: string;
  current: number;
  limit: number | null | undefined;
  resetAt: string | null | undefined;
  windowDuration: string;
  rawDuration: string | null | undefined;
  isCost?: boolean;
  anchor?: string | null;
  timezone: string;
  tzAbbr: string;
}

function RateLimitMetric({
  label,
  current,
  limit,
  resetAt,
  windowDuration,
  rawDuration,
  isCost,
  anchor,
  timezone,
  tzAbbr,
}: RateLimitMetricProps) {
  const { t } = useTranslation();
  const timeRemaining = formatTimeRemaining(resetAt, 'detailed', rawDuration);
  const usagePct = limit != null && limit > 0 ? Math.floor((current / limit) * 100) : 0;
  const isHigh = usagePct >= 80;
  const isCritical = usagePct >= 100;

  return (
    <div className='space-y-1.5'>
      <div className='flex items-center justify-between gap-2 text-sm'>
        <span className='text-muted-foreground shrink-0'>{label}:</span>
        <div className='flex items-center gap-2'>
          <span
            className={`font-mono text-xs ${isCritical ? 'text-destructive font-semibold' : isHigh ? 'font-semibold text-yellow-600' : ''}`}
          >
            {isCost ? current.toFixed(2) : formatNumber(current)}
            {limit != null ? `/${isCost && limit != null ? limit.toFixed(2) : formatNumber(limit)}` : ''} ({usagePct}%)
          </span>
        </div>
      </div>
      <div className='bg-muted h-2 w-full overflow-hidden rounded-full'>
        <div
          className={`h-full rounded-full transition-all ${isCritical ? 'bg-destructive' : isHigh ? 'bg-yellow-500' : 'bg-primary'}`}
          style={{ width: `${Math.min(usagePct, 100)}%` }}
        />
      </div>
      {timeRemaining && (
        <div className='flex items-center justify-between gap-2 text-xs'>
          <span className='text-muted-foreground'>
            {t('channels.expandedRow.rateLimit.window')}: {windowDuration || '?'}
          </span>
          <div className='flex items-center gap-1.5'>
            <span className='text-muted-foreground'>{t('channels.expandedRow.rateLimit.nextResetIn')}:</span>
            <Tooltip>
              <TooltipTrigger asChild>
                <span
                  className={`font-mono ${isCritical ? 'text-destructive font-semibold' : isHigh ? 'font-semibold text-yellow-600' : 'font-medium'}`}
                >
                  {timeRemaining}
                </span>
              </TooltipTrigger>
              <TooltipContent>
                <div className='space-y-1 text-xs'>
                  <div className='flex justify-between gap-3'>
                    <span className='opacity-70'>{t('channels.expandedRow.rateLimit.nextResetAt')}:</span>
                    <span className='font-mono'>
                      {formatInTz(resetAt, timezone, 'yyyy-MM-dd HH')} {tzAbbr}
                    </span>
                  </div>
                  {anchor && (
                    <div className='flex justify-between gap-3'>
                      <span className='opacity-70'>{t('channels.expandedRow.rateLimit.anchor')}:</span>
                      <span className='font-mono'>
                        {formatInTz(anchor, timezone, 'yyyy-MM-dd HH:mm')} {tzAbbr}
                      </span>
                    </div>
                  )}

                </div>
              </TooltipContent>
            </Tooltip>
          </div>
        </div>
      )}
    </div>
  );
}

interface ConcurrentMetricProps {
  current: number;
  limit: number | null | undefined;
}

function ConcurrentMetric({ current, limit }: ConcurrentMetricProps) {
  const { t } = useTranslation();
  const usagePct = limit != null && limit > 0 ? Math.floor((current / limit) * 100) : 0;
  const isFull = limit != null && limit > 0 && current >= limit;
  const isHigh = usagePct >= 80 && !isFull;

  return (
    <div className='space-y-1.5'>
      <div className='flex items-center justify-between gap-2 text-sm'>
        <span className='text-muted-foreground shrink-0'>{t('channels.expandedRow.rateLimit.concurrent')}:</span>
        <span className={`font-mono text-xs ${isFull ? 'text-destructive font-semibold' : isHigh ? 'font-semibold text-yellow-600' : ''}`}>
          {current}
          {limit != null ? `/${limit}` : ''}
        </span>
      </div>
      <div className='bg-muted h-2 w-full overflow-hidden rounded-full'>
        <div
          className={`h-full rounded-full transition-all ${isFull ? 'bg-destructive' : isHigh ? 'bg-yellow-500' : 'bg-blue-400'}`}
          style={{ width: `${Math.min(usagePct, 100)}%` }}
        />
      </div>
    </div>
  );
}

interface RateLimitStatusSectionProps {
  status: ChannelRateLimitStatus;
  rpmDuration: string | null | undefined;
  tpmDuration: string | null | undefined;
  costDuration: string | null | undefined;
  rpmWindowAnchor?: string | null;
  tpmWindowAnchor?: string | null;
  costWindowAnchor?: string | null;
  timezone: string;
  tzAbbr: string;
}

function RateLimitStatusSection({
  status,
  rpmDuration,
  tpmDuration,
  costDuration,
  rpmWindowAnchor,
  tpmWindowAnchor,
  costWindowAnchor,
  timezone,
  tzAbbr,
}: RateLimitStatusSectionProps) {
  const { t } = useTranslation();

  const durationKeyMap: Record<string, string> = {
    ONE_MIN: 'channels.dialogs.rateLimit.durations.1min',
    ONE_HOUR: 'channels.dialogs.rateLimit.durations.1hr',
    FIVE_HOUR: 'channels.dialogs.rateLimit.durations.5hr',
    ONE_WEEK: 'channels.dialogs.rateLimit.durations.1wk',
    ONE_MONTH: 'channels.dialogs.rateLimit.durations.1mo',
  };

  const formatWindowDuration = (d: string | null | undefined) => (d ? t(durationKeyMap[d] ?? d) : '');
  const hasRpm = status.rpmCurrent != null;
  const hasTpm = status.tpmCurrent != null;
  const hasConcurrent = status.concurrentCurrent != null;
  const hasCost = status.costCurrent != null;

  return (
    <div className='space-y-2' aria-label={t('channels.expandedRow.rateLimit.label', 'Rate limit details')} role='region'>
      {hasRpm && (
        <RateLimitMetric
          label={t('channels.expandedRow.rateLimit.requests')}
          current={status.rpmCurrent}
          limit={status.rpmLimit}
          resetAt={status.rpmResetAt}
          windowDuration={formatWindowDuration(rpmDuration)}
          rawDuration={rpmDuration}
          anchor={rpmWindowAnchor}
          timezone={timezone}
          tzAbbr={tzAbbr}
        />
      )}
      {hasTpm && (
        <RateLimitMetric
          label={t('channels.expandedRow.rateLimit.tokens')}
          current={status.tpmCurrent}
          limit={status.tpmLimit}
          resetAt={status.tpmResetAt}
          windowDuration={formatWindowDuration(tpmDuration)}
          rawDuration={tpmDuration}
          anchor={tpmWindowAnchor}
          timezone={timezone}
          tzAbbr={tzAbbr}
        />
      )}
      {hasCost && (
        <RateLimitMetric
          label={t('channels.expandedRow.rateLimit.cost')}
          current={status.costCurrent}
          limit={status.costLimit}
          resetAt={status.costResetAt}
          windowDuration={formatWindowDuration(costDuration)}
          rawDuration={costDuration}
          isCost
          anchor={costWindowAnchor}
          timezone={timezone}
          tzAbbr={tzAbbr}
        />
      )}
      {hasConcurrent && <ConcurrentMetric current={status.concurrentCurrent} limit={status.concurrentLimit} />}
      {status.isCoolingDown && status.cooldownUntil && (
        <div className='flex items-center justify-between text-sm'>
          <span className='text-muted-foreground'>{t('channels.expandedRow.rateLimit.cooldown')}:</span>
          <span className='text-destructive font-mono text-xs font-semibold'>{formatTimeRemaining(status.cooldownUntil, 'detailed')}</span>
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
  const { data: generalSettings } = useGeneralSettings();
  const timezone = generalSettings?.timezone || 'UTC';
  const tzAbbr = getTimezoneAbbrev(timezone);

  return (
    <div className='bg-muted/30 hover:bg-muted/50 p-6'>
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
                <span>{formatInTz(channel.createdAt, timezone, 'yyyy-MM-dd HH:mm')}</span>
              </div>
              <div className='flex justify-between'>
                <span className='text-muted-foreground'>{t('common.columns.updatedAt')}:</span>
                <span>{formatInTz(channel.updatedAt, timezone, 'yyyy-MM-dd HH:mm')}</span>
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
              rpmWindowAnchor={channel.rateLimitStatus.rpmWindowAnchor}
              tpmWindowAnchor={channel.rateLimitStatus.tpmWindowAnchor}
              costWindowAnchor={channel.rateLimitStatus.costWindowAnchor}
              timezone={timezone}
              tzAbbr={tzAbbr}
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
