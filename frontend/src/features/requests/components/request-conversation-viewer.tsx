'use client';

import { useMemo, useState, useEffect, useCallback, useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ArrowUp, ChevronDown, ChevronsDownUp, ChevronsUpDown, FileText, Layers, Search, Wrench } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { parseRequestConversation, type ConversationData, type ConversationMessage, type ConversationTool } from '../utils/request-conversation';

interface RequestConversationViewerProps {
  body?: any;
  format?: string;
  className?: string;
}

const CHAR_LIMIT = 1000;

const ROLE_LABELS: Record<string, string> = {
  system: 'system',
  user: 'user',
  assistant: 'assistant',
  tool: 'tool',
};

const ROLE_PILL_CLASSES: Record<string, string> = {
  system: 'bg-purple-500/10 text-purple-600 dark:text-purple-400',
  user: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  assistant: 'bg-green-500/10 text-green-600 dark:text-green-400',
  tool: 'bg-orange-500/10 text-orange-600 dark:text-orange-400',
};

const ROLE_BORDER_CLASSES: Record<string, string> = {
  system: 'border-purple-500/40',
  user: 'border-blue-500/40',
  assistant: 'border-green-500/40',
  tool: 'border-orange-500/40',
};

const ROLE_ICON: Record<string, string> = {
  system: '⚙',
  user: '👤',
  assistant: '🤖',
  tool: '🔧',
};

function fmtNum(n: number): string {
  return n.toLocaleString('en-US');
}

function prettyJsonBlock(value: unknown): string {
  if (value === undefined || value === null) return '';
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function fmtToolChoice(value: unknown): ReactNode {
  if (value === undefined || value === null) return '—';
  if (typeof value === 'string') return value;
  return <small className='font-mono text-[10.5px]'>{prettyJsonBlock(value)}</small>;
}

function matchesSearch(m: ConversationMessage, q: string): boolean {
  if (!q) return true;
  const hay: string[] = [];
  if (m.content) hay.push(m.content);
  if (m.reasoning) hay.push(m.reasoning);
  if (m.toolCalls) m.toolCalls.forEach((tc) => hay.push(tc.name, tc.arguments));
  return hay.join('\n').toLowerCase().includes(q);
}

/** Toggleable long-text block with "expand all" / "collapse". */
function CollapseBlock({ text, expandAll, className }: { text: string; expandAll: boolean; className?: string }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const showFull = expandAll || expanded;
  const isLong = text.length > CHAR_LIMIT;
  const collapsed = isLong && !showFull;

  return (
    <div className={cn('relative', className)}>
      <div className={cn(collapsed && 'max-h-60 overflow-hidden')}>
        <pre className='text-foreground whitespace-pre-wrap break-words font-mono text-[12.5px] leading-relaxed'>{text}</pre>
      </div>
      {collapsed && <div className='from-background pointer-events-none absolute inset-x-0 bottom-7 h-14 bg-gradient-to-t to-transparent' />}
      {isLong && !expandAll && (
        <button
          type='button'
          onClick={() => setExpanded((v) => !v)}
          className='text-muted-foreground hover:text-foreground mt-1 flex w-full cursor-pointer items-center justify-center gap-1 rounded-md border bg-muted/40 px-2 py-1 text-xs transition-colors'
        >
          {expanded ? `${t('requests.conversation.collapse')} ▲` : `${t('requests.conversation.expandAllChars', { count: fmtNum(text.length) })} ▼`}
        </button>
      )}
    </div>
  );
}

/** Content part renderer (text / image / other JSON parts). */
const SKIP_PART_TYPES = new Set(['thinking', 'redacted_thinking', 'tool_use', 'tool_result', 'functionCall', 'functionResponse', 'reasoning']);

function ContentParts({ message, expandAll }: { message: ConversationMessage; expandAll: boolean }) {
  const parts = message.contentParts;
  if (!parts || parts.length === 0) return null;

  return (
    <div className='space-y-2'>
      {parts.map((p, i) => {
        if (typeof p === 'string') {
          return <CollapseBlock key={i} text={p} expandAll={expandAll} />;
        }
        if (p && typeof p === 'object') {
          if (typeof p.type === 'string' && SKIP_PART_TYPES.has(p.type)) return null;
          if (p.type === 'text' && typeof p.text === 'string') {
            return <CollapseBlock key={i} text={p.text} expandAll={expandAll} />;
          }
          if (p.type === 'image_url' || p.type === 'image' || p.image_url) {
            const url = p.image_url?.url || p.url || '';
            return (
              <div key={i} className='border-border bg-muted/30 inline-flex items-center gap-1 rounded-md border px-2 py-1 font-mono text-[11px] text-muted-foreground'>
                [image] {url ? url.slice(0, 80) : ''}
              </div>
            );
          }
          if (p.type === 'input_audio' || p.type === 'audio') {
            return (
              <div key={i} className='border-border bg-muted/30 inline-flex items-center gap-1 rounded-md border px-2 py-1 font-mono text-[11px] text-muted-foreground'>
                [audio]
              </div>
            );
          }
          return (
            <pre key={i} className='bg-muted/30 border-border overflow-x-auto rounded-md border p-2 font-mono text-[11.5px] text-muted-foreground'>
              {prettyJsonBlock(p)}
            </pre>
          );
        }
        return null;
      })}
    </div>
  );
}

interface ToolCallCardProps {
  call: { id?: string; name: string; arguments: string };
  resultIndex?: number;
  showArgs: boolean;
  jumpTo: (index: number) => void;
}

function ToolCallCard({ call, resultIndex, showArgs, jumpTo }: ToolCallCardProps) {
  const { t } = useTranslation();
  const [argsOpen, setArgsOpen] = useState(false);
  return (
    <div className='border-purple-500/40 bg-muted/30 border-l-4 rounded-md border p-2.5 pl-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <span className='font-mono text-[12.5px] font-semibold text-purple-600 dark:text-purple-400'>{call.name}</span>
        {call.id && <span className='text-muted-foreground font-mono text-[11px]'>{call.id}</span>}
        {resultIndex !== undefined && (
          <button
            type='button'
            onClick={() => jumpTo(resultIndex)}
            className='text-muted-foreground hover:text-foreground ml-auto cursor-pointer text-[11px] underline decoration-dotted underline-offset-2'
          >
            {t('requests.conversation.toolResultJump', { index: resultIndex })}
          </button>
        )}
      </div>
      {showArgs && call.arguments && (
        <>
          <button
            type='button'
            onClick={() => setArgsOpen((v) => !v)}
            className='text-muted-foreground hover:text-foreground mt-1.5 cursor-pointer text-[11px] font-medium'
          >
            {argsOpen ? `${t('requests.conversation.collapse')} ▲` : `${t('requests.conversation.viewArgsJson')} ▼`}
          </button>
          {argsOpen && (
            <pre className='bg-muted/40 border-border mt-1 overflow-x-auto rounded-md border p-2 font-mono text-[11.5px] text-muted-foreground'>
              {call.arguments}
            </pre>
          )}
        </>
      )}
    </div>
  );
}

interface ToolResultCardProps {
  callIndex?: number;
  content: string;
  jumpTo: (index: number) => void;
}

function ToolResultCard({ callIndex, content, jumpTo }: ToolResultCardProps) {
  const { t } = useTranslation();
  return (
    <div className='border-orange-500/40 bg-muted/30 border-l-4 rounded-md border p-2.5 pl-3'>
      <div className='mb-1 flex flex-wrap items-center gap-2'>
        <span className='text-[11px] font-semibold text-orange-600 dark:text-orange-400'>TOOL RESULT</span>
        {callIndex !== undefined && (
          <button
            type='button'
            onClick={() => jumpTo(callIndex)}
            className='text-muted-foreground hover:text-foreground cursor-pointer font-mono text-[11px] underline decoration-dotted underline-offset-2'
            title={t('requests.conversation.jumpToToolCall')}
          >
            {callIndex} ↑
          </button>
        )}
      </div>
      <CollapseBlock text={content} expandAll={false} />
    </div>
  );
}

interface MessageCardProps {
  message: ConversationMessage;
  showReasoning: boolean;
  showToolArgs: boolean;
  showToolResult: boolean;
  expandAll: boolean;
  toolCallByCallId: Map<string, number>;
  toolResultByCallId: Map<string, number>;
  jumpTo: (index: number) => void;
  onToggleRaw: (index: number) => void;
  rawOpen: boolean;
}

function MessageCard({
  message,
  showReasoning,
  showToolArgs,
  showToolResult,
  expandAll,
  toolCallByCallId,
  toolResultByCallId,
  jumpTo,
  onToggleRaw,
  rawOpen,
}: MessageCardProps) {
  const { t } = useTranslation();
  const role = message.role || 'tool';
  const pillClass = ROLE_PILL_CLASSES[role] || ROLE_PILL_CLASSES.tool;
  const borderClass = ROLE_BORDER_CLASSES[role] || ROLE_BORDER_CLASSES.tool;

  const headMeta: string[] = [];
  if (message.content && message.content.length) headMeta.push(`${fmtNum(message.content.length)} chars`);
  if (message.reasoning && message.reasoning.length) headMeta.push(`reasoning ${fmtNum(message.reasoning.length)}`);
  if (message.toolCalls && message.toolCalls.length) headMeta.push(`${message.toolCalls.length} tool call(s)`);

  let body: React.ReactNode = null;

  // Tool messages render their content only inside the TOOL RESULT card.
  if (role !== 'tool') {
    if (message.contentParts && message.contentParts.length > 0) {
      body = <ContentParts message={message} expandAll={expandAll} />;
    } else if (message.content) {
      body = <CollapseBlock text={message.content} expandAll={expandAll} />;
    }
  }

  if (message.reasoning && showReasoning) {
    body = (
      <div className='space-y-2.5'>
        <div className='border-amber-500/40 bg-amber-500/5 rounded-md border border-dashed p-2.5'>
          <div className='mb-1 text-[11px] font-semibold tracking-wider text-amber-600 dark:text-amber-400'>◆ REASONING</div>
          <details className='group'>
            <summary className='text-muted-foreground cursor-pointer text-[11.5px] underline decoration-dotted underline-offset-2'>{t('requests.conversation.expandReasoning')}</summary>
            <CollapseBlock text={message.reasoning} expandAll={expandAll} className='mt-1.5' />
          </details>
        </div>
        {body}
      </div>
    );
  }

  if (message.toolCalls && message.toolCalls.length > 0) {
    body = (
      <div className='space-y-2.5'>
        {body}
        <div className='space-y-1.5'>
          {message.toolCalls.map((tc, j) => (
            <ToolCallCard
              key={tc.id || j}
              call={tc}
              resultIndex={tc.id ? toolResultByCallId.get(tc.id) : undefined}
              showArgs={showToolArgs}
              jumpTo={jumpTo}
            />
          ))}
        </div>
      </div>
    );
  }

  if (role === 'tool' && showToolResult) {
    const callSrc = message.toolCallId ? toolCallByCallId.get(message.toolCallId) : undefined;
    body = (
      <div className='space-y-2.5'>
        {body}
        <ToolResultCard callIndex={callSrc} content={message.content} jumpTo={jumpTo} />
      </div>
    );
  }

  if (!body) body = <span className='text-muted-foreground italic'>(empty)</span>;

  return (
    <div id={`conv-msg-${message.index}`} className={cn('border-border bg-muted/20 overflow-hidden rounded-lg border', borderClass)}>
      <div className='border-border flex flex-wrap items-center gap-2 border-b px-3 py-2'>
        <span className={cn('inline-flex items-center rounded-full px-2 py-0.5 font-mono text-[11px]', pillClass)}>{ROLE_LABELS[role] || role}</span>
        <span className='text-muted-foreground font-mono text-[11px]'>#{message.index}</span>
        <div className='ml-auto flex flex-wrap items-center gap-1.5'>
          {headMeta.map((meta, i) => (
            <span key={i} className='border-border bg-muted/40 text-muted-foreground rounded-full border px-2 py-0.5 font-mono text-[10.5px]'>
              {meta}
            </span>
          ))}
          <Button variant='ghost' size='icon-sm' className='h-6 w-6' onClick={() => onToggleRaw(message.index)} title={t('requests.conversation.viewRaw')}>
            <FileText className='h-3.5 w-3.5' />
          </Button>
        </div>
      </div>
      {rawOpen && (
        <div className='border-border bg-muted/40 border-b px-3 py-2.5'>
          <div className='text-muted-foreground mb-1 font-mono text-[10.5px]'>Raw JSON — message #{message.index}</div>
          <pre className='text-muted-foreground max-h-72 overflow-auto whitespace-pre-wrap break-words font-mono text-[11.5px] leading-relaxed'>
            {prettyJsonBlock(message.raw)}
          </pre>
        </div>
      )}
      <div className='space-y-2.5 p-3'>{body}</div>
    </div>
  );
}

interface ToolCardProps {
  tool: ConversationTool;
}

function ToolCard({ tool }: ToolCardProps) {
  const [open, setOpen] = useState(false);
  const params = tool.parameters && typeof tool.parameters === 'object' ? (tool.parameters as Record<string, any>) : {};
  const required = Array.isArray(params.required) ? params.required.join(', ') : '';
  const propCount = params.properties ? Object.keys(params.properties).length : 0;

  return (
    <div className='border-border bg-muted/20 overflow-hidden rounded-lg border'>
      <button
        type='button'
        onClick={() => setOpen((v) => !v)}
        className='flex w-full cursor-pointer items-center gap-3 px-3.5 py-2.5 text-left'
      >
        <Wrench className='text-cyan-600 dark:text-cyan-400 h-4 w-4 shrink-0' />
        <span className='font-mono text-[13px] font-semibold text-cyan-700 dark:text-cyan-300'>{tool.name}</span>
        {tool.description && (
          <span className='text-muted-foreground min-w-0 flex-1 truncate text-xs'>
            {tool.description.replace(/\s+/g, ' ').slice(0, 140)}
            {tool.description.length > 140 ? '…' : ''}
          </span>
        )}
        <span className='text-muted-foreground shrink-0 font-mono text-[11px]'>{propCount} props{required ? ` · required: ${required}` : ''}</span>
        <ChevronDown className={cn('text-muted-foreground h-4 w-4 shrink-0 transition-transform', open && 'rotate-180')} />
      </button>
      {open && (
        <div className='border-border border-t p-3.5'>
          {tool.description && <div className='text-muted-foreground mb-2.5 text-xs whitespace-pre-wrap'>{tool.description}</div>}
          <div className='text-muted-foreground mb-1 font-mono text-[10.5px]'>
            Parameters (JSON Schema) · required: <span className='text-orange-600 dark:text-orange-400'>{required || '—'}</span>
          </div>
          <pre className='bg-muted/40 border-border overflow-x-auto rounded-md border p-2.5 font-mono text-[11.5px] text-muted-foreground'>
            {prettyJsonBlock(tool.parameters)}
          </pre>
        </div>
      )}
    </div>
  );
}

function Stat({ label, value, jumpId, onJump }: { label: string; value: React.ReactNode; jumpId?: string; onJump?: (id: string) => void }) {
  return (
    <div
      className={cn('border-border bg-muted/20 rounded-lg border px-3 py-2', jumpId && 'cursor-pointer hover:border-primary')}
      onClick={jumpId ? () => onJump?.(jumpId) : undefined}
      role={jumpId ? 'button' : undefined}
    >
      <div className='text-muted-foreground text-[10px] tracking-wider uppercase'>{label}</div>
      <div className='text-foreground mt-0.5 truncate font-mono text-sm font-semibold'>{value}</div>
    </div>
  );
}

export function RequestConversationViewer({ body, format, className }: RequestConversationViewerProps) {
  const { t } = useTranslation();
  const data: ConversationData | null = useMemo(() => parseRequestConversation(body, format), [body, format]);

  const [search, setSearch] = useState('');
  const [roleFilter, setRoleFilter] = useState('');
  const [showReasoning, setShowReasoning] = useState(true);
  const [showToolArgs, setShowToolArgs] = useState(true);
  const [showToolResult, setShowToolResult] = useState(true);
  const [showSystem, setShowSystem] = useState(true);
  const [expandAllContent, setExpandAllContent] = useState(false);
  const [rawOpenIndex, setRawOpenIndex] = useState<number | null>(null);
  const [showBackTop, setShowBackTop] = useState(false);

  const jumpTo = useCallback((target: string | number) => {
    const el = typeof target === 'number' ? document.getElementById(`conv-msg-${target}`) : document.getElementById(target);
    el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }, []);

  const rootRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let scroller: HTMLElement | null = null;
    let node: HTMLElement | null = rootRef.current;
    while (node) {
      const style = getComputedStyle(node);
      if (/(auto|scroll|overlay)/.test(style.overflowY)) {
        scroller = node;
        break;
      }
      node = node.parentElement;
    }
    const target = scroller ?? window;
    const onScroll = () => {
      const top = scroller ? scroller.scrollTop : window.scrollY;
      setShowBackTop(top > 400);
    };
    target.addEventListener('scroll', onScroll, { passive: true });
    return () => target.removeEventListener('scroll', onScroll);
  }, []);

  const scrollToTop = useCallback(() => {
    let node: HTMLElement | null = rootRef.current;
    while (node) {
      const style = getComputedStyle(node);
      if (/(auto|scroll|overlay)/.test(style.overflowY)) {
        node.scrollTo({ top: 0, behavior: 'smooth' });
        return;
      }
      node = node.parentElement;
    }
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }, []);

  const toolCallByCallId = useMemo(() => {
    const map = new Map<string, number>();
    if (!data) return map;
    data.messages.forEach((m) => {
      if (m.toolCalls) m.toolCalls.forEach((tc) => tc.id && map.set(tc.id, m.index));
    });
    return map;
  }, [data]);

  const toolResultByCallId = useMemo(() => {
    const map = new Map<string, number>();
    if (!data) return map;
    data.messages.forEach((m) => {
      if (m.role === 'tool' && m.toolCallId) map.set(m.toolCallId, m.index);
    });
    return map;
  }, [data]);

  const visibleMessages = useMemo(() => {
    if (!data) return [];
    const q = search.trim().toLowerCase();
    return data.messages.filter((m) => {
      if (roleFilter && m.role !== roleFilter) return false;
      if (m.role === 'system' && !showSystem) return false;
      return matchesSearch(m, q);
    });
  }, [data, search, roleFilter, showSystem]);

  const sidebarGroups = useMemo(() => {
    if (!data) return [] as { role: string; items: { index: number; preview: string }[] }[];
    const groups: Record<string, { index: number; preview: string }[]> = {};
    data.messages.forEach((m) => {
      const g = m.role === 'system' ? 'system' : m.role || 'tool';
      if (m.role === 'system' && !showSystem) return;
      const preview =
        (m.content && typeof m.content === 'string' ? m.content : '') ||
        (m.toolCalls && m.toolCalls.length ? m.toolCalls.map((tc) => tc.name).join(', ') : '') ||
        (m.reasoning ? '…' : '');
      (groups[g] = groups[g] || []).push({ index: m.index, preview });
    });
    return Object.keys(ROLE_ICON)
      .filter((r) => groups[r])
      .map((r) => ({ role: r, items: groups[r] }));
  }, [data, showSystem]);

  if (!data) {
    return (
      <div className={cn('border-border bg-muted/20 flex h-48 w-full items-center justify-center rounded-lg border', className)}>
        <div className='space-y-2 text-center'>
          <Layers className='text-muted-foreground mx-auto h-10 w-10' />
          <p className='text-muted-foreground text-sm'>{t('requests.conversation.noMessages')}</p>
        </div>
      </div>
    );
  }

  const roles = data.messages.reduce<Record<string, number>>((acc, m) => {
    acc[m.role] = (acc[m.role] || 0) + 1;
    return acc;
  }, {});
  const totalChars = data.messages.reduce((s, m) => s + (m.content ? m.content.length : 0), 0);
  const totalToolCalls = data.messages.reduce((s, m) => s + (m.toolCalls ? m.toolCalls.length : 0), 0);

  const toggleRaw = (index: number) => setRawOpenIndex((cur) => (cur === index ? null : index));

  return (
    <div ref={rootRef} className={cn('space-y-4', className)}>
      {/* Toolbar */}
      <div className='border-border bg-muted/20 sticky top-0 z-10 rounded-lg border p-3 backdrop-blur'>
        <div className='flex flex-wrap items-center gap-2.5'>
          <div className='flex items-center gap-2'>
            <span className='h-2.5 w-2.5 rounded-full bg-blue-500' />
            <span className='text-sm font-semibold'>{t('requests.conversation.title')}</span>
          </div>
          {data.model && (
            <span className='border-blue-500/40 bg-blue-500/10 text-blue-600 dark:text-blue-400 rounded-full border px-2.5 py-0.5 font-mono text-[11.5px]'>
              {data.model}
            </span>
          )}
          <div className='ml-auto flex flex-wrap items-center gap-2'>
            <div className='relative'>
              <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2' />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t('requests.conversation.searchPlaceholder')}
                className='h-8 w-48 pl-8 text-xs md:w-60'
              />
            </div>
            <select
              value={roleFilter}
              onChange={(e) => setRoleFilter(e.target.value)}
              className='border-border bg-muted/40 text-muted-foreground h-8 rounded-md border px-2 text-xs outline-none'
            >
              <option value=''>{t('requests.conversation.allRoles')}</option>
              {Object.keys(ROLE_LABELS).map((r) => (
                <option key={r} value={r}>
                  {ROLE_LABELS[r]}
                </option>
              ))}
            </select>
            <label className='text-muted-foreground flex cursor-pointer items-center gap-1.5 text-xs whitespace-nowrap'>
              <input type='checkbox' className='accent-blue-500' checked={showReasoning} onChange={(e) => setShowReasoning(e.target.checked)} />
              {t('requests.conversation.toggleReasoning')}
            </label>
            <label className='text-muted-foreground flex cursor-pointer items-center gap-1.5 text-xs whitespace-nowrap'>
              <input type='checkbox' className='accent-blue-500' checked={showToolArgs} onChange={(e) => setShowToolArgs(e.target.checked)} />
              {t('requests.conversation.toggleToolArgs')}
            </label>
            <label className='text-muted-foreground flex cursor-pointer items-center gap-1.5 text-xs whitespace-nowrap'>
              <input type='checkbox' className='accent-blue-500' checked={showToolResult} onChange={(e) => setShowToolResult(e.target.checked)} />
              {t('requests.conversation.toggleToolResult')}
            </label>
            <label className='text-muted-foreground flex cursor-pointer items-center gap-1.5 text-xs whitespace-nowrap'>
              <input type='checkbox' className='accent-blue-500' checked={showSystem} onChange={(e) => setShowSystem(e.target.checked)} />
              system
            </label>
            <Button variant='outline' size='sm' className='h-8 px-2.5 text-xs' onClick={() => setExpandAllContent((v) => !v)}>
              {expandAllContent ? <ChevronsDownUp className='h-3.5 w-3.5' /> : <ChevronsUpDown className='h-3.5 w-3.5' />}
              {expandAllContent ? t('requests.conversation.collapseAll') : t('requests.conversation.expandAll')}
            </Button>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className='grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5'>
        <Stat label='Model' value={data.model ? <small className='text-xs'>{data.model}</small> : '—'} />
        <Stat label={t('requests.conversation.statMessages')} value={fmtNum(data.messages.length)} />
        <Stat label={t('requests.conversation.statTools')} value={fmtNum(data.tools.length)} jumpId='conv-tools' onJump={jumpTo} />
        <Stat label={t('requests.conversation.statToolCalls')} value={fmtNum(totalToolCalls)} />
        <Stat label='max_tokens' value={data.maxTokens != null ? fmtNum(data.maxTokens) : '—'} />
        <Stat label='stream' value={String(data.stream ?? '—')} />
        <Stat label='tool_choice' value={fmtToolChoice(data.toolChoice)} />
        <Stat label={t('requests.conversation.statChars')} value={fmtNum(totalChars)} />
        <Stat label='≈Tokens' value={fmtNum(Math.round(totalChars / 3.5))} />
        <Stat
          label={t('requests.conversation.statRoles')}
          value={
            <span className='flex flex-wrap gap-1'>
              {Object.entries(roles).map(([r, c]) => (
                <span key={r} className={cn('inline-flex items-center rounded-full px-1.5 py-0.5 font-mono text-[10.5px]', ROLE_PILL_CLASSES[r] || '')}>
                  {r} {c}
                </span>
              ))}
            </span>
          }
        />
      </div>

      {/* Layout */}
      <div className='grid grid-cols-1 gap-4 lg:grid-cols-[260px_1fr]'>
        {/* Sidebar jump */}
        <div className='hidden lg:block'>
          <div className='border-border bg-muted/20 sticky top-[72px] max-h-[calc(100vh-72px-120px-16px)] overflow-y-auto rounded-lg border p-2.5'>
            {sidebarGroups.map((g) => (
              <div key={g.role} className='mb-1.5'>
                <div className='text-muted-foreground px-1.5 py-0.5 font-mono text-[10.5px] tracking-wider uppercase'>
                  {ROLE_ICON[g.role]} {g.role} ({g.items.length})
                </div>
                {g.items.map((item) => (
                  <button
                    key={item.index}
                    type='button'
                    onClick={() => jumpTo(item.index)}
                    className='hover:bg-muted/50 text-muted-foreground hover:text-foreground flex w-full cursor-pointer items-center gap-2 rounded-md px-1.5 py-1 text-left font-mono text-[11px]'
                  >
                    <span className='text-muted-foreground/70 w-7 shrink-0'>#{item.index}</span>
                    <span className='truncate'>{item.preview.replace(/\n/g, ' ').slice(0, 44)}</span>
                  </button>
                ))}
              </div>
            ))}
          </div>
        </div>

        {/* Main */}
        <div className='min-w-0 space-y-5'>
          <div>
            <div className='text-muted-foreground mb-2.5 text-[13px] font-semibold'>
              Messages <span className='text-muted-foreground/70'>({fmtNum(visibleMessages.length)})</span>
            </div>
            {visibleMessages.length === 0 ? (
              <div className='border-border bg-muted/20 flex h-40 items-center justify-center rounded-lg border'>
                <p className='text-muted-foreground text-sm'>{t('requests.conversation.noMatch')}</p>
              </div>
            ) : (
              <div className='space-y-3'>
                {visibleMessages.map((m) => (
                  <MessageCard
                    key={m.index}
                    message={m}
                    showReasoning={showReasoning}
                    showToolArgs={showToolArgs}
                    showToolResult={showToolResult}
                    expandAll={expandAllContent}
                    toolCallByCallId={toolCallByCallId}
                    toolResultByCallId={toolResultByCallId}
                    jumpTo={jumpTo}
                    onToggleRaw={toggleRaw}
                    rawOpen={rawOpenIndex === m.index}
                  />
                ))}
              </div>
            )}
          </div>

          <div id='conv-tools' className='scroll-mt-24'>
            <div className='text-muted-foreground mb-2.5 text-[13px] font-semibold'>
              Tools <span className='text-muted-foreground/70'>({fmtNum(data.tools.length)})</span>
            </div>
            {data.tools.length === 0 ? (
              <div className='border-border bg-muted/20 flex h-24 items-center justify-center rounded-lg border'>
                <p className='text-muted-foreground text-sm'>{t('requests.conversation.noTools')}</p>
              </div>
            ) : (
              <div className='space-y-2'>
                {data.tools.map((tool, i) => (
                  <ToolCard key={tool.name || i} tool={tool} />
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Back to top */}
      {showBackTop && (
        <Button
          variant='outline'
          size='icon'
          className='fixed right-5 bottom-5 z-50 h-11 w-11 rounded-full shadow-lg'
          onClick={scrollToTop}
          title={t('requests.conversation.backToTop')}
        >
          <ArrowUp className='h-5 w-5' />
        </Button>
      )}
    </div>
  );
}
