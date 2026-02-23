import { useEffect, useMemo, useRef, useState } from 'react';
import { format } from 'date-fns';
import { zhCN, enUS } from 'date-fns/locale';
import { ArrowLeft, Send, MessageSquare, ShieldCheck, CheckCircle, XCircle, Info, Check, X, Globe, MessagesSquare } from 'lucide-react';
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
import { AgentChatMessage, AgentMessageKind, useAgentChatMessages, usePullAgentMessagesToUser, useSendAgentMessage, useResolveApproval, useAckAgentMessages } from '../data/agent-chat';

function messageKey(m: AgentChatMessage) {
  return `${m.id}:${m.sequence}`;
}

export function AgentChatPage() {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const navigate = useNavigate();
  const { agentId, threadId } = useParams({ from: '/_authenticated/project/agents/$agentId/threads/$threadId' as any }) as { agentId: string; threadId: string };
  const instanceID = threadId;

  const { data: agent } = useAgentDetail(agentId);
  const send = useSendAgentMessage();
  const resolveApproval = useResolveApproval();
  const ackMessages = useAckAgentMessages();
  const { data: initialMessages, refetch: refetchInitial } = useAgentChatMessages(agentId, instanceID);

  const [messages, setMessages] = useState<AgentChatMessage[]>([]);
  const [text, setText] = useState('');
  const [afterSequence, setAfterSequence] = useState(0);

  const endRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (initialMessages && initialMessages.length > 0) {
      setMessages(initialMessages);
      setAfterSequence(Math.max(...initialMessages.map((m) => m.sequence)));
    } else {
      setMessages([]);
      setAfterSequence(0);
    }
  }, [initialMessages]);

  const { data: pulledToUser, refetch: refetchPull } = usePullAgentMessagesToUser(agentId, instanceID, afterSequence);

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

    // Acknowledge received messages
    const messageIDs = pulledToUser.map((m) => m.id);
    if (messageIDs.length > 0) {
      ackMessages.mutate({ agentID: agentId, instanceID, messageIDs });
    }
  }, [pulledToUser, afterSequence, agentId, instanceID, ackMessages]);

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
      direction: 'to_runtime',
      senderType: 'user',
      senderID: null,
      kind: 'chat',
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
      const saved = await send.mutateAsync({ agentID: agentId, instanceID, text: trimmed });
      setMessages((prev) => prev.map((m) => (m.id === optimistic.id ? saved : m)));
      setAfterSequence((s) => Math.max(s, saved.sequence));
      await refetchInitial();
    } catch {
      setMessages((prev) => prev.filter((m) => m.id !== optimistic.id));
    }
  };

  const headerTitle = useMemo(() => {
    const agentName = agent?.name || agentId;
    return `${agentName} • ${instanceID}`;
  }, [agent?.name, agentId, instanceID]);

  const handleApprove = async (m: AgentChatMessage, granted: boolean, scope: 'once' | 'workspace' = 'once') => {
    const requestID = m.correlationID || (m.content?.id as string);
    if (!requestID) return;

    try {
      await resolveApproval.mutateAsync({
        agentID: agentId,
        requestID,
        granted,
        scope,
      });
      await refetchInitial();
    } catch (err) {
      console.error('Failed to resolve approval:', err);
    }
  };

  // Group messages: find approval_result for each approval_request
  const approvalResultsMap = useMemo(() => {
    const map = new Map<string, AgentChatMessage>();
    for (const m of messages) {
      if (m.kind === 'approval_result' && m.correlationID) {
        map.set(m.correlationID, m);
      }
    }
    return map;
  }, [messages]);

  const renderMessage = (m: AgentChatMessage) => {
    const ts = m.createdAt ? format(new Date(m.createdAt), 'HH:mm:ss', { locale }) : '';
    const kind: AgentMessageKind = m.kind || 'chat';
    const isUser = m.senderType === 'user';

    // Skip rendering approval_result as standalone message (will be rendered inside approval_request)
    if (kind === 'approval_result') {
      return null;
    }

    if (kind === 'system_event') {
      return (
        <div key={messageKey(m)} className="flex w-full justify-center">
          <div className="flex items-center gap-1.5 rounded-full border bg-muted/50 px-3 py-1 text-xs text-muted-foreground">
            <Info className="h-3 w-3" />
            {m.text || JSON.stringify(m.content)}
          </div>
        </div>
      );
    }

    if (kind === 'approval_request') {
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

      // Find corresponding approval_result
      const approvalResult = m.correlationID ? approvalResultsMap.get(m.correlationID) : undefined;
      const resultGranted = approvalResult ? Boolean(approvalResult.content?.granted) : undefined;

      return (
        <div key={messageKey(m)} className={cn('flex w-full', isUser ? 'justify-end' : 'justify-start')}>
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
                  <div className="text-xs text-muted-foreground mb-1">{t('agents.chat.parameters')}:</div>
                  <pre className="text-xs bg-amber-100/50 dark:bg-amber-900/50 rounded p-2 overflow-x-auto max-h-40 overflow-y-auto">
                    <code className="font-mono">{JSON.stringify(content.resources, null, 2)}</code>
                  </pre>
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

            {/* Approval Result - shown below the request */}
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

            {/* Action buttons or resolved status */}
            {isPending && !approvalResult ? (
              <div className="flex items-center gap-2 mt-3">
                <Button
                  size="sm"
                  variant="outline"
                  className="h-7 px-2 text-xs border-green-300 hover:bg-green-100 dark:border-green-700 dark:hover:bg-green-900"
                  onClick={() => handleApprove(m, true, 'once')}
                  disabled={resolveApproval.isPending}
                >
                  <Check className="h-3 w-3 mr-1" />
                  {t('agents.chat.approveOnce')}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="h-7 px-2 text-xs border-purple-300 hover:bg-purple-100 dark:border-purple-700 dark:hover:bg-purple-900"
                  onClick={() => handleApprove(m, true, 'thread')}
                  disabled={resolveApproval.isPending}
                >
                  <MessagesSquare className="h-3 w-3 mr-1" />
                  {t('agents.chat.approveThread')}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="h-7 px-2 text-xs border-blue-300 hover:bg-blue-100 dark:border-blue-700 dark:hover:bg-blue-900"
                  onClick={() => handleApprove(m, true, 'workspace')}
                  disabled={resolveApproval.isPending}
                >
                  <Globe className="h-3 w-3 mr-1" />
                  {t('agents.chat.approveWorkspace')}
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
            ) : !isPending && !approvalResult ? (
              <div className="text-xs text-muted-foreground italic mt-3">{t('agents.chat.approvalResolved')}</div>
            ) : null}

            <div className="mt-2 text-[11px] text-muted-foreground">seq {m.sequence} • {ts}</div>
          </div>
        </div>
      );
    }

    // kind === 'chat' (default)
    // Agent messages (from agent or system) show on left, user messages show on right
    return (
      <div key={messageKey(m)} className={cn('flex w-full', isUser ? 'justify-end' : 'justify-start')}>
        <div
          className={cn(
            'max-w-[80%] rounded-lg border px-3 py-2 text-sm',
            isUser ? 'bg-primary text-primary-foreground border-primary/20' : 'bg-muted'
          )}
        >
          <div className='whitespace-pre-wrap break-words'>{m.text}</div>
          <div className={cn('mt-1 text-[11px] opacity-80', isUser ? 'text-primary-foreground/80' : 'text-muted-foreground')}>
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
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
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
