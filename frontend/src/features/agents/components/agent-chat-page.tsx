import { useEffect, useMemo, useRef, useState } from 'react';
import { format } from 'date-fns';
import { zhCN, enUS } from 'date-fns/locale';
import { ArrowLeft, Send, MessageSquare, ShieldCheck, CheckCircle, XCircle, Info, Check, X, Globe, MessagesSquare, Shield } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Card } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { cn } from '@/lib/utils';
import { useAgentDetail } from '../data/agent-detail';
import { AgentChatMessage, AgentMessageType, useAgentChatMessages, usePullAgentMessagesToUser, useSendAgentMessage, useResolveApproval, useAckAgentMessages } from '../data/agent-chat';
import { Checkbox } from '@/components/ui/checkbox';

function messageKey(m: AgentChatMessage) {
  return `${m.id}:${m.sequence}`;
}

type PermissionResource = {
  type: string;
  path?: string;
  workspace_rel?: string;
  outside_workspace?: boolean;
  url?: string;
  domain?: string;
  command?: string;
  cwd?: string;
  skill?: string;
};

function formatResource(resource: PermissionResource, idx: number): string {
  switch (resource.type) {
    case 'path':
      const p = resource.workspace_rel || resource.path || '';
      const extra = resource.outside_workspace ? ' (outside workspace)' : '';
      return `path: ${p}${extra}`;
    case 'dir':
      const d = resource.workspace_rel || resource.path || '';
      const dirExtra = resource.outside_workspace ? ' (outside workspace)' : '';
      return `dir: ${d}${dirExtra}`;
    case 'url':
      return `url: ${resource.url || ''}`;
    case 'domain':
      return `domain: ${resource.domain || ''}`;
    case 'command':
      return `command: ${resource.command || ''}`;
    case 'skill':
      return `skill: ${resource.skill || ''}`;
    default:
      return `${resource.type}: ${JSON.stringify(resource)}`;
  }
}

export function AgentChatPage() {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const navigate = useNavigate();
  const { agentId, threadId } = useParams({ from: '/_authenticated/project/agents/$agentId/threads/$threadId' as any }) as { agentId: string; threadId: string };

  const agentInstanceId = threadId;

  const { data: agent } = useAgentDetail(agentId);
  const send = useSendAgentMessage();
  const resolveApproval = useResolveApproval();
  const ackMessages = useAckAgentMessages();
  const { data: initialMessages, refetch: refetchInitial } = useAgentChatMessages(agentId, agentInstanceId);

  const [messages, setMessages] = useState<AgentChatMessage[]>([]);
  const [text, setText] = useState('');
  const [afterSequence, setAfterSequence] = useState(0);
  const [isComposing, setIsComposing] = useState(false);
  const [selectedResources, setSelectedResources] = useState<Map<string, Set<number>>>(new Map());

  const endRef = useRef<HTMLDivElement | null>(null);

  const toggleResource = (msgKey: string, idx: number) => {
    setSelectedResources((prev) => {
      const newMap = new Map(prev);
      const current = newMap.get(msgKey) || new Set();
      const newSet = new Set(current);
      if (newSet.has(idx)) {
        newSet.delete(idx);
      } else {
        newSet.add(idx);
      }
      newMap.set(msgKey, newSet);
      return newMap;
    });
  };

  const selectAllResources = (msgKey: string, total: number) => {
    setSelectedResources((prev) => {
      const newMap = new Map(prev);
      const newSet = new Set<number>();
      for (let i = 0; i < total; i++) {
        newSet.add(i);
      }
      newMap.set(msgKey, newSet);
      return newMap;
    });
  };

  useEffect(() => {
    if (initialMessages && initialMessages.length > 0) {
      setMessages(initialMessages);
      setAfterSequence(Math.max(...initialMessages.map((m) => m.sequence)));
    } else {
      setMessages([]);
      setAfterSequence(0);
    }
  }, [initialMessages]);

  const { data: pulledToUser, refetch: refetchPull } = usePullAgentMessagesToUser(agentId, agentInstanceId, afterSequence);

  useEffect(() => {
    if (!pulledToUser || pulledToUser.length === 0) return;

    setMessages((prev) => {
      const merged = [...prev, ...pulledToUser];
      const dedup = new Map<string, AgentChatMessage>();
      for (const m of merged) dedup.set(messageKey(m), m);
      const out = Array.from(dedup.values()).sort((a, b) => a.sequence - b.sequence);
      return out;
    });

    const maxSeq = Math.max(...pulledToUser.map((m) => m.sequence));
    if (maxSeq > afterSequence) setAfterSequence(maxSeq);

    const messageIDs = pulledToUser.map((m) => m.id);
    if (messageIDs.length > 0) {
      ackMessages.mutate({ agentID: agentId, agentInstanceID: agentInstanceId, messageIDs });
    }
  }, [pulledToUser, afterSequence, agentId, agentInstanceId, ackMessages]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: 'end' });
  }, [messages.length]);

  const handleBack = () => {
    navigate({ to: '/project/agents/$agentId' as any, params: { agentId } as any });
  };

  const handleSend = async () => {
    const trimmed = text.trim();
    if (!trimmed || send.isPending) return;

    const optimistic: AgentChatMessage = {
      id: `optimistic:${Date.now()}`,
      agentID: agentId,
      agentInstanceID: agentInstanceId,
      direction: 'to_runtime',
      senderType: 'user',
      senderID: null,
      type: 'chat',
      correlationID: '',
      content: {},
      text: trimmed,
      sequence: afterSequence + 1,
      status: 'pending',
      createdAt: new Date(),
    };

    setText('');
    setMessages((prev) => [...prev, optimistic]);
    setAfterSequence((s) => s + 1);

    try {
      const saved = await send.mutateAsync({ agentID: agentId, agentInstanceID: agentInstanceId, text: trimmed });
      setMessages((prev) => prev.map((m) => (m.id === optimistic.id ? saved : m)));
      setAfterSequence((s) => Math.max(s, saved.sequence));
      await refetchInitial();
    } catch {
      setMessages((prev) => prev.filter((m) => m.id !== optimistic.id));
    }
  };

  const headerTitle = useMemo(() => {
    const agentName = agent?.name || agentId;
    const instance = agent?.instances?.edges?.find((e) => e.node.id === agentInstanceId)?.node;
    const instanceName = instance?.name || agentInstanceId;
    return `${agentName} - ${instanceName}`;
  }, [agent?.name, agent?.instances, agentId, agentInstanceId]);

  const handleApprove = async (m: AgentChatMessage, granted: boolean, scope: 'once' | 'thread' | 'workspace' | 'global' = 'once', resourceIndices?: number[]) => {
    const requestID = m.correlationID || (m.content?.id as string);
    if (!requestID) return;

    try {
      await resolveApproval.mutateAsync({
        agentID: agentId,
        agentInstanceID: agentInstanceId,
        requestID,
        granted,
        scope,
        resourceIndices,
      });
      await refetchInitial();
    } catch (err) {
      console.error('Failed to resolve approval:', err);
    }
  };

  const approvalResultsMap = useMemo(() => {
    const map = new Map<string, AgentChatMessage>();
    for (const m of messages) {
      if (m.type === 'approval_result' && m.correlationID) {
        map.set(m.correlationID, m);
      }
    }
    return map;
  }, [messages]);

  const renderMessage = (m: AgentChatMessage) => {
    const ts = m.createdAt ? format(new Date(m.createdAt), 'HH:mm:ss', { locale }) : '';
    const msgType: AgentMessageType = m.type || 'chat';
    const isToUser = m.direction === 'to_user';

    if (msgType === 'approval_result') {
      return null;
    }

    if (msgType === 'system_event') {
      return (
        <div key={messageKey(m)} className="flex w-full justify-center">
          <div className="flex items-center gap-1.5 rounded-full border bg-muted/50 px-3 py-1 text-xs text-muted-foreground">
            <Info className="h-3 w-3" />
            {m.text || JSON.stringify(m.content)}
          </div>
        </div>
      );
    }

    if (msgType === 'approval_request') {
      const content = m.content as {
        tool_name?: string;
        tool_call_id?: string;
        summary?: string;
        risk_level?: string;
        reason?: string;
        capabilities?: string[];
        resources?: unknown[];
        expires_at?: string;
      };
      const isPending = m.status === 'pending';
      const hasParams = content.resources && Array.isArray(content.resources) && content.resources.length > 0;

      const approvalResult = m.correlationID ? approvalResultsMap.get(m.correlationID) : undefined;
      const resultGranted = approvalResult ? Boolean(approvalResult.content?.granted) : undefined;

      return (
        <div key={messageKey(m)} className={cn('flex w-full', isToUser ? 'justify-start' : 'justify-end')}>
          <div className="max-w-[85%] rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950 px-4 py-3 text-sm">
            <div className="flex items-center gap-2 font-medium text-amber-700 dark:text-amber-400 mb-2">
              <ShieldCheck className="h-4 w-4" />
              {t('agents.chat.approvalRequest')}
            </div>
            <div className="space-y-2 mb-3">
              {content.tool_name && (
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">{t('agents.chat.toolName')}:</span>
                  <code className="px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900 rounded text-xs font-mono">
                    {content.tool_name}
                  </code>
                </div>
              )}
              {content.tool_call_id && (
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">{t('agents.chat.toolCallId')}:</span>
                  <code className="px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900 rounded text-xs font-mono text-muted-foreground">
                    {content.tool_call_id}
                  </code>
                </div>
              )}
              {content.summary && (
                <div className="text-sm">{content.summary}</div>
              )}
              {content.reason && (
                <div className="text-xs text-muted-foreground italic border-l-2 border-amber-300 dark:border-amber-700 pl-2">
                  {content.reason}
                </div>
              )}
              {hasParams && (
                <div className="mt-2">
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-xs text-muted-foreground">{t('agents.chat.resources')}:</span>
                    {isPending && !approvalResult && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-5 px-2 text-[10px]"
                        onClick={() => selectAllResources(messageKey(m), (content.resources as PermissionResource[]).length)}
                      >
                        {t('agents.chat.selectAll')}
                      </Button>
                    )}
                  </div>
                  <div className="space-y-1">
                    {(content.resources as PermissionResource[]).map((res, idx) => {
                      const msgK = messageKey(m);
                      const isSelected = selectedResources.get(msgK)?.has(idx) ?? false;
                      return (
                        <div key={idx} className="flex items-start gap-2">
                          {isPending && !approvalResult && (
                            <Checkbox
                              checked={isSelected}
                              onCheckedChange={() => toggleResource(msgK, idx)}
                              className="mt-0.5"
                            />
                          )}
                          <code className={cn(
                            "text-xs font-mono flex-1 px-1.5 py-0.5 rounded",
                            isSelected ? "bg-green-100 dark:bg-green-900" : "bg-amber-100/50 dark:bg-amber-900/50"
                          )}>
                            {formatResource(res, idx)}
                          </code>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
              {content.capabilities && content.capabilities.length > 0 && (
                <div className="flex flex-wrap gap-1 mt-1">
                  {content.capabilities.map((cap, idx) => (
                    <span key={idx} className="px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900 rounded text-xs">
                      {cap}
                    </span>
                  ))}
                </div>
              )}
              {content.risk_level && (
                <div className="text-xs mt-1">
                  <span className="text-muted-foreground">{t('agents.chat.risk')}: </span>
                  <span className={cn(
                    "font-medium",
                    content.risk_level === 'high' ? 'text-red-600' :
                    content.risk_level === 'medium' ? 'text-yellow-600' : 'text-green-600'
                  )}>{content.risk_level}</span>
                </div>
              )}
              {content.expires_at && (
                <div className="text-xs text-muted-foreground">
                  {t('agents.chat.expiresAt')}: {format(new Date(content.expires_at), 'HH:mm:ss', { locale })}
                </div>
              )}
            </div>

            {approvalResult && resultGranted !== undefined && (
              <div className={cn(
                "mt-3 rounded-md border px-3 py-2",
                resultGranted
                  ? "border-green-200 bg-green-100/50 dark:border-green-800 dark:bg-green-900/30"
                  : "border-red-200 bg-red-100/50 dark:border-red-800 dark:bg-red-900/30"
              )}>
                <div className={cn(
                  "flex items-center gap-2 text-xs font-medium",
                  resultGranted ? "text-green-700 dark:text-green-400" : "text-red-700 dark:text-red-400"
                )}>
                  {resultGranted ? <CheckCircle className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />}
                  {resultGranted ? t('agents.chat.approved') : t('agents.chat.denied')}
                </div>
                {approvalResult.text && (
                  <div className="text-xs text-muted-foreground mt-1 whitespace-pre-wrap break-words">
                    {approvalResult.text}
                  </div>
                )}
              </div>
            )}

            {isPending && !approvalResult ? (
              (() => {
                const msgK = messageKey(m);
                const selected = selectedResources.get(msgK);
                const resourceIndices = selected ? Array.from(selected).sort((a, b) => a - b) : undefined;
                return (
                  <div className="flex items-center gap-2 mt-3">
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-xs border-green-300 hover:bg-green-100 dark:border-green-700 dark:hover:bg-green-900"
                      onClick={() => handleApprove(m, true, 'once', resourceIndices)}
                      disabled={resolveApproval.isPending}
                    >
                      <Check className="h-3 w-3 mr-1" />
                      {t('agents.chat.approveOnce')}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-xs border-purple-300 hover:bg-purple-100 dark:border-purple-700 dark:hover:bg-purple-900"
                      onClick={() => handleApprove(m, true, 'thread', resourceIndices)}
                      disabled={resolveApproval.isPending}
                    >
                      <MessagesSquare className="h-3 w-3 mr-1" />
                      {t('agents.chat.approveThread')}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-xs border-blue-300 hover:bg-blue-100 dark:border-blue-700 dark:hover:bg-blue-900"
                      onClick={() => handleApprove(m, true, 'workspace', resourceIndices)}
                      disabled={resolveApproval.isPending}
                    >
                      <Globe className="h-3 w-3 mr-1" />
                      {t('agents.chat.approveWorkspace')}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-xs border-amber-300 hover:bg-amber-100 dark:border-amber-700 dark:hover:bg-amber-900"
                      onClick={() => handleApprove(m, true, 'global', resourceIndices)}
                      disabled={resolveApproval.isPending}
                    >
                      <Shield className="h-3 w-3 mr-1" />
                      {t('agents.chat.approveGlobal')}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-xs border-red-300 hover:bg-red-100 dark:border-red-700 dark:hover:bg-red-900"
                      onClick={() => handleApprove(m, false)}
                      disabled={resolveApproval.isPending}
                    >
                      <X className="h-3 w-3 mr-1" />
                      {t('agents.chat.deny')}
                    </Button>
                  </div>
                );
              })()
            ) : !isPending && !approvalResult ? (
              <div className="text-xs text-muted-foreground italic mt-3">{t('agents.chat.approvalResolved')}</div>
            ) : null}

            <div className="mt-2 text-[11px] text-muted-foreground">seq {m.sequence} • {ts}</div>
          </div>
        </div>
      );
    }

    return (
      <div key={messageKey(m)} className={cn('flex w-full', isToUser ? 'justify-start' : 'justify-end')}>
        <div
          className={cn(
            'max-w-[80%] rounded-lg border px-3 py-2 text-sm',
            isToUser ? 'bg-muted' : 'bg-primary text-primary-foreground border-primary/20'
          )}
        >
          <div className='whitespace-pre-wrap break-words'>{m.text}</div>
          <div className={cn('mt-1 text-[11px] opacity-80', isToUser ? 'text-muted-foreground' : 'text-primary-foreground/80')}>
            seq {m.sequence} • {ts}
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className='flex h-screen flex-col'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 border-b backdrop-blur'>
        <div className='flex items-center justify-between gap-4'>
          <div className='flex min-w-0 items-center gap-3'>
            <Button variant='ghost' size='sm' onClick={handleBack} className='hover:bg-accent'>
              <ArrowLeft className='mr-2 h-4 w-4' />
              {t('common.back')}
            </Button>
            <Separator orientation='vertical' className='h-6' />
            <div className='min-w-0'>
              <div className='flex items-center gap-2'>
                <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-lg'>
                  <MessageSquare className='text-primary h-4 w-4' />
                </div>
                <h1 className='truncate text-base font-semibold'>{headerTitle}</h1>
              </div>
            </div>
          </div>

          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => {
                refetchInitial();
                refetchPull();
              }}
            >
              {t('common.refresh')}
            </Button>
          </div>
        </div>
      </Header>

      <Main className='flex flex-1 flex-col overflow-hidden'>
        <div className='flex flex-1 flex-col overflow-hidden p-6'>
          <Card className='flex flex-1 flex-col overflow-hidden p-4'>
            <div className='flex-1 space-y-3 overflow-y-auto pr-1'>
              {messages.length === 0 ? (
                <div className='text-muted-foreground text-sm'>{t('agents.chat.noMessages')}</div>
              ) : (
                messages.map((m) => renderMessage(m))
              )}
              <div ref={endRef} />
            </div>

            <Separator className='my-4' />

            <div className='flex items-end gap-2'>
              <Textarea
                value={text}
                onChange={(e) => setText(e.target.value)}
                placeholder='Type a message...'
                className='min-h-10 resize-none'
                onCompositionStart={() => setIsComposing(true)}
                onCompositionEnd={() => setIsComposing(false)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    if (isComposing || e.nativeEvent.isComposing) {
                      return;
                    }
                    e.preventDefault();
                    handleSend();
                  }
                }}
              />
              <Button onClick={handleSend} disabled={!text.trim() || send.isPending} className='shrink-0'>
                <Send className='mr-2 h-4 w-4' />
                Send
              </Button>
            </div>
          </Card>
        </div>
      </Main>
    </div>
  );
}
