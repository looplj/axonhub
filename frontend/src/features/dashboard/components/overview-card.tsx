import { useTranslation } from 'react-i18next'
import { Users } from 'lucide-react'

import { formatNumber } from '@/utils/format-number'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useDashboardStats } from '../data/dashboard'

export function OverviewCard() {
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
          <div className='space-y-4'>
            <div className='flex gap-4'>
              <div className='flex-1'>
                <Skeleton className='h-8 w-[80px]' />
                <Skeleton className='h-4 w-[140px] mt-1' />
              </div>
              <div className='border-border border-l' />
              <div className='flex-1'>
                <Skeleton className='h-8 w-[80px]' />
                <Skeleton className='h-4 w-[140px] mt-1' />
              </div>
            </div>
            <div className='flex gap-4'>
              <div className='flex-1'>
                <Skeleton className='h-8 w-[80px]' />
                <Skeleton className='h-4 w-[140px] mt-1' />
              </div>
              <div className='border-border border-l' />
              <div className='flex-1'>
                <Skeleton className='h-8 w-[80px]' />
                <Skeleton className='h-4 w-[140px] mt-1' />
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <CardTitle className='text-sm font-medium'>{t('dashboard.cards.overview')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <CardTitle className='text-sm font-medium'>{t('dashboard.cards.overview')}</CardTitle>
        <div className='bg-primary/10 text-primary flex h-9 w-9 items-center justify-center rounded-full dark:bg-primary/20'>
          <Users className='h-4 w-4' />
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-4'>
          <div className='flex gap-4'>
            <div className='flex-1'>
              <div className='text-2xl font-bold'>{formatNumber(stats?.totalUsers || 0)}</div>
              <p className='text-muted-foreground text-xs'>{t('dashboard.stats.totalUsersInSystem')}</p>
            </div>

            {/* Divider */}
            <div className='border-border border-l'></div>

            <div className='flex-1'>
              <div className='text-2xl font-bold'>{formatNumber(stats?.totalRequests || 0)}</div>
              <p className='text-muted-foreground text-xs'>{t('dashboard.stats.allTimeRequests')}</p>
            </div>
          </div>

          <div className='flex gap-4'>
            <div className='flex-1'>
              <div className='text-2xl font-bold'>{formatNumber(stats?.failedRequests || 0)}</div>
              <p className='text-muted-foreground text-xs'>{t('dashboard.stats.failedRequests')}</p>
            </div>

            {/* Divider */}
            <div className='border-border border-l'></div>

            <div className='flex-1'>
              <div className='text-2xl font-bold'>
                {stats && stats.totalRequests > 0
                  ? (((stats.totalRequests - stats.failedRequests) / stats.totalRequests) * 100).toFixed(1)
                  : '0.0'}
                %
              </div>
              <p className='text-muted-foreground text-xs'>{t('dashboard.cards.successRate')}</p>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
