import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { BarChart3, Key, Zap, ChevronDown } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
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

interface CollapsibleSectionProps {
  title: string;
  icon: React.ReactNode;
  children: React.ReactNode;
  defaultOpen?: boolean;
}

function CollapsibleSection({ title, icon, children, defaultOpen = false }: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen);

  return (
    <div className='space-y-4'>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className='flex w-full items-center justify-between rounded-lg border bg-card p-4 text-left transition-colors hover:bg-accent/50'
      >
        <div className='flex items-center gap-3'>
          <div className='flex h-8 w-8 items-center justify-center rounded-md bg-primary/10'>
            {icon}
          </div>
          <span className='text-lg font-semibold'>{title}</span>
        </div>
        <motion.div
          animate={{ rotate: isOpen ? 180 : 0 }}
          transition={{ duration: 0.2, ease: 'easeInOut' }}
        >
          <ChevronDown className='h-5 w-5 text-muted-foreground' />
        </motion.div>
      </button>
      <AnimatePresence initial={false}>
        {isOpen && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.3, ease: 'easeInOut' }}
            className='overflow-hidden'
          >
            <div className='space-y-4'>{children}</div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

export default function DashboardPage() {
  const { t } = useTranslation();
  const { isLoading, error } = useDashboardStats();

  if (isLoading) {
    return (
      <div className='flex-1 space-y-4 p-8 pt-6'>
        <div className='flex items-center justify-between space-y-2'>
          <Skeleton className='h-8 w-[200px]' />
        </div>
        <div className='space-y-4'>
          <div className='grid gap-4 md:grid-cols-1 lg:grid-cols-4'>
            <Skeleton className='h-[180px]' />
            <Skeleton className='h-[180px]' />
            <Skeleton className='h-[180px]' />
            <Skeleton className='h-[180px]' />
          </div>
          <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
            <Skeleton className='col-span-1 h-[300px] lg:col-span-4' />
            <Skeleton className='col-span-1 h-[300px] lg:col-span-3' />
          </div>
        </div>
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
    <div className='flex-1 space-y-6 p-8 pt-6'>
      <Header />

      {/* 概览部分 - 始终展示 */}
      <section className='space-y-4'>
        {/* <h2 className='text-2xl font-bold tracking-tight'>{t('dashboard.sections.overview')}</h2> */}
        <div className='grid gap-6 md:grid-cols-2 lg:grid-cols-4'>
          <TotalRequestsCard />
          <SuccessRateCard />
          <TokenStatsCard />
          <TodayRequestsCard />
        </div>
        <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
          <Card className='hover-card col-span-1 lg:col-span-4'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.dailyRequestOverview')}</CardTitle>
            </CardHeader>
            <CardContent className='pl-2'>
              <DailyRequestStats />
            </CardContent>
          </Card>
          <Card className='hover-card col-span-1 lg:col-span-3'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.channelSuccessRate')}</CardTitle>
              <CardDescription>{t('dashboard.charts.channelSuccessRateDescription')}</CardDescription>
            </CardHeader>
            <CardContent>
              <ChannelSuccessRate />
            </CardContent>
          </Card>
        </div>
      </section>

      {/* 渠道分析 - 可折叠 */}
      <CollapsibleSection
        title={t('dashboard.sections.channels')}
        icon={<BarChart3 className='h-4 w-4 text-primary' />}
      >
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
      </CollapsibleSection>

      {/* API密钥分析 - 可折叠 */}
      <CollapsibleSection
        title={t('dashboard.sections.apiKeys')}
        icon={<Key className='h-4 w-4 text-primary' />}
      >
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
      </CollapsibleSection>

      {/* 性能分析 - 可折叠 */}
      <CollapsibleSection
        title={t('dashboard.sections.performance')}
        icon={<Zap className='h-4 w-4 text-primary' />}
      >
        <div className='grid gap-4 md:grid-cols-2'>
          <FastestChannelsCard />
          <FastestModelsCard />
        </div>
      </CollapsibleSection>
    </div>
  );
}
