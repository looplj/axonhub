import { useTranslation } from 'react-i18next'
import { BarChart4 } from 'lucide-react'

import { formatNumber } from '@/utils/format-number'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useTokenStats } from '../data/dashboard'

export function TokenStatsCard() {
  const { t } = useTranslation()
  const { data: stats, isLoading, error } = useTokenStats()

  if (isLoading) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-4 w-[120px]' />
          <Skeleton className='h-4 w-4' />
        </CardHeader>
        <CardContent>
          <Skeleton className='mb-2 h-8 w-[80px]' />
          <Skeleton className='h-4 w-[140px]' />
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <CardTitle className='text-sm font-medium'>{t('dashboard.cards.tokenStats')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <CardTitle className='text-sm font-medium'>{t('dashboard.cards.tokensByTime')}</CardTitle>
        <div className='bg-primary/10 text-primary flex h-9 w-9 items-center justify-center rounded-full dark:bg-primary/20'>
          <BarChart4 className='h-4 w-4' />
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-3'>
          {/* This month row */}
          <div className='flex items-center justify-between'>
            <span className='text-sm'>{t('dashboard.stats.thisMonth')}:</span>
            <div className='grid grid-cols-3 gap-3 text-xs'>
              <div className='flex flex-col items-center min-w-[3rem]'>
                <span className='text-muted-foreground text-center'>{t('dashboard.stats.input')}</span>
                <span className='font-semibold text-center'>{formatNumber(stats?.totalInputTokensThisMonth || 0)}</span>
              </div>
              <div className='flex flex-col items-center min-w-[3rem]'>
                <span className='text-muted-foreground text-center'>{t('dashboard.stats.output')}</span>
                <span className='font-semibold text-center'>{formatNumber(stats?.totalOutputTokensThisMonth || 0)}</span>
              </div>
              <div className='flex flex-col items-center min-w-[3rem]'>
                <span className='text-muted-foreground text-center'>{t('dashboard.stats.cached')}</span>
                <span className='font-semibold text-center'>{formatNumber(stats?.totalCachedTokensThisMonth || 0)}</span>
              </div>
            </div>
          </div>

          {/* This week row */}
          <div className='flex items-center justify-between'>
            <span className='text-sm'>{t('dashboard.stats.thisWeek')}:</span>
            <div className='grid grid-cols-3 gap-3 text-xs'>
              <div className='flex flex-col items-center min-w-[3rem]'>
                <span className='text-muted-foreground text-center'>{t('dashboard.stats.input')}</span>
                <span className='font-semibold text-center'>{formatNumber(stats?.totalInputTokensThisWeek || 0)}</span>
              </div>
              <div className='flex flex-col items-center min-w-[3rem]'>
                <span className='text-muted-foreground text-center'>{t('dashboard.stats.output')}</span>
                <span className='font-semibold text-center'>{formatNumber(stats?.totalOutputTokensThisWeek || 0)}</span>
              </div>
              <div className='flex flex-col items-center min-w-[3rem]'>
                <span className='text-muted-foreground text-center'>{t('dashboard.stats.cached')}</span>
                <span className='font-semibold text-center'>{formatNumber(stats?.totalCachedTokensThisWeek || 0)}</span>
              </div>
            </div>
          </div>

          {/* Today row */}
          <div className='flex items-center justify-between'>
            <span className='text-sm'>{t('dashboard.stats.today')}:</span>
            <div className='grid grid-cols-3 gap-3 text-xs'>
              <div className='flex flex-col items-center min-w-[3rem]'>
                <span className='text-muted-foreground text-center'>{t('dashboard.stats.input')}</span>
                <span className='font-semibold text-center'>{formatNumber(stats?.totalInputTokensToday || 0)}</span>
              </div>
              <div className='flex flex-col items-center min-w-[3rem]'>
                <span className='text-muted-foreground text-center'>{t('dashboard.stats.output')}</span>
                <span className='font-semibold text-center'>{formatNumber(stats?.totalOutputTokensToday || 0)}</span>
              </div>
              <div className='flex flex-col items-center min-w-[3rem]'>
                <span className='text-muted-foreground text-center'>{t('dashboard.stats.cached')}</span>
                <span className='font-semibold text-center'>{formatNumber(stats?.totalCachedTokensToday || 0)}</span>
              </div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
