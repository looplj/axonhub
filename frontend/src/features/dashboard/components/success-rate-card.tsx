import { useTranslation } from 'react-i18next'
import { ShieldCheck } from 'lucide-react'

import { formatNumber } from '@/utils/format-number'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Progress } from '@/components/ui/progress'
import { useDashboardStats } from '../data/dashboard'

export function SuccessRateCard() {
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
            <Skeleton className='h-2 w-full mt-2' />
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
              <ShieldCheck className='h-4 w-4' />
            </div>
            <CardTitle className='text-sm font-medium'>{t('dashboard.cards.successRate')}</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    )
  }

  const successRate = stats && stats.totalRequests > 0
    ? (((stats.totalRequests - stats.failedRequests) / stats.totalRequests) * 100).toFixed(1)
    : '0.0'

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex items-center gap-2'>
          <div className='p-1.5 bg-primary/10 text-primary rounded-lg dark:bg-primary/20'>
            <ShieldCheck className='h-4 w-4' />
          </div>
          <CardTitle className='text-sm font-medium'>{t('dashboard.cards.successRate')}</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-3'>
          <div className='flex items-end justify-between'>
            <div className='text-3xl font-bold font-mono'>{successRate}<span className='text-lg text-muted-foreground'>%</span></div>
          </div>
          <Progress value={parseFloat(successRate)} className='h-2' />
          <div className='flex justify-between text-xs'>
            <span className='text-muted-foreground'>{formatNumber(stats?.failedRequests || 0)} {t('dashboard.stats.failedRequests')}</span>
            <span className='text-primary font-medium'>{t('dashboard.stats.average')}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}