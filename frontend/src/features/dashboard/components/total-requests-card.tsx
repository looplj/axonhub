import { useTranslation } from 'react-i18next'
import { Database } from 'lucide-react'

import { formatNumber } from '@/utils/format-number'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useDashboardStats } from '../data/dashboard'

export function TotalRequestsCard() {
  const { t } = useTranslation()
  const { data: stats, isLoading, error } = useDashboardStats()

  const calculateGrowth = (current: number, previous: number): { percentage: number; isPositive: boolean } => {
    if (previous === 0) {
      return { percentage: current > 0 ? 100 : 0, isPositive: current > 0 }
    }
    const percentage = ((current - previous) / previous) * 100
    return { percentage, isPositive: percentage >= 0 }
  }

  const growth = stats?.requestStats
    ? calculateGrowth(stats.requestStats.requestsThisWeek, stats.requestStats.requestsLastWeek)
    : { percentage: 0, isPositive: true }

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
              <Database className='h-4 w-4' />
            </div>
            <CardTitle className='text-sm font-medium'>{t('dashboard.stats.allTimeRequests')}</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className='relative overflow-hidden hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex items-center gap-2'>
          <div className='p-1.5 bg-primary/10 text-primary rounded-lg dark:bg-primary/20'>
            <Database className='h-4 w-4' />
          </div>
          <CardTitle className='text-sm font-medium'>{t('dashboard.stats.allTimeRequests')}</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-2'>
          <div className='text-3xl font-bold font-mono'>{formatNumber(stats?.totalRequests || 0)}</div>
          <div className={`flex items-center gap-1 text-xs font-medium ${growth.isPositive ? 'text-primary' : 'text-red-500'}`}>
            <span className={`px-1.5 py-0.5 rounded-md ${growth.isPositive ? 'bg-primary/10 border border-primary/20' : 'bg-red-500/10 border border-red-500/20'}`}>
              {growth.isPositive ? '+' : ''}{growth.percentage.toFixed(0)}%
            </span>
            <span className='text-muted-foreground'>{t('dashboard.stats.vsLastWeek')}</span>
          </div>
        </div>
      </CardContent>
      <div className='absolute -bottom-4 -right-4 text-primary/5 opacity-50'>
        <div className='h-24 w-24'>
          <svg viewBox='0 0 24 24' fill='none' xmlns='http://www.w3.org/2000/svg'>
            <path d='M3 13.5C3 13.5 7 19 12 19C17 19 21 13.5 21 13.5' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' />
            <path d='M3 9C3 9 7 14 12 14C17 14 21 9 21 9' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' />
            <path d='M3 4.5C3 4.5 7 10 12 10C17 10 21 4.5 21 4.5' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' />
          </svg>
        </div>
      </div>
    </Card>
  )
}