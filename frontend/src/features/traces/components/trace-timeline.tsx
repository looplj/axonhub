"use client"

import { useMemo, useState } from 'react'
import { ChevronDown, ChevronRight, Circle, Workflow, MessageSquare, Sparkles, Wrench, CheckCircle2, Image, Settings } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

import type { Segment, RequestMetadata, Span } from '../data/schema'
import { getSpanDisplayLabels, normalizeSpanType } from '../utils/span-display'

type SpanKind = 'request' | 'response'

interface TraceTimelineProps {
  trace: Segment
  onSelectSpan: (trace: Segment, span: Span, type: SpanKind) => void
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
  color: string
  source:
    | {
        type: 'span'
        span: Span
        trace: Segment
        spanKind: SpanKind
      }
    | {
        type: 'segment'
        trace: Segment
      }
}

const segmentHueCache = new Map<string, number>()

type ColorVariant = 'segment' | 'request' | 'response'

function hashStringToHue(value: string): number {
  if (segmentHueCache.has(value)) {
    return segmentHueCache.get(value) as number
  }

  let hash = 0
  for (let i = 0; i < value.length; i += 1) {
    hash = (hash * 31 + value.charCodeAt(i)) % 360
  }

  const hue = (hash + 360) % 360
  segmentHueCache.set(value, hue)
  return hue
}

function getSegmentTimelineColor(segmentId: string, variant: ColorVariant): string {
  const hue = hashStringToHue(segmentId)
  const variantConfig: Record<ColorVariant, { lightness: number; alpha: number }> = {
    segment: { lightness: 56, alpha: 0.75 },
    request: { lightness: 64, alpha: 0.62 },
    response: { lightness: 70, alpha: 0.55 },
  }

  const { lightness, alpha } = variantConfig[variant]
  return `hsla(${hue}, 70%, ${lightness}%, ${alpha})`
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
  trace: Segment,
  span: Span,
  spanKind: SpanKind,
  rootStart: number
): TimelineNode | null {
  const spanStart = safeTime(span.startTime)
  const spanEnd = safeTime(span.endTime)
  if (spanStart == null || spanEnd == null) return null

  const duration = Math.max(spanEnd - spanStart, 0)
  const colorVariant: ColorVariant = spanKind === 'request' ? 'request' : 'response'
  return {
    id: span.id,
    name: span.type,
    type: span.type || 'default',
    startOffset: Math.max(spanStart - rootStart, 0),
    duration,
    metadata: trace.metadata,
    children: [],
    spanKind,
    color: getSegmentTimelineColor(trace.id, colorVariant),
    source: {
      type: 'span',
      span,
      trace,
      spanKind,
    },
  }
}

function buildSegmentNode(trace: Segment, rootStart: number): TimelineNode | null {
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
    color: getSegmentTimelineColor(trace.id, 'segment'),
    source: {
      type: 'segment',
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
    .map((child: Segment) => buildSegmentNode(child, rootStart))
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
  onSelectSpan: (trace: Segment, span: Span, kind: SpanKind) => void
}

function SpanRow({ node, depth, totalDuration, onSelectSpan, selectedSpanId }: SpanRowProps) {
  const { t } = useTranslation()
  const [isExpanded, setIsExpanded] = useState(true)
  const hasChildren = node.children.length > 0
  const spanSource = node.source.type === 'span' ? node.source : null
  const isActive = spanSource ? selectedSpanId === spanSource.span.id : false

  const leftOffsetRatio = totalDuration > 0 ? node.startOffset / totalDuration : 0
  const widthRatio = totalDuration > 0 ? node.duration / totalDuration : 0

  const leftOffset = Math.min(Math.max(leftOffsetRatio * 100, 0), 100)
  const maxAvailableWidth = Math.max(100 - leftOffset, 0)

  let width = Math.min(Math.max(widthRatio * 100, 0), maxAvailableWidth)
  if (widthRatio > 0 && width < 0.5) {
    width = Math.min(0.5, maxAvailableWidth)
  }
  const iconColor = node.source.type === 'segment' ? 'text-primary' : 'text-muted-foreground'
  const spanDisplay = spanSource ? getSpanDisplayLabels(spanSource.span, t) : null
  const spanKindLabel = spanSource ? t(`traces.common.badges.${spanSource.spanKind}`) : null
  const normalizedSpanType = spanSource ? normalizeSpanType(spanSource.span.type) : null

  const spanIcon = () => {
    switch (normalizedSpanType) {
      case 'user_query':
      case 'text':
      case 'message':
        return MessageSquare
      case 'thinking':
      case 'llm':
        return Sparkles
      case 'tool_use':
      case 'function_call':
        return Wrench
      case 'tool_result':
      case 'function_result':
        return CheckCircle2
      case 'user_image_url':
      case 'image_url':
        return Image
      case 'system_instruction':
        return Settings
      default:
        return Circle
    }
  }

  const SpanIcon = spanIcon()

  return (
    <div className="border-b border-border/40">
      <div
        className={cn(
          'flex items-center gap-3 py-2.5 px-3 transition-colors',
          spanSource ? 'hover:bg-accent/30 cursor-pointer' : 'cursor-default',
          isActive && 'bg-accent/40'
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
          {node.source.type === 'segment' ? (
            <Workflow className={cn('h-4 w-4', iconColor)} />
          ) : (
            <SpanIcon className={cn('h-4 w-4', iconColor)} />
          )}
        </div>

        <div className="flex min-w-0 flex-1 items-center gap-3">
          <div className="flex items-center gap-2 min-w-0 flex-1">
            {node.source.type === 'segment' ? (
              <>
                <Badge variant="secondary" className="text-xs font-medium">
                  {node.name}
                </Badge>
                <span className="text-xs text-muted-foreground">{formatDuration(node.duration)}</span>
                {node.metadata?.totalTokens && (
                  <span className="text-xs text-muted-foreground">
                    {t('traces.timeline.summary.tokenCount', {
                      value: node.metadata.totalTokens.toLocaleString(),
                    })}
                  </span>
                )}
              </>
            ) : (
              <div className="flex min-w-0 flex-col gap-1">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <span className="truncate text-sm font-medium">
                    {spanDisplay?.primary ?? node.name}
                  </span>
                  {spanKindLabel && (
                    <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                      {spanKindLabel}
                    </Badge>
                  )}
                  {spanDisplay?.secondary && (
                    <span className="truncate text-xs text-muted-foreground">
                      {spanDisplay.secondary}
                    </span>
                  )}
                </div>
                <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span>{formatDuration(node.duration)}</span>
                </div>
              </div>
            )}
          </div>
        </div>

        <div className="relative h-5 min-w-[180px] w-[180px] rounded bg-muted/30">
          <div
            className="absolute inset-y-0 rounded"
            style={{
              left: `${leftOffset}%`,
              width: `${width}%`,
              backgroundColor: node.color,
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

// Helper function to find the earliest start time across all segments
function findEarliestStart(segment: Segment): number | null {
  const times: number[] = []
  
  const collectTimes = (seg: Segment) => {
    const start = safeTime(seg.startTime)
    if (start != null) {
      times.push(start)
    }
    if (seg.children) {
      seg.children.forEach(collectTimes)
    }
  }
  
  collectTimes(segment)
  return times.length > 0 ? Math.min(...times) : null
}

// Helper function to find the latest end time across all segments
function findLatestEnd(segment: Segment): number | null {
  const times: number[] = []

  const collectTimes = (seg: Segment) => {
    const end = safeTime(seg.endTime)
    if (end != null) {
      times.push(end)
    }
    if (seg.children) {
      seg.children.forEach(collectTimes)
    }
  }

  collectTimes(segment)
  return times.length > 0 ? Math.max(...times) : null
}

export function TraceTimeline({ trace, onSelectSpan, selectedSpanId }: TraceTimelineProps) {
  const { t } = useTranslation()

  const timelineData = useMemo(() => {
    const earliestStart = findEarliestStart(trace)
    const latestEnd = findLatestEnd(trace)
    if (earliestStart == null || latestEnd == null) {
      return null
    }

    const rootNode = buildSegmentNode(trace, earliestStart)
    if (!rootNode) {
      return null
    }

    const totalDuration = Math.max(latestEnd - earliestStart, rootNode.duration, 1)

    return {
      rootNode,
      totalDuration: Math.max(totalDuration, 1),
    }
  }, [trace])

  if (!timelineData) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        {t('traces.timeline.emptyDescription')}
      </div>
    )
  }

  const { rootNode, totalDuration } = timelineData
  const totalItems = countNodes(rootNode)

  return (
    <div className="h-full flex flex-col">
      <div className="border-b border-border/60 pb-4 mb-4">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold">{t('traces.timeline.title')}</h2>
          <div className="text-sm text-muted-foreground">
            {t('traces.timeline.itemsCount', { count: totalItems })}
          </div>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-sm text-muted-foreground">
            {t('traces.timeline.totalDurationLabel')}
          </span>
          <span className="text-sm font-medium">{formatDuration(totalDuration)}</span>
          <div className="flex-1 h-6 rounded bg-muted/30 relative overflow-hidden">
            <div className="absolute inset-0 bg-gradient-to-r from-primary/60 to-primary/80 rounded" />
          </div>
        </div>
      </div>
      <div className="flex-1 overflow-auto border border-border/40 rounded-lg bg-card/50">
        <SpanRow
          node={rootNode}
          depth={0}
          totalDuration={totalDuration}
          onSelectSpan={onSelectSpan}
          selectedSpanId={selectedSpanId}
        />
      </div>
    </div>
  )
}
