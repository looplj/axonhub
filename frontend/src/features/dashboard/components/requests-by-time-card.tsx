import { Clock } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useDashboardStats } from '../data/dashboard';

export function RequestsByTimeCard() {
  const { t } = useTranslation();
  const { data: stats, isLoading, error } = useDashboardStats();

  if (isLoading) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-4 w-[120px]' />
          <Skeleton className='h-4 w-4' />
        </CardHeader>
        <CardContent>
          <div className='space-y-3'>
            <div className='flex justify-between'>
              <Skeleton className='h-4 w-[100px]' />
              <Skeleton className='h-4 w-[60px]' />
            </div>
            <div className='flex justify-between'>
              <Skeleton className='h-4 w-[100px]' />
              <Skeleton className='h-4 w-[60px]' />
            </div>
            <div className='flex justify-between'>
              <Skeleton className='h-4 w-[100px]' />
              <Skeleton className='h-4 w-[60px]' />
            </div>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <CardTitle className='text-sm font-medium'>{t('dashboard.cards.requestsByTime')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <CardTitle className='text-sm font-medium'>{t('dashboard.cards.requestsByTime')}</CardTitle>
        <div className='bg-primary/10 text-primary dark:bg-primary/20 flex h-9 w-9 items-center justify-center rounded-full'>
          <Clock className='h-4 w-4' />
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-3'>
          <div className='flex justify-between text-sm'>
            <span>{t('dashboard.stats.thisMonth')}:</span>
            <span className='font-semibold'>{formatNumber(stats?.requestStats?.requestsThisMonth || 0)}</span>
          </div>
          <div className='flex justify-between text-sm'>
            <span>{t('dashboard.stats.thisWeek')}:</span>
            <span className='font-semibold'>{formatNumber(stats?.requestStats?.requestsThisWeek || 0)}</span>
          </div>
          <div className='flex justify-between text-sm'>
            <span>{t('dashboard.stats.today')}:</span>
            <span className='font-semibold'>{formatNumber(stats?.requestStats?.requestsToday || 0)}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
