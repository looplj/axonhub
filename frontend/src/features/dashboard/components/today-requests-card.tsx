import { useTranslation } from 'react-i18next'
import { Activity } from 'lucide-react'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatNumber } from '@/utils/format-number'
import { useDashboardStats } from '../data/dashboard'

export function TodayRequestsCard() {
  const { t } = useTranslation()
  const { data: stats, isLoading, error } = useDashboardStats()

  if (isLoading) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-4 w-[120px]' />
          <Skeleton className='h-4 w-4' />
        </CardHeader>
        <CardContent>
          <div className='space-y-2'>
            <Skeleton className='h-8 w-[80px]' />
            <Skeleton className='h-4 w-[140px] mt-1' />
          </div>
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <div className='flex items-center gap-2'>
            <div className='p-1.5 bg-primary/10 text-primary rounded-lg dark:bg-primary/20'>
              <Activity className='h-4 w-4' />
            </div>
            <CardTitle className='text-sm font-medium'>{t('dashboard.stats.todayRequests')}</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className='bg-primary text-primary-foreground hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex items-center gap-2'>
          <Activity className='h-4 w-4 text-primary-foreground/70' />
          <CardTitle className='text-sm font-medium text-primary-foreground/90'>{t('dashboard.stats.todayRequests')}</CardTitle>
        </div>
        <div className='w-2 h-2 bg-primary-foreground rounded-full animate-ping' />
      </CardHeader>
      <CardContent>
        <div className='space-y-4'>
          <div className='text-4xl font-bold font-mono tracking-tight mt-2'>{formatNumber(stats?.requestStats?.requestsToday || 0)}</div>
          <div className='mt-4 pt-3 border-t border-primary-foreground/10 flex justify-between text-xs text-primary-foreground/70'>
            <span>{t('dashboard.stats.thisWeek')}: {formatNumber(stats?.requestStats?.requestsThisWeek || 0)}</span>
            <span>{t('dashboard.stats.thisMonth')}: {formatNumber(stats?.requestStats?.requestsThisMonth || 0)}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}