import { useMemo, useState } from 'react';
import { formatDistanceToNow, format } from 'date-fns';
import { zhCN, enUS } from 'date-fns/locale';
import { ArrowLeft, MessageSquare, Activity, Server, Clock, MessageSquareText } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from '@tanstack/react-router';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Separator } from '@/components/ui/separator';
import { Badge } from '@/components/ui/badge';
import { extractNumberID } from '@/lib/utils';
import { useAgentDetail, useAgentThreads } from '../data/agent-detail';

function isInstanceOnline(lastHeartbeatAt: string | Date, thresholdMs: number) {
  const t = new Date(lastHeartbeatAt).getTime();
  if (Number.isNaN(t)) return false;
  return Date.now() - t <= thresholdMs;
}

export function AgentDetailPage() {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const navigate = useNavigate();
  const { agentId } = useParams({ from: '/_authenticated/project/agents/$agentId/' as any }) as { agentId: string };
  const { getSearchParams } = usePaginationSearch({ defaultPageSize: 20 });

  const { data: agent, isLoading, refetch } = useAgentDetail(agentId);
  const { data: threads, isLoading: threadsLoading, refetch: refetchThreads } = useAgentThreads(agentId);
  const [onlineThresholdSeconds, setOnlineThresholdSeconds] = useState(30);

  const instances = useMemo(() => agent?.instances?.edges?.map((e) => e.node) ?? [], [agent?.instances?.edges]);
  const onlineCount = useMemo(() => {
    const thresholdMs = onlineThresholdSeconds * 1000;
    return instances.filter((inst) => isInstanceOnline(inst.lastHeartbeatAt, thresholdMs)).length;
  }, [instances, onlineThresholdSeconds]);

  const handleBack = () => {
    navigate({ to: '/project/agents' as any, search: getSearchParams() as any });
  };

  const startNewThread = () => {
    const random = typeof crypto !== 'undefined' && 'randomUUID' in crypto ? (crypto as any).randomUUID() : String(Date.now());
    const newThreadID = `agent-${extractNumberID(agentId)}-${random}`;

    navigate({
      to: '/project/agents/$agentId/threads/$threadId' as any,
      params: { agentId, threadId: newThreadID } as any,
    });
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

  if (!agent) {
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

  const createdAtLabel = format(agent.createdAt, 'yyyy-MM-dd HH:mm:ss', { locale });

  return (
    <div className='flex h-screen flex-col'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 border-b backdrop-blur'>
        <div className='flex items-center justify-between'>
          <div className='flex items-center space-x-4'>
            <Button variant='ghost' size='sm' onClick={handleBack} className='hover:bg-accent'>
              <ArrowLeft className='mr-2 h-4 w-4' />
              {t('common.back')}
            </Button>
            <Separator orientation='vertical' className='h-6' />
            <div className='flex items-center space-x-3'>
              <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-lg'>
                <MessageSquare className='text-primary h-4 w-4' />
              </div>
              <div>
                <h1 className='text-lg leading-none font-semibold'>
                  {agent.name} <span className='text-muted-foreground font-normal'>#{extractNumberID(agent.id) || agent.id}</span>
                </h1>
                <div className='mt-1 flex items-center gap-2'>
                  <p className='text-xs text-muted-foreground'>{createdAtLabel}</p>
                  <span className='text-xs text-muted-foreground'>•</span>
                  <Badge variant={agent.status === 'enabled' ? 'default' : 'secondary'} className='h-5 px-1.5 text-[10px] uppercase'>
                    {agent.status}
                  </Badge>
                </div>
              </div>
            </div>
          </div>

          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => {
                refetch();
                refetchThreads();
              }}
            >
              {t('common.refresh')}
            </Button>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-hidden'>
        <div className='h-full overflow-y-auto p-6'>
          <div className='container mx-auto flex max-w-7xl flex-col gap-6'>
            <Card className='border-0 shadow-sm'>
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between'>
                  <CardTitle className='flex items-center gap-2 text-base'>
                    <Server className='h-4 w-4' />
                    Instances
                  </CardTitle>
                  <div className='flex items-center gap-2'>
                    <div className='flex items-center gap-1 text-xs text-muted-foreground'>
                      <Clock className='h-3 w-3' />
                      <span>Threshold:</span>
                    </div>
                    <div className='flex items-center gap-1'>
                      {[10, 30, 60].map((sec) => (
                        <Button
                          key={sec}
                          type='button'
                          size='sm'
                          variant={onlineThresholdSeconds === sec ? 'default' : 'ghost'}
                          onClick={() => setOnlineThresholdSeconds(sec)}
                          className='h-7 px-2 text-xs'
                        >
                          {sec}s
                        </Button>
                      ))}
                    </div>
                  </div>
                </div>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='flex items-center gap-2'>
                  <Badge variant={onlineCount > 0 ? 'default' : 'secondary'}>
                    {onlineCount}/{instances.length} online
                  </Badge>
                </div>

                {instances.length === 0 ? (
                  <div className='text-sm text-muted-foreground'>No instances registered.</div>
                ) : (
                  <div className='space-y-2'>
                    {instances.map((inst) => {
                      const online = isInstanceOnline(inst.lastHeartbeatAt, onlineThresholdSeconds * 1000);
                      return (
                        <div
                          key={inst.id}
                          className='flex flex-wrap items-center justify-between gap-2 rounded-md border border-transparent bg-muted/30 p-3 transition-colors hover:bg-muted/50'
                        >
                          <div className='flex min-w-0 items-center gap-3'>
                            <span className={`h-2.5 w-2.5 rounded-full ${online ? 'bg-green-500' : 'bg-zinc-400'}`} />
                            <div className='min-w-0'>
                              <div className='truncate text-sm font-medium'>{inst.name || inst.instanceID}</div>
                              <div className='truncate text-xs text-muted-foreground'>
                                {inst.platform || '-'} • {inst.version || '-'} • {inst.instanceID}
                              </div>
                            </div>
                          </div>
                          <div className='text-xs text-muted-foreground'>
                            {inst.lastHeartbeatAt
                              ? `${formatDistanceToNow(new Date(inst.lastHeartbeatAt), { addSuffix: true, locale })}`
                              : '-'}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card className='border-0 shadow-sm'>
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between gap-2'>
                  <CardTitle className='flex items-center gap-2 text-base'>
                    <MessageSquareText className='h-4 w-4' />
                    Threads
                  </CardTitle>
                  <Button size='sm' onClick={startNewThread}>
                    New
                  </Button>
                </div>
              </CardHeader>
              <CardContent className='space-y-2'>
                {threadsLoading ? (
                  <div className='text-sm text-muted-foreground'>{t('common.loading')}</div>
                ) : threads.length === 0 ? (
                  <div className='text-sm text-muted-foreground'>No threads yet. Create one to start chatting.</div>
                ) : (
                  <div className='space-y-2'>
                    {threads.map((th) => (
                      <button
                        key={th.threadID}
                        type='button'
                        className='flex w-full items-center justify-between gap-2 rounded-md border border-transparent bg-muted/30 p-3 text-left transition-colors hover:bg-muted/50'
                        onClick={() =>
                          navigate({
                            to: '/project/agents/$agentId/threads/$threadId' as any,
                            params: { agentId: agent.id, threadId: th.threadID } as any,
                          })
                        }
                      >
                        <div className='min-w-0'>
                          <div className='truncate text-sm font-medium'>{th.threadID}</div>
                          <div className='text-xs text-muted-foreground'>
                            {format(new Date(th.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}
                          </div>
                        </div>
                        <div className='text-xs text-muted-foreground'>Open</div>
                      </button>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      </Main>
    </div>
  );
}
