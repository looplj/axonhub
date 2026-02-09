import { useTranslation } from 'react-i18next';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent } from '@/components/ui/tabs';
import { Header } from '@/components/layout/header';
import { ChannelSuccessRate } from './components/channel-success-rate';
import { DailyRequestStats } from './components/daily-requests-stats';
import { RequestsByChannelChart } from './components/requests-by-channel-chart';
import { RequestsByModelChart } from './components/requests-by-model-chart';
import { RequestsByAPIKeyChart } from './components/requests-by-api-key-chart';
import { TokensByAPIKeyChart } from './components/tokens-by-api-key-chart';
import { SuccessRateCard } from './components/success-rate-card';
import { TodayRequestsCard } from './components/today-requests-card';
import { TokenStatsCard } from './components/token-stats-card';
import { TotalRequestsCard } from './components/total-requests-card';
import { FastestChannelsCard } from './components/fastest-channels-card';
import { FastestModelsCard } from './components/fastest-models-card';
import { useDashboardStats } from './data/dashboard';

export default function DashboardPage() {
  const { t } = useTranslation();
  const { isLoading, error } = useDashboardStats();

  if (isLoading) {
    return (
      <div className='flex-1 space-y-4 p-8 pt-6'>
        <div className='flex items-center justify-between space-y-2'>
          <Skeleton className='h-8 w-[200px]' />
          <div className='flex items-center space-x-2'>
            <Skeleton className='h-10 w-[200px]' />
            <Skeleton className='h-10 w-[100px]' />
          </div>
        </div>
        <Tabs defaultValue='overview' className='space-y-4'>
          <Skeleton className='h-10 w-[400px]' />
          <div className='space-y-4'>
            <div className='grid gap-4 md:grid-cols-1 lg:grid-cols-4'>
              <Skeleton className='h-[180px]' />
              <Skeleton className='h-[180px]' />
              <Skeleton className='h-[180px]' />
              <Skeleton className='h-[180px]' />
            </div>
            <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
              <Skeleton className='col-span-4 h-[300px]' />
              <Skeleton className='col-span-3 h-[300px]' />
            </div>
          </div>
        </Tabs>
      </div>
    );
  }

  if (error) {
    return (
      <div className='flex-1 space-y-4 p-8 pt-6'>
        <div className='text-red-500'>
          {t('common.loadError')} {error.message}
        </div>
      </div>
    );
  }

  return (
    <div className='flex-1 space-y-4 p-8 pt-6'>
      <Header>{/* <TopNav links={topNav} /> */}</Header>
      <Tabs defaultValue='overview' className='space-y-4'>
        <TabsContent value='overview' className='space-y-4'>
          <div className='grid gap-6 md:grid-cols-2 lg:grid-cols-4'>
            <TotalRequestsCard />
            <SuccessRateCard />
            <TokenStatsCard />
            <TodayRequestsCard />
          </div>
          <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
            <Card className='hover-card col-span-4'>
              <CardHeader>
                <CardTitle>{t('dashboard.charts.dailyRequestOverview')}</CardTitle>
              </CardHeader>
              <CardContent className='pl-2'>
                <DailyRequestStats />
              </CardContent>
            </Card>
            <Card className='hover-card col-span-3'>
              <CardHeader>
                <CardTitle>{t('dashboard.charts.channelSuccessRate')}</CardTitle>
                <CardDescription>{t('dashboard.charts.channelSuccessRateDescription')}</CardDescription>
              </CardHeader>
              <CardContent>
                <ChannelSuccessRate />
              </CardContent>
            </Card>
          </div>
          <div className='grid gap-4 md:grid-cols-2'>
            <Card className='hover-card'>
              <CardHeader>
                <CardTitle>{t('dashboard.charts.requestsByChannel')}</CardTitle>
                <CardDescription>{t('dashboard.charts.requestsByChannelDescription')}</CardDescription>
              </CardHeader>
              <CardContent>
                <RequestsByChannelChart />
              </CardContent>
            </Card>
            <Card className='hover-card'>
              <CardHeader>
                <CardTitle>{t('dashboard.charts.requestsByModel')}</CardTitle>
                <CardDescription>{t('dashboard.charts.requestsByModelDescription')}</CardDescription>
              </CardHeader>
              <CardContent>
                <RequestsByModelChart />
              </CardContent>
            </Card>
          </div>
          <div className='grid gap-4 md:grid-cols-2'>
            <Card className='hover-card'>
              <CardHeader>
                <CardTitle>{t('dashboard.charts.requestsByAPIKey')}</CardTitle>
                <CardDescription>{t('dashboard.charts.requestsByAPIKeyDescription')}</CardDescription>
              </CardHeader>
              <CardContent>
                <RequestsByAPIKeyChart />
              </CardContent>
            </Card>
            <Card className='hover-card'>
              <CardHeader>
                <CardTitle>{t('dashboard.charts.tokensByAPIKey')}</CardTitle>
                <CardDescription>{t('dashboard.charts.tokensByAPIKeyDescription')}</CardDescription>
              </CardHeader>
              <CardContent>
                <TokensByAPIKeyChart />
              </CardContent>
            </Card>
          </div>
          <div className='grid gap-4 md:grid-cols-2'>
            <FastestChannelsCard />
            <FastestModelsCard />
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
