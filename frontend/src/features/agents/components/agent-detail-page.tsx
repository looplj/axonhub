import { useMemo, useState } from 'react';
import { formatDistanceToNow, format } from 'date-fns';
import { zhCN, enUS } from 'date-fns/locale';
import {
  ArrowLeft,
  MessageSquare,
  Activity,
  Server,
  Clock,
  MessageSquareText,
  Rocket,
  Edit,
  Trash2,
  MoreHorizontal,
  Copy,
  Check,
  Terminal,
  Settings,
  Info,
  Key,
  Cpu,
  Calendar,
  User,
  LayoutGrid,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from '@tanstack/react-router';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Separator } from '@/components/ui/separator';
import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { ScrollArea } from '@/components/ui/scroll-area';
import { extractNumberID } from '@/lib/utils';
import { useAgentDetail } from '../data/agent-detail';
import { DeployAxonclawDialog } from './deploy-axonclaw-dialog';

function isInstanceOnline(lastHeartbeatAt: string | Date, thresholdMs: number) {
  const t = new Date(lastHeartbeatAt).getTime();
  if (Number.isNaN(t)) return false;
  return Date.now() - t <= thresholdMs;
}

function StatusBadge({ status }: { status: string }) {
  const variant =
    status === 'enabled' ? 'default' : status === 'disabled' ? 'secondary' : 'outline';
  return (
    <Badge variant={variant} className='h-5 px-2 text-[10px] uppercase'>
      {status}
    </Badge>
  );
}

function CopyableField({ value, label }: { value: string; label?: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className='flex items-center gap-2'>
      {label && <span className='text-muted-foreground text-xs'>{label}</span>}
      <code className='bg-muted max-w-[200px] truncate rounded px-2 py-1 font-mono text-xs'>
        {value}
      </code>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant='ghost' size='icon' className='h-6 w-6' onClick={handleCopy}>
            {copied ? <Check className='h-3 w-3 text-green-500' /> : <Copy className='h-3 w-3' />}
          </Button>
        </TooltipTrigger>
        <TooltipContent>
          <p>{copied ? 'Copied!' : 'Copy'}</p>
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

export function AgentDetailPage() {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const navigate = useNavigate();
  const { agentId } = useParams({ from: '/_authenticated/project/agents/$agentId/' as any }) as {
    agentId: string;
  };
  const { getSearchParams } = usePaginationSearch({ defaultPageSize: 20 });

  const { data: agent, isLoading, refetch } = useAgentDetail(agentId);
  const [onlineThresholdSeconds, setOnlineThresholdSeconds] = useState(30);
  const [deployDialogOpen, setDeployDialogOpen] = useState(false);
  const { agentRuntimesPermissions } = usePermissions();

  const instances = useMemo(
    () => agent?.instances?.edges?.map((e) => e.node) ?? [],
    [agent?.instances?.edges]
  );
  const onlineCount = useMemo(() => {
    const thresholdMs = onlineThresholdSeconds * 1000;
    return instances.filter((inst) => isInstanceOnline(inst.lastHeartbeatAt, thresholdMs)).length;
  }, [instances, onlineThresholdSeconds]);

  const handleBack = () => {
    navigate({ to: '/project/agents' as any, search: getSearchParams() as any });
  };

  const handleEdit = () => {
    navigate({ to: '/project/agents/$agentId/edit' as any, params: { agentId } as any });
  };

  if (isLoading) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b' />
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
        <Header className='border-b' />
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-6 text-center'>
              <div className='space-y-2'>
                <Activity className='text-muted-foreground mx-auto h-16 w-16' />
                <p className='text-muted-foreground text-xl font-medium'>
                  {t('threads.detail.notFound')}
                </p>
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
  const updatedAtLabel = format(agent.updatedAt, 'yyyy-MM-dd HH:mm:ss', { locale });

  const skillsPolicyText =
    agent.skillsPolicy?.add === 'open'
      ? t('agents.skillsPolicy.open')
      : agent.skillsPolicy?.add === 'approval_required'
        ? t('agents.skillsPolicy.approvalRequired')
        : agent.skillsPolicy?.add === 'registry_only'
          ? t('agents.skillsPolicy.registryOnly')
          : '-';

  return (
    <div className='flex h-screen flex-col'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 border-b backdrop-blur h-auto min-h-16 py-3'>
        <div className='flex w-full items-center justify-between'>
          <div className='flex items-center gap-4'>
            <Button variant='ghost' size='sm' onClick={handleBack} className='hover:bg-accent'>
              <ArrowLeft className='mr-2 h-4 w-4' />
              {t('common.back')}
            </Button>
            <Separator orientation='vertical' className='h-6' />
            <div className='flex items-center gap-3'>
              <div className='bg-primary/10 flex h-10 w-10 items-center justify-center rounded-xl'>
                <MessageSquare className='text-primary h-5 w-5' />
              </div>
              <div>
                <div className='flex items-center gap-2'>
                  <h1 className='text-lg font-semibold'>{agent.name}</h1>
                  <span className='text-muted-foreground text-sm'>
                    #{extractNumberID(agent.id) || agent.id}
                  </span>
                  <StatusBadge status={agent.status} />
                </div>
                <div className='mt-0.5 flex items-center gap-2 text-xs text-muted-foreground'>
                  <Calendar className='h-3 w-3' />
                  <span>{createdAtLabel}</span>
                </div>
              </div>
            </div>
          </div>

          <div className='flex items-center gap-2'>
            {agentRuntimesPermissions.canWrite && (
              <Button
                variant='default'
                size='sm'
                onClick={() => setDeployDialogOpen(true)}
                className='gap-2'
              >
                <Rocket className='h-4 w-4' />
                {t('agents.actions.deploy')}
              </Button>
            )}
            <Button variant='outline' size='sm' onClick={() => refetch()} className='gap-2'>
              <Activity className='h-4 w-4' />
              {t('common.refresh')}
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant='ghost' size='icon' className='h-9 w-9'>
                  <MoreHorizontal className='h-4 w-4' />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align='end'>
                <DropdownMenuItem onClick={handleEdit}>
                  <Edit className='mr-2 h-4 w-4' />
                  {t('common.buttons.edit')}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem className='text-destructive'>
                  <Trash2 className='mr-2 h-4 w-4' />
                  {t('common.buttons.delete')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-hidden bg-muted/30'>
        <ScrollArea className='h-full'>
          <div className='container mx-auto max-w-7xl p-6'>
            <div className='grid grid-cols-1 gap-6 lg:grid-cols-3'>
              {/* Left Column - Agent Info (2/3) */}
              <div className='flex flex-col gap-6 lg:col-span-2'>
                {/* Basic Info Card */}
                <Card className='border-0 shadow-sm'>
                  <CardHeader className='pb-3'>
                    <CardTitle className='flex items-center gap-2 text-base'>
                      <Info className='text-primary h-4 w-4' />
                      {t('agents.form.basicInfo')}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className='space-y-4'>
                    {agent.description && (
                      <div>
                        <p className='text-muted-foreground text-sm'>{agent.description}</p>
                      </div>
                    )}
                    <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                      <div className='space-y-1'>
                        <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
                          <Cpu className='h-3 w-3' />
                          {t('agents.fields.model')}
                        </div>
                        <div className='text-sm font-medium'>{agent.model || '-'}</div>
                      </div>
                      <div className='space-y-1'>
                        <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
                          <Settings className='h-3 w-3' />
                          {t('agents.fields.skillsPolicy')}
                        </div>
                        <div className='text-sm font-medium'>{skillsPolicyText}</div>
                      </div>
                      <div className='space-y-1'>
                        <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
                          <User className='h-3 w-3' />
                          {t('common.columns.createdBy')}
                        </div>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <div className='text-sm font-medium'>
                              {(() => {
                                const id = agent.createdByUserID;
                                const shortId = id.length > 8 ? id.slice(0, 8) + '...' : id;
                                return <span>{shortId}</span>;
                              })()}
                            </div>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p>{agent.createdByUserID}</p>
                          </TooltipContent>
                        </Tooltip>
                      </div>
                      <div className='space-y-1'>
                        <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
                          <Clock className='h-3 w-3' />
                          {t('common.columns.updatedAt')}
                        </div>
                        <div className='text-sm font-medium'>{updatedAtLabel}</div>
                      </div>
                    </div>
                  </CardContent>
                </Card>

                {/* System Prompt Card */}
                {agent.prompt?.content && (
                  <Card className='border-0 shadow-sm'>
                    <CardHeader className='pb-3'>
                      <CardTitle className='flex items-center gap-2 text-base'>
                        <Terminal className='text-primary h-4 w-4' />
                        {t('agents.fields.systemPrompt')}
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className='bg-muted max-h-[300px] overflow-y-auto rounded-lg p-4'>
                        <pre className='whitespace-pre-wrap font-mono text-sm leading-relaxed'>
                          {agent.prompt.content}
                        </pre>
                      </div>
                    </CardContent>
                  </Card>
                )}

                {/* API Key Card */}
                {agent.apiKey?.key && (
                  <Card className='border-0 shadow-sm'>
                    <CardHeader className='pb-3'>
                      <CardTitle className='flex items-center gap-2 text-base'>
                        <Key className='text-primary h-4 w-4' />
                        {t('agents.fields.apiKey')}
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className='bg-muted flex items-center justify-between gap-2 rounded-lg p-3'>
                        <code className='truncate font-mono text-sm'>
                          {'sk-...' + agent.apiKey.key.slice(-4)}
                        </code>
                        <CopyableField value={agent.apiKey.key} />
                      </div>
                    </CardContent>
                  </Card>
                )}

                {/* Built-in Tools Card */}
                {agent.agentBuiltinTools && agent.agentBuiltinTools.length > 0 && (
                  <Card className='border-0 shadow-sm'>
                    <CardHeader className='pb-3'>
                      <CardTitle className='flex items-center gap-2 text-base'>
                        <LayoutGrid className='text-primary h-4 w-4' />
                        {t('agents.fields.builtinTools')}
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className='flex flex-wrap gap-2'>
                        {agent.agentBuiltinTools.map((tool: any) => (
                          <Tooltip key={tool.name}>
                            <TooltipTrigger asChild>
                              <Badge
                                variant={tool.enabled ? 'default' : 'secondary'}
                                className='cursor-default gap-1.5 px-2.5 py-1'
                              >
                                <span
                                  className={`h-1.5 w-1.5 rounded-full ${tool.enabled ? 'bg-green-400' : 'bg-gray-400'}`}
                                />
                                {tool.name}
                              </Badge>
                            </TooltipTrigger>
                            <TooltipContent>
                              <p>
                                {tool.enabled ? t('common.status.enabled') : t('common.status.disabled')} •{' '}
                                {t('agents.toolDescriptions.' + tool.name) || tool.name}
                              </p>
                            </TooltipContent>
                          </Tooltip>
                        ))}
                      </div>
                    </CardContent>
                  </Card>
                )}
              </div>

              {/* Right Column - Instances (1/3) */}
              <div className='lg:col-span-1'>
                <Card className='border-0 shadow-sm'>
                  <CardHeader className='pb-3'>
                    <div className='flex items-center justify-between'>
                      <CardTitle className='flex items-center gap-2 text-base'>
                        <Server className='text-primary h-4 w-4' />
                        Instances
                      </CardTitle>
                      <Badge variant={onlineCount > 0 ? 'default' : 'secondary'} className='text-xs'>
                        {onlineCount}/{instances.length} online
                      </Badge>
                    </div>
                    <CardDescription className='flex items-center gap-2 pt-1'>
                      <span>Threshold:</span>
                      <div className='flex items-center gap-1'>
                        {[10, 30, 60].map((sec) => (
                          <Button
                            key={sec}
                            type='button'
                            size='sm'
                            variant={onlineThresholdSeconds === sec ? 'default' : 'ghost'}
                            onClick={() => setOnlineThresholdSeconds(sec)}
                            className='h-6 px-2 text-xs'
                          >
                            {sec}s
                          </Button>
                        ))}
                      </div>
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    {instances.length === 0 ? (
                      <div className='text-muted-foreground py-8 text-center text-sm'>
                        <Server className='text-muted-foreground/50 mx-auto mb-2 h-8 w-8' />
                        <p>No instances registered.</p>
                      </div>
                    ) : (
                      <div className='space-y-2'>
                        {instances.map((inst) => {
                          const online = isInstanceOnline(inst.lastHeartbeatAt, onlineThresholdSeconds * 1000);
                          return (
                            <div
                              key={inst.id}
                              className='group flex items-center justify-between gap-2 rounded-lg border border-transparent bg-muted/50 p-3 transition-all hover:border-border hover:bg-muted'
                            >
                              <div className='flex min-w-0 items-center gap-3'>
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <span
                                      className={`h-2.5 w-2.5 shrink-0 rounded-full ${online ? 'bg-green-500' : 'bg-zinc-400'}`}
                                    />
                                  </TooltipTrigger>
                                  <TooltipContent>
                                    <p>{online ? 'Online' : 'Offline'}</p>
                                  </TooltipContent>
                                </Tooltip>
                                <div className='min-w-0'>
                                  <div className='truncate text-sm font-medium'>
                                    {inst.name || inst.instanceID}
                                  </div>
                                  <div className='text-muted-foreground truncate text-xs'>
                                    {inst.platform || '-'} • {inst.version || '-'}
                                  </div>
                                </div>
                              </div>
                              <div className='flex items-center gap-2'>
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <span className='text-muted-foreground text-xs'>
                                      {inst.lastHeartbeatAt
                                        ? `${formatDistanceToNow(new Date(inst.lastHeartbeatAt), { addSuffix: true, locale })}`
                                        : '-'}
                                    </span>
                                  </TooltipTrigger>
                                  <TooltipContent>
                                    <p>
                                      {inst.lastHeartbeatAt
                                        ? format(new Date(inst.lastHeartbeatAt), 'yyyy-MM-dd HH:mm:ss')
                                        : 'No heartbeat'}
                                    </p>
                                  </TooltipContent>
                                </Tooltip>
                                <Button
                                  variant='ghost'
                                  size='icon'
                                  className='h-7 w-7 opacity-0 transition-opacity group-hover:opacity-100'
                                  onClick={() =>
                                    navigate({
                                      to: '/project/agents/$agentId/threads/$threadId' as any,
                                      params: { agentId, threadId: inst.instanceID } as any,
                                    })
                                  }
                                >
                                  <MessageSquareText className='h-4 w-4' />
                                </Button>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            </div>
          </div>
        </ScrollArea>
      </Main>

      <DeployAxonclawDialog
        agentId={agentId}
        open={deployDialogOpen}
        onOpenChange={setDeployDialogOpen}
      />
    </div>
  );
}
