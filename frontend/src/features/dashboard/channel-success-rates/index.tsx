import { useState, useMemo, useEffect } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useChannelSuccessRates, useTokensByChannel, type TokensByChannel } from '../data/dashboard';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Checkbox } from '@/components/ui/checkbox';
import { Skeleton } from '@/components/ui/skeleton';
import { ActivityIcon, AlertTriangleIcon, CheckCircle2Icon, CoinsIcon, XCircleIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Header } from '@/components/layout/header';
import { Card, CardContent } from '@/components/ui/card';
import ContentSection from '@/features/settings/components/content-section';

type SortField = 'totalCount' | 'successCount' | 'failedCount' | 'successRate' | 'inputTokens' | 'outputTokens' | 'totalTokens';
type SortOrder = 'asc' | 'desc';

const PAGE_SIZE = 20;

export default function DashboardChannelSuccessRates() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [timeWindow, setTimeWindow] = useState<string>('day');
  const [sortField, setSortField] = useState<SortField>('successRate');
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc');
  const [currentPage, setCurrentPage] = useState(1);
  const [filterType, setFilterType] = useState<string>('all');
  const [showWarningsOnly, setShowWarningsOnly] = useState(false);

  // Fetch all data (limit = undefined)
  const { data: channels, isLoading, error } = useChannelSuccessRates(undefined, timeWindow);

  // Fetch token stats by channel (reuse existing API)
  const { data: tokenData } = useTokensByChannel(timeWindow);

  // Build token map by channelId for reliable matching
  const tokenByChannel = useMemo(() => {
    if (!tokenData) return new Map<string, TokensByChannel>();
    const map = new Map<string, TokensByChannel>();
    tokenData.forEach((t) => map.set(t.channelId, t));
    return map;
  }, [tokenData]);

  // Extract unique channel types
  const channelTypes = useMemo(() => {
    if (!channels) return [];
    const types = new Set<string>();
    channels.forEach((c) => {
      if (c.channelType) types.add(c.channelType);
    });
    return Array.from(types).sort();
  }, [channels]);

  // Filter channels
  const filteredChannels = useMemo(() => {
    if (!channels) return [];
    let result = [...channels];

    // Filter by channel type
    if (filterType !== 'all') {
      result = result.filter((c) => c.channelType === filterType);
    }

    // Show warnings only (disabled channels)
    if (showWarningsOnly) {
      result = result.filter((c) => c.channelDisabled);
    }

    return result;
  }, [channels, filterType, showWarningsOnly]);

  // Sort channels
  const sortedChannels = useMemo(() => {
    return [...filteredChannels].sort((a, b) => {
      let aVal: number, bVal: number;
      switch (sortField) {
        case 'successCount':
          aVal = a.successCount;
          bVal = b.successCount;
          break;
        case 'failedCount':
          aVal = a.failedCount;
          bVal = b.failedCount;
          break;
        case 'successRate':
          aVal = a.successRate;
          bVal = b.successRate;
          break;
        case 'inputTokens':
          aVal = tokenByChannel.get(a.channelId)?.inputTokens ?? 0;
          bVal = tokenByChannel.get(b.channelId)?.inputTokens ?? 0;
          break;
        case 'outputTokens':
          aVal = tokenByChannel.get(a.channelId)?.outputTokens ?? 0;
          bVal = tokenByChannel.get(b.channelId)?.outputTokens ?? 0;
          break;
        case 'totalTokens':
          aVal = tokenByChannel.get(a.channelId)?.totalTokens ?? 0;
          bVal = tokenByChannel.get(b.channelId)?.totalTokens ?? 0;
          break;
        default:
          aVal = a.totalCount;
          bVal = b.totalCount;
      }
      return sortOrder === 'asc' ? aVal - bVal : bVal - aVal;
    });
  }, [filteredChannels, sortField, sortOrder, tokenByChannel]);

  // Paginate channels
  const totalPages = Math.ceil(sortedChannels.length / PAGE_SIZE);
  const paginatedChannels = sortedChannels.slice(
    (currentPage - 1) * PAGE_SIZE,
    currentPage * PAGE_SIZE
  );

  // Reset to page 1 when filters change
  useEffect(() => {
    setCurrentPage(1);
  }, [timeWindow, filterType, showWarningsOnly, sortField, sortOrder]);

  const handleBack = () => {
    navigate({ to: '/' });
  };

  const scrollToTop = () => {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  function getSuccessRateColor(rate: number): string {
    if (rate >= 95) return 'text-green-600';
    if (rate >= 50) return 'text-yellow-600';
    return 'text-red-600';
  }

  function getProgressBarColor(rate: number): string {
    if (rate >= 95) return 'bg-green-600';
    if (rate >= 50) return 'bg-yellow-600';
    return 'bg-red-600';
  }

  function formatNumber(num: number): string {
    return num.toLocaleString();
  }

  if (error) {
    return (
      <ContentSection title={t('dashboard.channelSuccessRates.pageTitle')} desc="">
        <div className="text-red-500">加载失败: {error.message}</div>
      </ContentSection>
    );
  }

  const start = (currentPage - 1) * PAGE_SIZE + 1;
  const end = Math.min(currentPage * PAGE_SIZE, sortedChannels.length);
  const total = sortedChannels.length;

  return (
    <div className="flex flex-1 flex-col gap-4 p-8 pt-6">
      <Header title={t('dashboard.channelSuccessRates.pageTitle')} description="查看所有渠道的请求成功率统计" />
      <div className="space-y-4">
        {/* Toolbar */}
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <Button onClick={handleBack} variant="outline" className="self-start">
            {t('dashboard.channelSuccessRates.backToDashboard')}
          </Button>

          <div className="flex flex-wrap items-center gap-2">
            {/* Time window */}
            <Select value={timeWindow} onValueChange={setTimeWindow}>
              <SelectTrigger className="w-[120px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="day">{t('dashboard.stats.today')}</SelectItem>
                <SelectItem value="week">{t('dashboard.stats.thisWeek')}</SelectItem>
                <SelectItem value="month">{t('dashboard.stats.thisMonth')}</SelectItem>
              </SelectContent>
            </Select>

            {/* Channel type filter */}
            <Select value={filterType} onValueChange={setFilterType}>
              <SelectTrigger className="w-[150px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t('dashboard.channelSuccessRates.allTypes')}</SelectItem>
                {channelTypes.map((type) => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            {/* Show warnings only */}
            <label className="flex items-center gap-2 whitespace-nowrap text-sm">
              <Checkbox checked={showWarningsOnly} onCheckedChange={(checked) => setShowWarningsOnly(checked === true)} />
              {t('dashboard.channelSuccessRates.showWarnings')}
            </label>

            {/* Sort field */}
            <Select value={sortField} onValueChange={(value) => setSortField(value as SortField)}>
              <SelectTrigger className="w-[150px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="totalCount">{t('dashboard.channelSuccessRates.sortByTotal')}</SelectItem>
                <SelectItem value="successCount">{t('dashboard.channelSuccessRates.sortBySuccess')}</SelectItem>
                <SelectItem value="failedCount">{t('dashboard.channelSuccessRates.sortByFailed')}</SelectItem>
                <SelectItem value="successRate">{t('dashboard.channelSuccessRates.sortByRate')}</SelectItem>
                <SelectItem value="inputTokens">{t('dashboard.channelSuccessRates.sortByInputTokens')}</SelectItem>
                <SelectItem value="outputTokens">{t('dashboard.channelSuccessRates.sortByOutputTokens')}</SelectItem>
                <SelectItem value="totalTokens">{t('dashboard.channelSuccessRates.sortByTotalTokens')}</SelectItem>
              </SelectContent>
            </Select>

            {/* Sort order */}
            <Select value={sortOrder} onValueChange={(value) => setSortOrder(value as SortOrder)}>
              <SelectTrigger className="w-[100px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="desc">{t('dashboard.channelSuccessRates.desc')}</SelectItem>
                <SelectItem value="asc">{t('dashboard.channelSuccessRates.asc')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        {/* Loading skeleton */}
        {isLoading && (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <Card key={i}>
                <CardContent className="space-y-3">
                  <div className="flex items-center gap-3">
                    <Skeleton className="h-5 w-5 rounded-md" />
                    <div className="flex-1 space-y-1">
                      <Skeleton className="h-4 w-[140px]" />
                      <Skeleton className="h-3 w-[80px]" />
                    </div>
                  </div>
                  <div className="space-y-2">
                    <div className="flex items-baseline justify-between">
                      <Skeleton className="h-8 w-[80px]" />
                      <Skeleton className="h-3 w-[72px]" />
                    </div>
                    <Skeleton className="h-2 w-full rounded-full" />
                  </div>
                  <div className="flex gap-3">
                    <Skeleton className="h-4 w-[64px]" />
                    <Skeleton className="h-4 w-[64px]" />
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}

        {/* Empty state */}
        {!isLoading && paginatedChannels.length === 0 && (
          <div className="py-12 text-center text-muted-foreground">暂无数据</div>
        )}

        {/* Channel cards grid */}
        {!isLoading && paginatedChannels.length > 0 && (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
            {paginatedChannels.map((channel) => {
              const tokens = tokenByChannel.get(channel.channelId);
              const showTokens = tokens && tokens.totalTokens > 0;

              return (
                <Card key={channel.channelId} className="hover-card min-w-0">
                  <CardContent className="space-y-3">
                    {/* Channel info */}
                    <div className="flex items-center gap-3">
                      <ActivityIcon className="h-5 w-5 shrink-0 text-muted-foreground" />
                      <div className="min-w-0 flex-1">
                        <p className="truncate font-medium">{channel.channelName}</p>
                        <span className="text-xs text-muted-foreground">{channel.channelType}</span>
                      </div>
                      {channel.channelDisabled && <AlertTriangleIcon className="h-5 w-5 shrink-0 text-red-500" />}
                    </div>

                    {/* Success rate display */}
                    <div>
                      <div className="mb-1 flex items-baseline justify-between">
                        <span className={`text-2xl font-bold ${getSuccessRateColor(channel.successRate)}`}>
                          {channel.successRate.toFixed(1)}%
                        </span>
                        <span className="text-xs text-muted-foreground">{formatNumber(channel.totalCount)} total</span>
                      </div>

                      {/* Progress bar */}
                      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                        <div className={`h-full ${getProgressBarColor(channel.successRate)}`} style={{ width: `${channel.successRate}%` }} />
                      </div>
                    </div>

                    {/* Success/Failed counts */}
                    <div className="flex gap-3 text-sm">
                      <span className="flex items-center gap-1">
                        <CheckCircle2Icon className="h-4 w-4 text-green-500" />
                        {formatNumber(channel.successCount)}
                      </span>
                      <span className="flex items-center gap-1">
                        <XCircleIcon className="h-4 w-4 text-red-500" />
                        {formatNumber(channel.failedCount)}
                      </span>
                    </div>

                    {/* Token consumption (from tokenStatsByChannel API) */}
                    {showTokens && (
                      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
                        <CoinsIcon className="h-3 w-3 shrink-0" />
                        <span>{t('dashboard.channelSuccessRates.inputTokens')}: {formatNumber(tokens.inputTokens)}</span>
                        <span className="text-border">|</span>
                        <span>{t('dashboard.channelSuccessRates.outputTokens')}: {formatNumber(tokens.outputTokens)}</span>
                        <span className="text-border">|</span>
                        <span>{t('dashboard.channelSuccessRates.totalTokens')}: {formatNumber(tokens.totalTokens)}</span>
                      </div>
                    )}
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}

        {/* Pagination */}
        {!isLoading && totalPages > 1 && (
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="text-sm text-muted-foreground">
              {t('dashboard.channelSuccessRates.showing', { start, end, total })}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button onClick={() => setCurrentPage((p) => p - 1)} disabled={currentPage === 1} variant="outline" size="sm">
                {t('dashboard.channelSuccessRates.prev')}
              </Button>
              <span className="text-sm">
                {currentPage} / {totalPages}
              </span>
              <Button onClick={() => setCurrentPage((p) => p + 1)} disabled={currentPage === totalPages} variant="outline" size="sm">
                {t('dashboard.channelSuccessRates.next')}
              </Button>
              <Button onClick={scrollToTop} variant="outline" size="sm">
                回到顶部
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
