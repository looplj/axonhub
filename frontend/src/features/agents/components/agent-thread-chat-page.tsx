import { useEffect, useMemo, useRef, useState } from 'react';
import { format } from 'date-fns';
import { zhCN, enUS } from 'date-fns/locale';
import { ArrowLeft, Send, MessageSquare } from 'lucide-react';
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
import { AgentChatMessage, useAgentThreadMessages, usePullAgentMessagesToUser, useSendAgentMessage } from '../data/agent-chat';

function messageKey(m: AgentChatMessage) {
  return `${m.id}:${m.sequence}`;
}

export function AgentThreadChatPage() {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const navigate = useNavigate();
  const { agentId, threadId } = useParams({ from: '/_authenticated/project/agents/$agentId/threads/$threadId' as any }) as {
    agentId: string;
    threadId: string;
  };

  const { data: agent } = useAgentDetail(agentId);
  const send = useSendAgentMessage();
  const { data: initialMessages, refetch: refetchInitial } = useAgentThreadMessages(agentId, threadId);

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

  const { data: pulledToUser, refetch: refetchPull } = usePullAgentMessagesToUser(agentId, threadId, afterSequence);

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
  }, [pulledToUser, afterSequence]);

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
      threadID: threadId,
      direction: 'to_runtime',
      senderType: 'user',
      senderID: null,
      text: trimmed,
      sequence: afterSequence + 1,
      status: 'pending',
      createdAt: new Date(),
    };

    setText('');
    setMessages((prev) => [...prev, optimistic]);
    setAfterSequence((s) => s + 1);

    try {
      const saved = await send.mutateAsync({ agentID: agentId, threadID: threadId, text: trimmed });
      setMessages((prev) => prev.map((m) => (m.id === optimistic.id ? saved : m)));
      setAfterSequence((s) => Math.max(s, saved.sequence));
      await refetchInitial();
    } catch {
      setMessages((prev) => prev.filter((m) => m.id !== optimistic.id));
    }
  };

  const headerTitle = useMemo(() => {
    const agentName = agent?.name || agentId;
    return `${agentName} • ${threadId}`;
  }, [agent?.name, agentId, threadId]);

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
                <div className='text-muted-foreground text-sm'>No messages yet.</div>
              ) : (
                messages.map((m) => {
                  const isUser = m.senderType === 'user';
                  const ts = m.createdAt ? format(new Date(m.createdAt), 'HH:mm:ss', { locale }) : '';
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
                })
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
