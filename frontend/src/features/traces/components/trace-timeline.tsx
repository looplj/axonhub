"use client"

import { useMemo, useState } from 'react'
import { ChevronDown, ChevronRight, Link2, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

import type { RequestTrace, RequestMetadata, Span } from '../data/schema'

type SpanKind = 'request' | 'response'

interface TraceTimelineProps {
  trace: RequestTrace
  onSelectSpan: (trace: RequestTrace, span: Span, type: SpanKind) => void
  selectedSpanId?: string
}

interface TimelineNode {
  id: string
  name: string
  type: string
  startOffset: number
  duration: number
  metadata?: RequestMetadata | null
  children: TimelineNode[]
  spanKind?: SpanKind
  source:
    | {
        type: 'span'
        span: Span
        trace: RequestTrace
        spanKind: SpanKind
      }
    | {
        type: 'trace'
        trace: RequestTrace
      }
}

const spanTypeColors: Record<string, string> = {
  query: 'bg-primary/80',
  retrieve: 'bg-secondary/80',
  embedding: 'bg-secondary/80',
  synthesize: 'bg-accent/70',
  chunking: 'bg-accent/60',
  templating: 'bg-primary/60',
  llm: 'bg-primary',
  message: 'bg-primary/70',
  default: 'bg-muted',
}

function getTypeColor(type: string) {
  return spanTypeColors[type] || spanTypeColors.default
}

function safeTime(value?: Date | string | null) {
  if (!value) return null
  const date = value instanceof Date ? value : new Date(value)
  const time = date.getTime()
  return Number.isFinite(time) ? time : null
}

function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '0ms'
  if (ms < 1) return `${ms.toFixed(3)}ms`
  if (ms < 1000) return `${ms.toFixed(0)}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

function countNodes(node: TimelineNode): number {
  return 1 + node.children.reduce((acc, child) => acc + countNodes(child), 0)
}

function buildSpanNode(
  trace: RequestTrace,
  span: Span,
  spanKind: SpanKind,
  rootStart: number
): TimelineNode | null {
  const spanStart = safeTime(span.startTime)
  const spanEnd = safeTime(span.endTime)
  if (spanStart == null || spanEnd == null) return null

  const duration = Math.max(spanEnd - spanStart, 0)
  return {
    id: span.id,
    name: span.name || span.type,
    type: span.type || 'default',
    startOffset: Math.max(spanStart - rootStart, 0),
    duration,
    metadata: trace.metadata,
    children: [],
    spanKind,
    source: {
      type: 'span',
      span,
      trace,
      spanKind,
    },
  }
}

function buildTraceNode(trace: RequestTrace, rootStart: number): TimelineNode | null {
  const traceStart = safeTime(trace.startTime)
  const traceEnd = safeTime(trace.endTime)
  if (traceStart == null || traceEnd == null) return null

  const duration = Math.max(traceEnd - traceStart, 0)
  const node: TimelineNode = {
    id: trace.id,
    name: trace.model,
    type: trace.model?.toLowerCase() || 'default',
    startOffset: Math.max(traceStart - rootStart, 0),
    duration,
    metadata: trace.metadata,
    children: [],
    source: {
      type: 'trace',
      trace,
    },
  }

  const spanNodes = [
    ...(trace.requestSpans || [])
      .map((span: Span) => buildSpanNode(trace, span, 'request', rootStart))
      .filter(Boolean),
    ...(trace.responseSpans || [])
      .map((span: Span) => buildSpanNode(trace, span, 'response', rootStart))
      .filter(Boolean),
  ] as TimelineNode[]

  const childTraceNodes = (trace.children || [])
    .map((child: RequestTrace) => buildTraceNode(child, rootStart))
    .filter(Boolean) as TimelineNode[]

  spanNodes.sort((a, b) => a.startOffset - b.startOffset)
  childTraceNodes.sort((a, b) => a.startOffset - b.startOffset)

  node.children = [...spanNodes, ...childTraceNodes]

  return node
}

interface SpanRowProps {
  node: TimelineNode
  depth: number
  totalDuration: number
  selectedSpanId?: string
  onSelectSpan: (trace: RequestTrace, span: Span, kind: SpanKind) => void
}

function SpanRow({ node, depth, totalDuration, onSelectSpan, selectedSpanId }: SpanRowProps) {
  const { t } = useTranslation()
  const [isExpanded, setIsExpanded] = useState(true)
  const hasChildren = node.children.length > 0
  const spanSource = node.source.type === 'span' ? node.source : null
  const isActive = spanSource ? selectedSpanId === spanSource.span.id : false

  const leftOffset = totalDuration > 0 ? (node.startOffset / totalDuration) * 100 : 0
  const width = totalDuration > 0 ? (node.duration / totalDuration) * 100 : 0
  const colorClass = getTypeColor(node.type)
  const iconColor = node.source.type === 'trace' ? 'text-primary' : 'text-muted-foreground'

  return (
    <div className="border-b border-border/60">
      <div
        className={cn(
          'flex items-center gap-3 py-2.5 px-3 transition-colors',
          spanSource ? 'hover:bg-accent/30 cursor-pointer' : 'cursor-default',
          isActive && 'bg-accent/40 shadow-inner ring-1 ring-primary/60'
        )}
        style={{ paddingLeft: `${depth * 24 + 12}px` }}
        onClick={() => {
          if (spanSource) {
            onSelectSpan(spanSource.trace, spanSource.span, spanSource.spanKind)
          }
        }}
      >
        <button
          onClick={(event) => {
            event.stopPropagation()
            if (hasChildren) {
              setIsExpanded((prev) => !prev)
            }
          }}
          className={cn(
            'flex h-4 w-4 items-center justify-center rounded transition-colors',
            hasChildren ? 'hover:bg-accent text-muted-foreground' : 'opacity-0'
          )}
          aria-label={
            hasChildren
              ? isExpanded
                ? t('traces.timeline.aria.collapseRow')
                : t('traces.timeline.aria.expandRow')
              : undefined
          }
        >
          {hasChildren && (isExpanded ? (
            <ChevronDown className="h-3 w-3" />
          ) : (
            <ChevronRight className="h-3 w-3" />
          ))}
        </button>

        <div className="flex-shrink-0 text-muted-foreground">
          {node.type === 'llm' ? (
            <Zap className={cn('h-4 w-4', iconColor)} />
          ) : (
            <Link2 className={cn('h-4 w-4', iconColor)} />
          )}
        </div>

        <div className="flex min-w-0 flex-1 items-center gap-3">
          <div className="flex items-center gap-2 min-w-0">
            <span className="truncate text-sm font-medium">{node.name}</span>
            {node.source.type === 'trace' ? (
              <Badge variant="secondary" className="text-xs capitalize">
                {t('traces.common.badges.trace')}
              </Badge>
            ) : (
              <Badge variant="outline" className="text-xs capitalize">
                {spanSource?.spanKind ? t(`traces.common.badges.${spanSource.spanKind}`) : ''}
              </Badge>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <span>{formatDuration(node.duration)}</span>
            {node.metadata?.itemCount && (
              <span className="flex items-center gap-1">
                <span>#</span>
                <span>{node.metadata.itemCount}</span>
              </span>
            )}
            {node.metadata?.cost != null && (
              <span>${node.metadata.cost.toFixed(3)}</span>
            )}
            {node.metadata?.tokens && <span>{node.metadata.tokens}</span>}
          </div>
        </div>

        <div className="relative h-6 min-w-[200px] flex-1 rounded bg-muted/40">
          <div
            className={cn('absolute inset-y-0 rounded-lg', colorClass)}
            style={{
              left: `${Math.min(Math.max(leftOffset, 0), 100)}%`,
              width: `${Math.max(Math.min(width, 100), 0.5)}%`,
            }}
          />
        </div>
      </div>

      {hasChildren && isExpanded && (
        <div>
          {node.children.map((child) => (
            <SpanRow
              key={child.id}
              node={child}
              depth={depth + 1}
              totalDuration={totalDuration}
              onSelectSpan={onSelectSpan}
              selectedSpanId={selectedSpanId}
            />
          ))}
        </div>
      )}
    </div>
  )
}

export function TraceTimeline({ trace, onSelectSpan, selectedSpanId }: TraceTimelineProps) {
  const { t } = useTranslation()
  const timelineData = useMemo(() => {
    const start = safeTime(trace.startTime)
    const end = safeTime(trace.endTime)
    if (start == null || end == null || end <= start) {
      return null
    }

    const rootNode = buildTraceNode(trace, start)
    if (!rootNode) {
      return null
    }

    return {
      rootNode,
      totalDuration: Math.max(end - start, 1),
    }
  }, [trace])

  if (!timelineData) {
    return (
      <Card className="h-full border-0 shadow-sm">
        <CardHeader>
          <CardTitle>{t('traces.timeline.title')}</CardTitle>
        </CardHeader>
        <CardContent className="flex h-full items-center justify-center text-sm text-muted-foreground">
          {t('traces.timeline.emptyDescription')}
        </CardContent>
      </Card>
    )
  }

  const { rootNode, totalDuration } = timelineData
  const totalItems = countNodes(rootNode)

  return (
    <Card className="h-full border border-border/60 bg-card/90 shadow-sm backdrop-blur">
      <CardHeader className="border-b border-border/60">
        <div className="flex items-center justify-between">
          <CardTitle>{t('traces.timeline.title')}</CardTitle>
          <div className="text-sm text-muted-foreground">
            {t('traces.timeline.itemsCount', { count: totalItems })}
          </div>
        </div>
      </CardHeader>
      <CardContent className="flex-1 overflow-auto p-0">
        <div>
          <SpanRow
            node={rootNode}
            depth={0}
            totalDuration={totalDuration}
            onSelectSpan={onSelectSpan}
            selectedSpanId={selectedSpanId}
          />
        </div>
      </CardContent>
    </Card>
  )
}
