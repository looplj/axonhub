import { useState, useEffect, useMemo, useRef } from 'react';
import { Link } from '@tanstack/react-router';
import { motion, AnimatePresence } from 'framer-motion';
import { BarChart3, Brain, Key, Users, Zap, ChevronDown, ChevronRight, TrendingUp } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Header } from '@/components/layout/header';
import { TimePeriodSelector, type TimePeriod } from '@/components/time-period-selector';
import { ChannelPerformanceStats } from './components/channel-performance-stats';
import { ChannelSuccessRate } from './components/channel-success-rate';
import { DailyRequestStats } from './components/daily-requests-stats';
import { FastestChannelsCard } from './components/fastest-channels-card';
import { FastestModelsCard } from './components/fastest-models-card';
import { ModelPerformanceStats } from './components/model-performance-stats';
import { RequestsByAPIKeyChart } from './components/requests-by-api-key-chart';
import { RequestsByChannelChart } from './components/requests-by-channel-chart';
import { RequestsByModelChart } from './components/requests-by-model-chart';
import { SuccessRateCard } from './components/success-rate-card';
import { TodayRequestsCard } from './components/today-requests-card';
import { TokenStatsCard } from './components/token-stats-card';
import { TokensByAPIKeyChart } from './components/tokens-by-api-key-chart';
import { TokensByChannelChart } from './components/tokens-by-channel-chart';
import { TokensByModelChart } from './components/tokens-by-model-chart';
import { TokensByUserChart } from './components/tokens-by-user-chart';
import { TotalRequestsCard } from './components/total-requests-card';
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
  // Pre-expanded sections below the fold used to mount every widget (and fire
  // every aggregate query) during the initial page load. Defer mounting until
  // the section approaches the viewport; once visible it stays mounted.
  const [hasBeenVisible, setHasBeenVisible] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    try {
      localStorage.setItem(`dashboard-section-${storageKey}`, isOpen.toString());
    } catch {
      // Silently fail - persistence is a nice-to-have, not critical
    }
  }, [isOpen, storageKey]);

  useEffect(() => {
    if (hasBeenVisible) return;

    const node = containerRef.current;
    if (!node) return;

    if (typeof IntersectionObserver === 'undefined') {
      setHasBeenVisible(true);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setHasBeenVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: '240px 0px' }
    );

    observer.observe(node);
    return () => observer.disconnect();
  }, [hasBeenVisible]);

  return (
    <div ref={containerRef} className='space-y-4'>
      <button
        type='button'
        onClick={() => setIsOpen(!isOpen)}
        className='bg-card hover:bg-accent/50 flex w-full items-center justify-between rounded-lg border p-4 text-left transition-colors'
      >
        <div className='flex items-center gap-3'>
          <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-md'>{icon}</div>
          <span className='text-lg font-semibold'>{title}</span>
        </div>
        <motion.div animate={{ rotate: isOpen ? 180 : 0 }} transition={{ duration: 0.2, ease: 'easeInOut' }}>
          <ChevronDown className='text-muted-foreground h-5 w-5' />
        </motion.div>
      </button>
      <AnimatePresence initial={false}>
        {isOpen && hasBeenVisible && (
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
  const { isProjectOwner } = useRoutePermissions();
  const [modelTotalRequests, setModelTotalRequests] = useState(0);
  const [channelTotalRequests, setChannelTotalRequests] = useState(0);

  const [channelTimePeriod, setChannelTimePeriod] = useState<TimePeriod>('allTime');
  const [channelTokensTimePeriod, setChannelTokensTimePeriod] = useState<TimePeriod>('allTime');
  const [modelTimePeriod, setModelTimePeriod] = useState<TimePeriod>('allTime');
  const [modelTokensTimePeriod, setModelTokensTimePeriod] = useState<TimePeriod>('allTime');
  const [apiKeyTimePeriod, setApiKeyTimePeriod] = useState<TimePeriod>('allTime');
  const [apiKeyTokensTimePeriod, setApiKeyTokensTimePeriod] = useState<TimePeriod>('allTime');
  const [userTokensTimePeriod, setUserTokensTimePeriod] = useState<TimePeriod>('day');

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
                <Link to='/dashboard/channel-success-rates' className='text-primary text-sm hover:underline'>
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

      {/* 使用详情分析 - 导航卡片 */}
      <Link
        to='/analytics'
        className='bg-card hover:bg-accent/50 flex w-full items-center justify-between rounded-lg border p-4 text-left transition-colors'
      >
        <div className='flex items-center gap-3'>
          <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-md'>
            <TrendingUp className='text-primary h-4 w-4' />
          </div>
          <div>
            <span className='text-lg font-semibold'>{t('dashboard.sections.analytics')}</span>
            <p className='text-muted-foreground text-sm'>{t('dashboard.sections.analyticsDescription')}</p>
          </div>
        </div>
        <ChevronRight className='text-muted-foreground h-5 w-5' />
      </Link>

      {/* 渠道分析 - 可折叠 */}
      <CollapsibleSection
        title={t('dashboard.sections.channels')}
        icon={<BarChart3 className='text-primary h-4 w-4' />}
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
      <CollapsibleSection title={t('dashboard.sections.models')} icon={<Brain className='text-primary h-4 w-4' />} storageKey='models'>
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
      <CollapsibleSection title={t('dashboard.sections.apiKeys')} icon={<Key className='text-primary h-4 w-4' />} storageKey='apiKeys'>
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

      {/* 用户分析 - 可折叠 */}
      {isProjectOwner && (
        <CollapsibleSection title={t('dashboard.sections.users')} icon={<Users className='text-primary h-4 w-4' />} storageKey='users'>
          <div className='grid gap-4 md:grid-cols-1 lg:grid-cols-7'>
            <Card className='hover-card col-span-1 lg:col-span-4'>
              <CardHeader>
                <CardTitle>{t('dashboard.charts.tokensByUser')}</CardTitle>
                <CardDescription>{t('dashboard.charts.tokensByUserDescription')}</CardDescription>
                <CardAction>
                  <TimePeriodSelector value={userTokensTimePeriod} onChange={setUserTokensTimePeriod} />
                </CardAction>
              </CardHeader>
              <CardContent>
                <TokensByUserChart timePeriod={userTokensTimePeriod} />
              </CardContent>
            </Card>
          </div>
        </CollapsibleSection>
      )}

      {/* 性能分析 - 可折叠 */}
      <CollapsibleSection
        title={t('dashboard.sections.performance')}
        icon={<Zap className='text-primary h-4 w-4' />}
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
