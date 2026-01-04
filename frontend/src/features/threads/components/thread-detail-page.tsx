import { useMemo, useState } from 'react';
import { format } from 'date-fns';
import { useParams, useNavigate } from '@tanstack/react-router';
import { zhCN, enUS } from 'date-fns/locale';
import { ArrowLeft, Activity } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { extractNumberID } from '@/lib/utils';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import type { Trace } from '@/features/traces/data/schema';
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
  const [selectedTraceId, setSelectedTraceId] = useState<string | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);

  const { pageSize, setCursors, paginationArgs, getSearchParams } = usePaginationSearch({
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

  const handleBack = () => {
    navigate({ to: '/project/threads' as any, search: getSearchParams() as any });
  };

  const handleViewTrace = (traceId: string) => {
    setSelectedTraceId(traceId);
    setIsDrawerOpen(true);
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
  const updatedAtLabel = format(thread.updatedAt, 'yyyy-MM-dd HH:mm:ss', { locale });

  return (
    <div className='flex h-screen flex-col'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 border-b backdrop-blur'>
        <div className='flex items-center space-x-4'>
          <Button variant='ghost' size='sm' onClick={handleBack} className='hover:bg-accent'>
            <ArrowLeft className='mr-2 h-4 w-4' />
            {t('common.back')}
          </Button>
          <Separator orientation='vertical' className='h-6' />
          <div className='flex items-center space-x-3'>
            <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-lg'>
              <Activity className='text-primary h-4 w-4' />
            </div>
            <div>
              <h1 className='text-lg leading-none font-semibold'>
                {t('threads.detail.title')} #{extractNumberID(thread.id) || thread.threadID}
              </h1>
              <div className='mt-1 flex items-center gap-2'>
                <p className='text-muted-foreground text-sm'>{thread.threadID}</p>
                <span className='text-muted-foreground text-xs'>•</span>
                <p className='text-muted-foreground text-xs'>{createdAtLabel}</p>
              </div>
            </div>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-hidden'>
        <div className='flex h-full flex-col gap-4 overflow-y-auto p-6'>
          {/* <Card className='border-0 shadow-sm'>
            <CardContent className='grid gap-4 p-6 md:grid-cols-3'>
              <div>
                <p className='text-muted-foreground text-sm'>{t('threads.detail.project')}</p>
                <p className='font-medium'>{thread.project?.name || t('threads.detail.unknownProject')}</p>
              </div>
              <div>
                <p className='text-muted-foreground text-sm'>{t('threads.detail.createdAt')}</p>
                <p className='font-medium'>{createdAtLabel}</p>
              </div>
              <div>
                <p className='text-muted-foreground text-sm'>{t('threads.detail.updatedAt')}</p>
                <p className='font-medium'>{updatedAtLabel}</p>
              </div>
              <div>
                <p className='text-muted-foreground text-sm'>{t('threads.detail.traceCount')}</p>
                <p className='font-medium'>{thread.tracesSummary?.totalCount ?? 0}</p>
              </div>
            </CardContent>
          </Card> */}

          <div className='flex flex-col gap-6'>
            <div className='flex items-center justify-between'>
              <div>
                <h2 className='text-xl font-semibold'>{t('threads.detail.tracesTitle')}</h2>
                <p className='text-muted-foreground text-sm'>{t('threads.detail.tracesSubtitle')}</p>
              </div>
              <div className='flex items-center gap-2'>
                <Button variant='outline' size='sm' onClick={() => refetch()} disabled={isLoading}>
                  {t('common.refresh')}
                </Button>
              </div>
            </div>

            {/* Traces as Cards */}
            <div className='space-y-4'>
              {traces.length > 0 ? (
                traces.map((trace, index) => (
                  <div key={trace.id}>
                    <TraceCard trace={trace} onViewTrace={handleViewTrace} />
                    {index < traces.length - 1 && <Separator className='mt-4' />}
                  </div>
                ))
              ) : (
                <Card className='border-0 shadow-sm'>
                  <CardContent className='py-16'>
                    <div className='flex h-full items-center justify-center'>
                      <div className='space-y-4 text-center'>
                        <Activity className='text-muted-foreground mx-auto h-16 w-16' />
                        <p className='text-muted-foreground text-lg'>{t('threads.detail.noTraces')}</p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              )}
            </div>

            {/* Pagination */}
            {totalCount && totalCount > 0 && (
              <div className='flex items-center justify-between'>
                <div className='text-muted-foreground text-sm'>
                  {t('common.showing')} {traces.length} {t('common.of')} {totalCount} {t('threads.detail.traces')}
                </div>
                <div className='flex items-center gap-2'>
                  <Button variant='outline' size='sm' onClick={handlePreviousPage} disabled={!pageInfo?.hasPreviousPage}>
                    {t('common.previous')}
                  </Button>
                  <Button variant='outline' size='sm' onClick={handleNextPage} disabled={!pageInfo?.hasNextPage}>
                    {t('common.next')}
                  </Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </Main>

      {/* Trace Detail Drawer */}
      <TraceDrawer open={isDrawerOpen} onOpenChange={setIsDrawerOpen} traceId={selectedTraceId} />
    </div>
  );
}
