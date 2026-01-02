import { ShieldCheck } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { useDashboardStats } from '../data/dashboard';

export function SuccessRateCard() {
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
          <div className='space-y-2'>
            <Skeleton className='h-8 w-[80px]' />
            <Skeleton className='mt-1 h-4 w-[140px]' />
            <Skeleton className='mt-2 h-2 w-full' />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <div className='flex items-center gap-2'>
            <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
              <ShieldCheck className='h-4 w-4' />
            </div>
            <CardTitle className='text-sm font-medium'>{t('dashboard.cards.successRate')}</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  const successRate =
    stats && stats.totalRequests > 0 ? (((stats.totalRequests - stats.failedRequests) / stats.totalRequests) * 100).toFixed(1) : '0.0';

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex items-center gap-2'>
          <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
            <ShieldCheck className='h-4 w-4' />
          </div>
          <CardTitle className='text-sm font-medium'>{t('dashboard.cards.successRate')}</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-3'>
          <div className='flex items-end justify-between'>
            <div className='font-mono text-3xl font-bold'>
              {successRate}
              <span className='text-muted-foreground text-lg'>%</span>
            </div>
          </div>
          <Progress value={parseFloat(successRate)} className='h-2' />
          <div className='flex justify-between text-xs'>
            <span className='text-muted-foreground'>
              {formatNumber(stats?.failedRequests || 0)} {t('dashboard.stats.failedRequests')}
            </span>
            <span className='text-primary font-medium'>{t('dashboard.stats.average')}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
