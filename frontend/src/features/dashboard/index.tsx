import { useState, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from '@tanstack/react-router';
import { BarChart3, Brain, Key, Zap, ChevronDown } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Header } from '@/components/layout/header';
import { formatNumber } from '@/utils/format-number';
import { TimePeriodSelector, type TimePeriod } from '@/components/time-period-selector';
import { ChannelSuccessRate } from './components/channel-success-rate';
import { DailyRequestStats } from './components/daily-requests-stats';
import { RequestsByChannelChart } from './components/requests-by-channel-chart';
import { RequestsByModelChart } from './components/requests-by-model-chart';
import { RequestsByAPIKeyChart } from './components/requests-by-api-key-chart';
import { TokensByAPIKeyChart } from './components/tokens-by-api-key-chart';
import { TokensByChannelChart } from './components/tokens-by-channel-chart';
import { TokensByModelChart } from './components/tokens-by-model-chart';
import { SuccessRateCard } from './components/success-rate-card';
import { TodayRequestsCard } from './components/today-requests-card';
import { TokenStatsCard } from './components/token-stats-card';
import { TotalRequestsCard } from './components/total-requests-card';
import { FastestChannelsCard } from './components/fastest-channels-card';
import { FastestModelsCard } from './components/fastest-models-card';
import { ModelPerformanceStats } from './components/model-performance-stats';
import { ChannelPerformanceStats } from './components/channel-performance-stats';
import { useDashboardStats } from './data/dashboard';

interface CollapsibleSectionProps {
  title: string;
  icon: React.ReactNode;
  children: React.ReactNode;
  storageKey: string;
  defaultOpen?: boolean;
}

function CollapsibleSection({ title, icon, children, storageKey, defaultOpen = false }: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(() => {
    try {
      const stored = localStorage.getItem(`dashboard-section-${storageKey}`);
      return stored !== null ? stored === 'true' : defaultOpen;
    } catch {
      return defaultOpen;
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem(`dashboard-section-${storageKey}`, isOpen.toString());
    } catch {
      // Silently fail - persistence is a nice-to-have, not critical
    }
  }, [isOpen, storageKey]);

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
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15, ease: 'easeInOut' }}
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
  const [modelTotalRequests, setModelTotalRequests] = useState(0);
  const [channelTotalRequests, setChannelTotalRequests] = useState(0);

  const [channelTimePeriod, setChannelTimePeriod] = useState<TimePeriod>('allTime');
  const [channelTokensTimePeriod, setChannelTokensTimePeriod] = useState<TimePeriod>('allTime');
  const [modelTimePeriod, setModelTimePeriod] = useState<TimePeriod>('allTime');
  const [modelTokensTimePeriod, setModelTokensTimePeriod] = useState<TimePeriod>('allTime');
  const [apiKeyTimePeriod, setApiKeyTimePeriod] = useState<TimePeriod>('allTime');
  const [apiKeyTokensTimePeriod, setApiKeyTokensTimePeriod] = useState<TimePeriod>('allTime');

  const modelPerformanceDescription = useMemo(() => {
    return t('dashboard.charts.performanceDescription', { count: formatNumber(modelTotalRequests) });
  }, [t, modelTotalRequests]);

  const channelPerformanceDescription = useMemo(() => {
    return t('dashboard.charts.performanceDescription', { count: formatNumber(channelTotalRequests) });
  }, [t, channelTotalRequests]);

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
              <CardAction>
                <Link to='/dashboard/channel-success-rates' className='text-sm text-primary hover:underline'>
                  {t('dashboard.viewAll')}
                </Link>
              </CardAction>
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
        storageKey='channels'
      >
        <div className='grid gap-4 md:grid-cols-2'>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.requestsCostByChannel')}</CardTitle>
              <CardDescription>{t('dashboard.charts.requestsCostByChannelDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={channelTimePeriod} onChange={setChannelTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <RequestsByChannelChart timePeriod={channelTimePeriod} />
            </CardContent>
          </Card>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.tokensByChannel')}</CardTitle>
              <CardDescription>{t('dashboard.charts.tokensByChannelDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={channelTokensTimePeriod} onChange={setChannelTokensTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <TokensByChannelChart timePeriod={channelTokensTimePeriod} />
            </CardContent>
          </Card>
        </div>
      </CollapsibleSection>

      {/* 模型分析 - 可折叠 */}
      <CollapsibleSection
        title={t('dashboard.sections.models')}
        icon={<Brain className='h-4 w-4 text-primary' />}
        storageKey='models'
      >
        <div className='grid gap-4 md:grid-cols-2'>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.requestsCostByModel')}</CardTitle>
              <CardDescription>{t('dashboard.charts.requestsCostByModelDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={modelTimePeriod} onChange={setModelTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <RequestsByModelChart timePeriod={modelTimePeriod} />
            </CardContent>
          </Card>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.tokensByModel')}</CardTitle>
              <CardDescription>{t('dashboard.charts.tokensByModelDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={modelTokensTimePeriod} onChange={setModelTokensTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <TokensByModelChart timePeriod={modelTokensTimePeriod} />
            </CardContent>
          </Card>
        </div>
      </CollapsibleSection>

      {/* API密钥分析 - 可折叠 */}
      <CollapsibleSection
        title={t('dashboard.sections.apiKeys')}
        icon={<Key className='h-4 w-4 text-primary' />}
        storageKey='apiKeys'
      >
        <div className='grid gap-4 md:grid-cols-2'>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.requestsCostByAPIKey')}</CardTitle>
              <CardDescription>{t('dashboard.charts.requestsCostByAPIKeyDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={apiKeyTimePeriod} onChange={setApiKeyTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <RequestsByAPIKeyChart timePeriod={apiKeyTimePeriod} />
            </CardContent>
          </Card>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.tokensByAPIKey')}</CardTitle>
              <CardDescription>{t('dashboard.charts.tokensByAPIKeyDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={apiKeyTokensTimePeriod} onChange={setApiKeyTokensTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <TokensByAPIKeyChart timePeriod={apiKeyTokensTimePeriod} />
            </CardContent>
          </Card>
        </div>
      </CollapsibleSection>

      {/* 性能分析 - 可折叠 */}
      <CollapsibleSection
        title={t('dashboard.sections.performance')}
        icon={<Zap className='h-4 w-4 text-primary' />}
        storageKey='performance'
      >
        <div className='grid gap-4 md:grid-cols-1 lg:grid-cols-7'>
          <Card className='hover-card col-span-1 lg:col-span-4'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.modelPerformance')}</CardTitle>
              <CardDescription>{modelPerformanceDescription}</CardDescription>
            </CardHeader>
            <CardContent>
              <ModelPerformanceStats onTotalRequestsChange={setModelTotalRequests} />
            </CardContent>
          </Card>
          <div className='col-span-1 lg:col-span-3'>
            <FastestModelsCard />
          </div>
        </div>
        <div className='grid gap-4 md:grid-cols-1 lg:grid-cols-7'>
          <Card className='hover-card col-span-1 lg:col-span-4'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.channelPerformance')}</CardTitle>
              <CardDescription>{channelPerformanceDescription}</CardDescription>
            </CardHeader>
            <CardContent>
              <ChannelPerformanceStats onTotalRequestsChange={setChannelTotalRequests} />
            </CardContent>
          </Card>
          <div className='col-span-1 lg:col-span-3'>
            <FastestChannelsCard />
          </div>
        </div>
      </CollapsibleSection>
    </div>
  );
}
