import { useMemo, useState } from 'react';
import { formatDistanceToNow, format } from 'date-fns';
import { zhCN, enUS } from 'date-fns/locale';
import { ArrowLeft, MessageSquare, Activity } from 'lucide-react';
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
  const { agentId } = useParams({ from: '/_authenticated/project/agents/$agentId' as any }) as { agentId: string };
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
                  <p className='text-muted-foreground text-xs'>{createdAtLabel}</p>
                  <span className='text-muted-foreground text-xs'>•</span>
                  <p className='text-muted-foreground text-xs'>{agent.status}</p>
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
        <div className='flex h-full flex-col gap-4 overflow-y-auto p-6'>
          <Card className='border-0 shadow-sm'>
            <CardHeader className='pb-3'>
              <CardTitle className='text-base'>Instances</CardTitle>
            </CardHeader>
            <CardContent className='space-y-4'>
              <div className='flex flex-wrap items-center gap-2'>
                <Badge variant={onlineCount > 0 ? 'default' : 'secondary'}>
                  {onlineCount}/{instances.length} online
                </Badge>
                <div className='text-muted-foreground text-xs'>threshold</div>
                <div className='flex items-center gap-2'>
                  <Button
                    type='button'
                    size='sm'
                    variant={onlineThresholdSeconds === 10 ? 'default' : 'outline'}
                    onClick={() => setOnlineThresholdSeconds(10)}
                  >
                    10s
                  </Button>
                  <Button
                    type='button'
                    size='sm'
                    variant={onlineThresholdSeconds === 30 ? 'default' : 'outline'}
                    onClick={() => setOnlineThresholdSeconds(30)}
                  >
                    30s
                  </Button>
                  <Button
                    type='button'
                    size='sm'
                    variant={onlineThresholdSeconds === 60 ? 'default' : 'outline'}
                    onClick={() => setOnlineThresholdSeconds(60)}
                  >
                    60s
                  </Button>
                </div>
              </div>

              {instances.length === 0 ? (
                <div className='text-muted-foreground text-sm'>No instances registered.</div>
              ) : (
                <div className='space-y-2'>
                  {instances.map((inst) => {
                    const online = isInstanceOnline(inst.lastHeartbeatAt, onlineThresholdSeconds * 1000);
                    return (
                      <div key={inst.id} className='flex flex-wrap items-center justify-between gap-2 rounded-md border p-3'>
                        <div className='flex min-w-0 items-center gap-3'>
                          <span className={`h-2.5 w-2.5 rounded-full ${online ? 'bg-green-500' : 'bg-zinc-400'}`} />
                          <div className='min-w-0'>
                            <div className='truncate text-sm font-medium'>{inst.name || inst.instanceID}</div>
                            <div className='text-muted-foreground truncate text-xs'>
                              {inst.platform || '-'} • {inst.version || '-'} • {inst.instanceID}
                            </div>
                          </div>
                        </div>
                        <div className='text-muted-foreground text-xs'>
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
                <CardTitle className='text-base'>Threads</CardTitle>
                <Button size='sm' onClick={startNewThread}>
                  New
                </Button>
              </div>
            </CardHeader>
            <CardContent className='space-y-2'>
              {threadsLoading ? (
                <div className='text-muted-foreground text-sm'>{t('common.loading')}</div>
              ) : threads.length === 0 ? (
                <div className='text-muted-foreground text-sm'>No threads yet. Create one to start chatting.</div>
              ) : (
                <div className='space-y-2'>
                  {threads.map((th) => (
                    <button
                      key={th.threadID}
                      type='button'
                      className='hover:bg-accent flex w-full items-center justify-between gap-2 rounded-md border p-3 text-left transition-colors'
                      onClick={() =>
                        navigate({
                          to: '/project/agents/$agentId/threads/$threadId' as any,
                          params: { agentId: agent.id, threadId: th.threadID } as any,
                        })
                      }
                    >
                      <div className='min-w-0'>
                        <div className='truncate text-sm font-medium'>{th.threadID}</div>
                        <div className='text-muted-foreground text-xs'>{format(new Date(th.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}</div>
                      </div>
                      <div className='text-muted-foreground text-xs'>Open</div>
                    </button>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </Main>
    </div>
  );
}
