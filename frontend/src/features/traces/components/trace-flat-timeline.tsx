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

interface FlatSegment {
  segment: TimelineNode
  spans: TimelineNode[]
  sequentialOffset: number // Offset in sequential layout
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

  // Use the duration field directly as it's already in milliseconds
  const duration = trace.duration
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

  spanNodes.sort((a, b) => a.startOffset - b.startOffset)
  node.children = spanNodes

  return node
}

function flattenSegments(node: TimelineNode, rootStart: number): FlatSegment[] {
  const result: FlatSegment[] = []
  let cumulativeOffset = 0
  
  const collectSegments = (segment: Segment) => {
    const segmentNode = buildSegmentNode(segment, rootStart)
    if (segmentNode) {
      result.push({
        segment: segmentNode,
        spans: segmentNode.children,
        sequentialOffset: cumulativeOffset,
      })
      // Accumulate duration for sequential layout
      cumulativeOffset += segmentNode.duration
    }
    
    if (segment.children) {
      segment.children.forEach(collectSegments)
    }
  }
  
  if (node.source.type === 'segment') {
    collectSegments(node.source.trace)
  }
  
  return result
}

interface SegmentRowProps {
  segment: TimelineNode
  spans: TimelineNode[]
  totalDuration: number
  sequentialOffset: number
  selectedSpanId?: string
  onSelectSpan: (trace: Segment, span: Span, kind: SpanKind) => void
  defaultExpanded?: boolean
}

function SegmentRow({ segment, spans, totalDuration, sequentialOffset, onSelectSpan, selectedSpanId, defaultExpanded = true }: SegmentRowProps) {
  const { t } = useTranslation()
  const [isExpanded, setIsExpanded] = useState(defaultExpanded)
  const hasSpans = spans.length > 0

  // Use sequential offset for segment positioning
  const leftOffsetRatio = totalDuration > 0 ? sequentialOffset / totalDuration : 0
  const widthRatio = totalDuration > 0 ? segment.duration / totalDuration : 0

  const leftOffset = Math.min(Math.max(leftOffsetRatio * 100, 0), 100)
  const maxAvailableWidth = Math.max(100 - leftOffset, 0)

  let width = Math.min(Math.max(widthRatio * 100, 0), maxAvailableWidth)
  if (widthRatio > 0 && width < 0.5) {
    width = Math.min(0.5, maxAvailableWidth)
  }

  return (
    <>
      {/* Segment Row - no indentation */}
      <div className="border-b border-border/40">
        <div
          className="flex items-center gap-3 py-2.5 px-3 transition-colors cursor-default"
        >
          <button
            onClick={(event) => {
              event.stopPropagation()
              if (hasSpans) {
                setIsExpanded((prev) => !prev)
              }
            }}
            className={cn(
              'flex h-4 w-4 items-center justify-center rounded transition-colors',
              hasSpans ? 'hover:bg-accent text-muted-foreground' : 'opacity-0'
            )}
            aria-label={
              hasSpans
                ? isExpanded
                  ? t('traces.timeline.aria.collapseRow')
                  : t('traces.timeline.aria.expandRow')
                : undefined
            }
          >
            {hasSpans && (isExpanded ? (
              <ChevronDown className="h-3 w-3" />
            ) : (
              <ChevronRight className="h-3 w-3" />
            ))}
          </button>

          <div className="flex-shrink-0 text-muted-foreground">
            <Workflow className="h-4 w-4 text-primary" />
          </div>

          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="flex items-center gap-2 min-w-0 flex-1">
              <Badge variant="secondary" className="text-xs font-medium">
                {segment.name}
              </Badge>
              <span className="text-xs text-muted-foreground">{formatDuration(segment.duration)}</span>
              {segment.metadata?.totalTokens && (
                <span className="text-xs text-muted-foreground">
                  {t('traces.timeline.summary.tokenCount', {
                    value: segment.metadata.totalTokens.toLocaleString(),
                  })}
                </span>
              )}
            </div>
          </div>

          <div className="relative h-5 min-w-[180px] w-[180px] rounded bg-muted/30">
            <div
              className="absolute inset-y-0 rounded"
              style={{
                left: `${leftOffset}%`,
                width: `${width}%`,
                backgroundColor: segment.color,
              }}
            />
          </div>
        </div>
      </div>

      {/* Spans - with indentation */}
      {hasSpans && isExpanded && (
        <div>
          {spans.map((span) => (
            <SpanRow
              key={span.id}
              span={span}
              totalDuration={totalDuration}
              segmentSequentialOffset={sequentialOffset}
              segmentDuration={segment.duration}
              onSelectSpan={onSelectSpan}
              selectedSpanId={selectedSpanId}
            />
          ))}
        </div>
      )}
    </>
  )
}

interface SpanRowProps {
  span: TimelineNode
  totalDuration: number
  segmentSequentialOffset: number
  segmentDuration: number
  selectedSpanId?: string
  onSelectSpan: (trace: Segment, span: Span, kind: SpanKind) => void
}

function SpanRow({ span, totalDuration, segmentSequentialOffset, segmentDuration, onSelectSpan, selectedSpanId }: SpanRowProps) {
  const { t } = useTranslation()
  const spanSource = span.source.type === 'span' ? span.source : null
  const isActive = spanSource ? selectedSpanId === spanSource.span.id : false

  if (!spanSource) return null

  // Position span within its segment's sequential range
  // Get the segment's actual start time from the span's source
  const segmentNode = spanSource.trace
  const segmentStartTime = safeTime(segmentNode.startTime)
  const spanStartTime = safeTime(spanSource.span.startTime)
  
  // Calculate span's offset within its segment (in milliseconds)
  const spanOffsetWithinSegment = (segmentStartTime != null && spanStartTime != null) 
    ? Math.max(spanStartTime - segmentStartTime, 0)
    : 0
  
  // Position in the sequential timeline
  const spanAbsoluteOffset = segmentSequentialOffset + spanOffsetWithinSegment
  
  const leftOffsetRatio = totalDuration > 0 ? spanAbsoluteOffset / totalDuration : 0
  const widthRatio = totalDuration > 0 ? span.duration / totalDuration : 0

  const leftOffset = Math.min(Math.max(leftOffsetRatio * 100, 0), 100)
  const maxAvailableWidth = Math.max(100 - leftOffset, 0)

  let width = Math.min(Math.max(widthRatio * 100, 0), maxAvailableWidth)
  if (widthRatio > 0 && width < 0.5) {
    width = Math.min(0.5, maxAvailableWidth)
  }

  const spanDisplay = getSpanDisplayLabels(spanSource.span, t)
  const spanKindLabel = t(`traces.common.badges.${spanSource.spanKind}`)
  const normalizedSpanType = normalizeSpanType(spanSource.span.type)

  const getSpanIcon = () => {
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

  const SpanIcon = getSpanIcon()

  return (
    <div className="border-b border-border/40">
      <div
        className={cn(
          'flex items-center gap-3 py-2.5 px-3 transition-colors hover:bg-accent/30 cursor-pointer',
          isActive && 'bg-accent/40'
        )}
        style={{ paddingLeft: '48px' }}
        onClick={() => {
          onSelectSpan(spanSource.trace, spanSource.span, spanSource.spanKind)
        }}
      >
        <div className="flex h-4 w-4" />

        <div className="flex-shrink-0 text-muted-foreground">
          <SpanIcon className="h-4 w-4 text-muted-foreground" />
        </div>

        <div className="flex min-w-0 flex-1 items-center gap-3">
          <div className="flex min-w-0 flex-col gap-1 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <span className="truncate text-sm font-medium">
                {spanDisplay?.primary ?? span.name}
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
              <span>{formatDuration(span.duration)}</span>
            </div>
          </div>
        </div>

        <div className="relative h-5 min-w-[180px] w-[180px] rounded bg-muted/30">
          <div
            className="absolute inset-y-0 rounded"
            style={{
              left: `${leftOffset}%`,
              width: `${width}%`,
              backgroundColor: span.color,
            }}
          />
        </div>
      </div>
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

export function TraceFlatTimeline({ trace, onSelectSpan, selectedSpanId }: TraceTimelineProps) {
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

    const flatSegments = flattenSegments(rootNode, earliestStart)
    // Total duration is the sum of all segment durations (sequential layout)
    const totalDuration = Math.max(
      flatSegments.reduce((sum, seg) => sum + seg.segment.duration, 0),
      1
    )

    // Count total items (segments + spans)
    const totalItems = flatSegments.reduce((acc, seg) => acc + 1 + seg.spans.length, 0)

    return {
      flatSegments,
      totalDuration: Math.max(totalDuration, 1),
      totalItems,
    }
  }, [trace])

  if (!timelineData) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        {t('traces.timeline.emptyDescription')}
      </div>
    )
  }

  const { flatSegments, totalDuration, totalItems } = timelineData

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
        {flatSegments.map((flatSegment, index) => (
          <SegmentRow
            key={flatSegment.segment.id}
            segment={flatSegment.segment}
            spans={flatSegment.spans}
            totalDuration={totalDuration}
            sequentialOffset={flatSegment.sequentialOffset}
            onSelectSpan={onSelectSpan}
            selectedSpanId={selectedSpanId}
            defaultExpanded={index < 10}
          />
        ))}
      </div>
    </div>
  )
}
