import { useTranslation } from 'react-i18next'
import { useChannelSuccessRates } from '../data/dashboard'
import { Skeleton } from '@/components/ui/skeleton'
import { ActivityIcon, CheckCircle2Icon, XCircleIcon } from 'lucide-react'
import { formatNumber } from '@/utils/format-number'

export function ChannelSuccessRate() {
  const { t } = useTranslation()
  const { data: channels, isLoading, error } = useChannelSuccessRates()

  if (isLoading) {
    return (
      <div className="space-y-8">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center">
            <Skeleton className="h-9 w-9 rounded-md" />
            <div className="ml-4 space-y-1">
              <Skeleton className="h-4 w-[120px]" />
              <Skeleton className="h-3 w-[160px]" />
            </div>
            <Skeleton className="ml-auto h-4 w-[60px]" />
          </div>
        ))}
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-red-500 text-sm">
        {t('dashboard.charts.errorLoadingChannelSuccessRate')} {error.message}
      </div>
    )
  }

  if (!channels || channels.length === 0) {
    return (
      <div className="text-muted-foreground text-sm">
        {t('dashboard.charts.noChannelData')}
      </div>
    )
  }

  return (
    <div className="space-y-8">
      {channels.map((channel) => (
        <div key={channel.channelId} className="flex items-center">
          <div className="flex h-9 w-9 items-center justify-center rounded-md bg-primary/10">
            <ActivityIcon className="h-5 w-5 text-primary" />
          </div>
          <div className="ml-4 space-y-1">
            <p className="text-sm font-medium leading-none">{channel.channelName || '-'}</p>
            <div className="flex gap-3 text-sm text-muted-foreground">
              <span className="flex items-center gap-1">
                <CheckCircle2Icon className="h-3 w-3 text-green-500" />
                {formatNumber(channel.successCount)}
              </span>
              <span className="flex items-center gap-1">
                <XCircleIcon className="h-3 w-3 text-red-500" />
                {formatNumber(channel.failedCount)}
              </span>
            </div>
          </div>
          <div className="ml-auto font-medium">
            {channel.successRate.toFixed(1)}%
          </div>
        </div>
      ))}
    </div>
  )
}
