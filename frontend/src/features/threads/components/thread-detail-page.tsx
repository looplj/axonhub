import { useMemo, useState } from 'react';
import { format } from 'date-fns';
import { useParams, useNavigate } from '@tanstack/react-router';
import { zhCN, enUS } from 'date-fns/locale';
import { ArrowLeft, Activity, RefreshCw, FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { extractNumberID } from '@/lib/utils';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { ServerSidePagination } from '@/components/server-side-pagination';
import type { Trace } from '@/features/traces/data/schema';
import { useGeneralSettings } from '@/features/system/data/system';
import { useThreadDetail } from '../data/threads';
import { TraceCard } from './trace-card';
import { TraceDrawer } from './trace-drawer';

const THREAD_CURSOR_OPTIONS = {
  startCursorKey: 'threadTracesStart',
  endCursorKey: 'threadTracesEnd',
  pageSizeKey: 'threadTracesPageSize',
  directionKey: 'threadTracesDirection',
  cursorHistoryKey: 'threadTracesHistory',
} as const;

export default function ThreadDetailPage() {
  const { threadId } = useParams({ from: '/_authenticated/project/threads/$threadId' as any }) as {
    threadId: string;
  };
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  
  // 合并 Drawer 相关状态
  const [drawerState, setDrawerState] = useState<{
    open: boolean;
    traceId: string | null;
  }>({ open: false, traceId: null });

  const { data: settings } = useGeneralSettings();

  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs, getSearchParams } = usePaginationSearch({
    defaultPageSize: 20,
    ...THREAD_CURSOR_OPTIONS,
  });

  const tracesFirst = paginationArgs.first ?? pageSize;
  const tracesAfter = paginationArgs.after;

  const {
    data: thread,
    isLoading,
    refetch,
  } = useThreadDetail({
    id: threadId,
    tracesFirst,
    tracesAfter,
    traceOrderBy: { field: 'CREATED_AT', direction: 'DESC' },
  });

  const traces: Trace[] = useMemo(() => {
    if (!thread?.tracesConnection) return [];
    return thread.tracesConnection.edges.map(({ node }) => node);
  }, [thread?.tracesConnection]);

  const pageInfo = thread?.tracesConnection?.pageInfo;
  const totalCount = thread?.tracesConnection?.totalCount;

  const handleNextPage = () => {
    if (pageInfo?.hasNextPage && pageInfo.endCursor) {
      setCursors(pageInfo.startCursor ?? undefined, pageInfo.endCursor ?? undefined, 'after');
    }
  };

  const handlePreviousPage = () => {
    if (pageInfo?.hasPreviousPage) {
      setCursors(pageInfo.startCursor ?? undefined, pageInfo.endCursor ?? undefined, 'before');
    }
  };

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize);
    resetCursor();
  };

  const handleBack = () => {
    navigate({ to: '/project/threads' as any, search: getSearchParams() as any });
  };

  const handleViewTrace = (traceId: string) => {
    setDrawerState({ open: true, traceId });
  };

  if (isLoading) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b'></Header>
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-4 text-center'>
              <div className='border-primary mx-auto h-12 w-12 animate-spin rounded-full border-b-2'></div>
              <p className='text-muted-foreground text-lg'>{t('common.loading')}</p>
            </div>
          </div>
        </Main>
      </div>
    );
  }

  if (!thread) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b'></Header>
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-6 text-center'>
              <div className='space-y-2'>
                <Activity className='text-muted-foreground mx-auto h-16 w-16' />
                <p className='text-muted-foreground text-xl font-medium'>{t('threads.detail.notFound')}</p>
              </div>
              <Button onClick={handleBack} size='lg'>
                <ArrowLeft className='mr-2 h-4 w-4' />
                {t('common.back')}
              </Button>
            </div>
          </div>
        </Main>
      </div>
    );
  }

  const createdAtLabel = format(thread.createdAt, 'yyyy-MM-dd HH:mm:ss', { locale });

  return (
    <div className='flex h-screen flex-col'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 w-full border-b backdrop-blur'>
        <div className='flex w-full items-center justify-between gap-2'>
          <div className='flex items-center gap-2 sm:gap-4 min-w-0 flex-1'>
            <Button variant='ghost' size='sm' onClick={handleBack} className='hover:bg-accent shrink-0'>
              <ArrowLeft className='mr-1 sm:mr-2 h-4 w-4' />
              <span className='hidden sm:inline'>{t('common.back')}</span>
            </Button>
            <Separator orientation='vertical' className='h-6 shrink-0 hidden sm:block' />
            <div className='flex items-center gap-2 sm:gap-3 min-w-0'>
              <div className='bg-primary/10 flex h-7 w-7 sm:h-8 sm:w-8 items-center justify-center rounded-lg shrink-0'>
                <Activity className='text-primary h-3.5 w-3.5 sm:h-4 sm:w-4' />
              </div>
              <div className='min-w-0'>
                <h1 className='text-sm sm:text-lg leading-none font-semibold truncate'>
                  {t('threads.detail.title')} #{extractNumberID(thread.id) || thread.threadID}
                </h1>
                <div className='mt-1 flex items-center gap-1 sm:gap-2 text-xs sm:text-sm'>
                  <p className='text-muted-foreground truncate max-w-[120px] sm:max-w-none'>{thread.threadID}</p>
                  <span className='text-muted-foreground hidden sm:inline'>•</span>
                  <p className='text-muted-foreground text-[10px] sm:text-xs hidden sm:inline'>{createdAtLabel}</p>
                </div>
              </div>
            </div>
          </div>
          <div className='flex items-center gap-1 sm:gap-2 shrink-0'>
            <Button variant='outline' size='sm' onClick={() => refetch()} disabled={isLoading} className='px-2 sm:px-3'>
              <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
              <span className='hidden sm:inline ml-2'>{t('common.refresh')}</span>
            </Button>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-hidden flex flex-col p-0'>
        {/* Top: Usage Metadata */}
        <div className='px-4 sm:px-6 py-3 sm:py-4 border-b bg-background'>
          <div className='grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 sm:gap-4'>
            <div className='bg-muted/30 rounded-lg px-3 py-2'>
              <p className='text-muted-foreground text-xs sm:text-sm'>{t('traces.detail.totalTokensLabel')}</p>
              <p className='text-base sm:text-lg font-semibold'>{(thread.usageMetadata?.totalTokens ?? 0).toLocaleString()}</p>
            </div>
            <div className='bg-muted/30 rounded-lg px-3 py-2'>
              <p className='text-muted-foreground text-xs sm:text-sm'>{t('traces.detail.inputTokensLabel')}</p>
              <p className='text-base sm:text-lg font-semibold'>{(thread.usageMetadata?.totalInputTokens ?? 0).toLocaleString()}</p>
            </div>
            <div className='bg-muted/30 rounded-lg px-3 py-2'>
              <p className='text-muted-foreground text-xs sm:text-sm'>{t('traces.detail.outputTokensLabel')}</p>
              <p className='text-base sm:text-lg font-semibold'>{(thread.usageMetadata?.totalOutputTokens ?? 0).toLocaleString()}</p>
            </div>
            <div className='bg-muted/30 rounded-lg px-3 py-2'>
              <p className='text-muted-foreground text-xs sm:text-sm'>{t('traces.detail.cachedTokensLabel')}</p>
              <p className='text-base sm:text-lg font-semibold'>{(thread.usageMetadata?.totalCachedTokens ?? 0).toLocaleString()}</p>
            </div>
            <div className='bg-muted/30 rounded-lg px-3 py-2'>
              <p className='text-muted-foreground text-xs sm:text-sm'>{t('traces.detail.cachedWriteTokensLabel')}</p>
              <p className='text-base sm:text-lg font-semibold'>{(thread.usageMetadata?.totalCachedWriteTokens ?? 0).toLocaleString()}</p>
            </div>
            <div className='bg-muted/30 rounded-lg px-3 py-2'>
              <p className='text-muted-foreground text-xs sm:text-sm'>{t('usageLogs.columns.totalCost')}</p>
              {thread.usageMetadata?.totalCost ? (
                <p className='text-base sm:text-lg font-semibold'>
                  {t('currencies.format', {
                    val: thread.usageMetadata.totalCost,
                    currency: settings?.currencyCode,
                    locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
                    minimumFractionDigits: 6,
                  })}
                </p>
              ) : (
                <p className='text-muted-foreground text-base sm:text-lg font-semibold'>-</p>
              )}
            </div>
          </div>
        </div>

        {/* Traces List */}
        <div className='flex-1 overflow-auto p-3 sm:p-6'>
          {traces.length > 0 ? (
            <div className='space-y-3 sm:space-y-4'>
              {traces.map((trace, index) => (
                <TraceCard key={trace.id} trace={trace} onViewTrace={handleViewTrace} index={index} />
              ))}
            </div>
          ) : (
            <div className='flex h-full items-center justify-center p-6'>
              <Card className='border-0 shadow-sm'>
                <CardContent className='py-16'>
                  <div className='flex h-full items-center justify-center'>
                    <div className='space-y-4 text-center'>
                      <FileText className='text-muted-foreground mx-auto h-16 w-16' />
                      <p className='text-muted-foreground text-lg'>{t('threads.detail.noTraces')}</p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>
          )}
        </div>

        {/* Pagination */}
        {totalCount !== undefined && totalCount > 0 && (
          <div className='border-t bg-background px-3 sm:px-6 py-3'>
            <ServerSidePagination
              pageInfo={pageInfo}
              pageSize={pageSize}
              dataLength={traces.length}
              totalCount={totalCount}
              selectedRows={0}
              onNextPage={handleNextPage}
              onPreviousPage={handlePreviousPage}
              onPageSizeChange={handlePageSizeChange}
              onResetCursor={resetCursor}
            />
          </div>
        )}
      </Main>

      {/* Trace Detail Drawer */}
      <TraceDrawer
        open={drawerState.open}
        onOpenChange={(open) => setDrawerState((prev) => ({ ...prev, open }))}
        traceId={drawerState.traceId}
      />
    </div>
  );
}
