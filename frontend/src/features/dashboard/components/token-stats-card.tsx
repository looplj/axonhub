import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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
          <div className='text-sm text-red-500'>{t('dashboard.loadError')}</div>
        </CardContent>
      </Card>
    )
  }

  const formatNumber = (num: number) => {
    if (num >= 1000000000) {
      return (num / 1000000000).toFixed(1) + 'B'
    }
    if (num >= 1000000) {
      return (num / 1000000).toFixed(1) + 'M'
    }
    if (num >= 1000) {
      return (num / 1000).toFixed(1) + 'K'
    }
    return num.toString()
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <CardTitle className='text-sm font-medium'>{t('dashboard.cards.tokensByTime')}</CardTitle>
        <svg
          xmlns='http://www.w3.org/2000/svg'
          viewBox='0 0 24 24'
          fill='none'
          stroke='currentColor'
          strokeLinecap='round'
          strokeLinejoin='round'
          strokeWidth='2'
          className='text-muted-foreground h-4 w-4'
        >
          <path d='M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6' />
        </svg>
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
