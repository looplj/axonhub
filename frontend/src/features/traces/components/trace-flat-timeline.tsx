'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronDown, ChevronRight, Workflow, ChevronsDownUp, ExternalLink, Filter } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { AnimatePresence, motion } from 'framer-motion';
import { buildGUID, cn } from '@/lib/utils';
import { formatNumber } from '@/utils/format-number';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { formatDuration } from '../../../utils/format-duration';
import type { Segment, RequestMetadata, Span } from '../data/schema';
import { getSpanDisplayLabels, normalizeSpanType } from '../utils/span-display';
import { getSpanIcon } from './constant';

type SpanKind = 'request' | 'response';

interface TraceTimelineProps {
  trace: Segment;
  onSelectSpan: (trace: Segment, span: Span, type: SpanKind) => void;
  selectedSpanId?: string;
}

interface TimelineNode {
  id: string;
  name: string;
  type: string;
  startOffset: number;
  duration: number;
  metadata?: RequestMetadata | null;
  children: TimelineNode[];
  spanKind?: SpanKind;
  color: string;
  source:
    | {
        type: 'span';
        span: Span;
        trace: Segment;
        spanKind: SpanKind;
      }
    | {
        type: 'segment';
        trace: Segment;
      };
}

interface FlatSegment {
  segment: TimelineNode;
  spans: TimelineNode[];
  sequentialOffset: number; // Offset in sequential layout
}

type ColorVariant = 'segment' | 'request' | 'response';

// 使用 WeakMap 缓存，避免内存泄漏（如果 key 是对象）
// 这里使用普通 Map，但限制缓存大小
const MAX_HUE_CACHE_SIZE = 100;
const segmentHueCache = new Map<string, number>();

function hashStringToHue(value: string): number {
  const cached = segmentHueCache.get(value);
  if (cached !== undefined) {
    return cached;
  }

  let hash = 0;
  for (let i = 0; i < value.length; i += 1) {
    hash = (hash * 31 + value.charCodeAt(i)) % 360;
  }

  const hue = (hash + 360) % 360;
  
  // LRU 缓存策略：超过限制时清除最早的
  if (segmentHueCache.size >= MAX_HUE_CACHE_SIZE) {
    const firstKey = segmentHueCache.keys().next().value;
    if (firstKey) {
      segmentHueCache.delete(firstKey);
    }
  }
  
  segmentHueCache.set(value, hue);
  return hue;
}

function getSegmentTimelineColor(segmentId: string, variant: ColorVariant): string {
  const hue = hashStringToHue(segmentId);
  const variantConfig: Record<ColorVariant, { lightness: number; alpha: number }> = {
    segment: { lightness: 56, alpha: 0.75 },
    request: { lightness: 64, alpha: 0.62 },
    response: { lightness: 70, alpha: 0.55 },
  };

  const { lightness, alpha } = variantConfig[variant];
  return `hsla(${hue}, 70%, ${lightness}%, ${alpha})`;
}

function safeTime(value?: Date | string | null): number | null {
  if (!value) return null;
  
  try {
    const date = value instanceof Date ? value : new Date(value);
    const time = date.getTime();
    return Number.isFinite(time) ? time : null;
  } catch {
    return null;
  }
}

/**
 * 计算时间条的位置和宽度
 * @param startRatio 开始位置的比率 (0-1)
 * @param widthRatio 宽度的比率 (0-1)
 * @returns { left: number, width: number } 返回百分比数值
 */
function calculateBarPosition(startRatio: number, widthRatio: number): { left: number; width: number } {
  const leftOffset = Math.min(Math.max(startRatio * 100, 0), 100);
  const maxAvailableWidth = Math.max(100 - leftOffset, 0);

  let width = Math.min(Math.max(widthRatio * 100, 0), maxAvailableWidth);
  // 确保至少有最小宽度（如果 widthRatio > 0）
  if (widthRatio > 0 && width < 0.5) {
    width = Math.min(0.5, maxAvailableWidth);
  }

  return { left: leftOffset, width };
}

function buildSpanNode(trace: Segment, span: Span, spanKind: SpanKind, rootStart: number): TimelineNode | null {
  const spanStart = safeTime(span.startTime);
  const spanEnd = safeTime(span.endTime);
  if (spanStart == null || spanEnd == null) return null;

  const duration = Math.max(spanEnd - spanStart, 0);
  const colorVariant: ColorVariant = spanKind === 'request' ? 'request' : 'response';
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
  };
}

function buildSegmentNode(trace: Segment, rootStart: number): TimelineNode | null {
  const traceStart = safeTime(trace.startTime);
  const traceEnd = safeTime(trace.endTime);
  if (traceStart == null || traceEnd == null) return null;

  // Use the duration field directly as it's already in milliseconds
  const duration = trace.duration;
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
  };

  const spanNodes = [
    ...(trace.requestSpans || []).map((span: Span) => buildSpanNode(trace, span, 'request', rootStart)).filter(Boolean),
    ...(trace.responseSpans || []).map((span: Span) => buildSpanNode(trace, span, 'response', rootStart)).filter(Boolean),
  ] as TimelineNode[];

  spanNodes.sort((a, b) => a.startOffset - b.startOffset);
  node.children = spanNodes;

  return node;
}

function flattenSegments(node: TimelineNode, rootStart: number): FlatSegment[] {
  const result: FlatSegment[] = [];
  let cumulativeOffset = 0;

  const collectSegments = (segment: Segment) => {
    const segmentNode = buildSegmentNode(segment, rootStart);
    if (segmentNode) {
      result.push({
        segment: segmentNode,
        spans: segmentNode.children,
        sequentialOffset: cumulativeOffset,
      });
      // Accumulate duration for sequential layout
      cumulativeOffset += segmentNode.duration;
    }

    if (segment.children) {
      segment.children.forEach(collectSegments);
    }
  };

  if (node.source.type === 'segment') {
    collectSegments(node.source.trace);
  }

  return result;
}

interface SegmentRowProps {
  segment: TimelineNode;
  spans: TimelineNode[];
  totalDuration: number;
  sequentialOffset: number;
  selectedSpanId?: string;
  onSelectSpan: (trace: Segment, span: Span, kind: SpanKind) => void;
  defaultExpanded?: boolean;
  isExpanded: boolean;
  onToggleExpand: () => void;
}

function SegmentRow({
  segment,
  spans,
  totalDuration,
  sequentialOffset,
  onSelectSpan,
  selectedSpanId,
  isExpanded,
  onToggleExpand,
}: SegmentRowProps) {
  const { t } = useTranslation();
  const hasSpans = spans.length > 0;

  const handleViewRequest = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (segment.source.type === 'segment') {
      const requestId = segment.source.trace.id;
      const url = `/project/requests/${encodeURIComponent(buildGUID('Request', requestId))}`;
      window.open(url, '_blank', 'noopener,noreferrer');
    }
  };

  // Use sequential offset for segment positioning
  const leftOffsetRatio = totalDuration > 0 ? sequentialOffset / totalDuration : 0;
  const widthRatio = totalDuration > 0 ? segment.duration / totalDuration : 0;

  const { left: leftOffset, width } = calculateBarPosition(leftOffsetRatio, widthRatio);

  return (
    <>
      {/* Segment Row - no indentation */}
      <div className='border-border/40 border-b'>
        <div className='flex cursor-default items-center gap-3 px-3 py-2.5 transition-colors'>
          <button
            onClick={(event) => {
              event.stopPropagation();
              if (hasSpans) {
                onToggleExpand();
              }
            }}
            className={cn(
              'flex h-4 w-4 items-center justify-center rounded transition-colors',
              hasSpans ? 'hover:bg-accent text-muted-foreground' : 'opacity-0'
            )}
            aria-label={hasSpans ? (isExpanded ? t('traces.timeline.aria.collapseRow') : t('traces.timeline.aria.expandRow')) : undefined}
          >
            {hasSpans && (isExpanded ? <ChevronDown className='h-3 w-3' /> : <ChevronRight className='h-3 w-3' />)}
          </button>

          <div className='text-muted-foreground flex-shrink-0'>
            <Workflow className='text-primary h-4 w-4' />
          </div>

          <div className='flex min-w-0 flex-1 items-center gap-3'>
            <div className='flex min-w-0 flex-1 items-center gap-2'>
              <Badge variant='secondary' className='text-xs font-medium'>
                {segment.name}
              </Badge>
              <span className='text-muted-foreground text-[11px]'>
              {t('traces.timeline.summary.duration', {
                  value: formatDuration(segment.duration),
                })}
              </span>
              <Button variant='ghost' size='sm' className='h-6 w-6 p-0' onClick={handleViewRequest}>
                <ExternalLink className='h-3 w-3' />
              </Button>
            </div>
          </div>

          <div className='bg-muted/30 relative h-5 w-[180px] min-w-[180px] rounded'>
            <div
              className='absolute inset-y-0 rounded'
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
      <AnimatePresence>
        {hasSpans && isExpanded && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.2, ease: 'easeInOut' }}
          >
            {spans.map((span) => (
              <SpanRow
                key={span.id}
                span={span}
                totalDuration={totalDuration}
                segmentSequentialOffset={sequentialOffset}
                onSelectSpan={onSelectSpan}
                selectedSpanId={selectedSpanId}
              />
            ))}
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}

interface SpanRowProps {
  span: TimelineNode;
  totalDuration: number;
  segmentSequentialOffset: number;
  selectedSpanId?: string;
  onSelectSpan: (trace: Segment, span: Span, kind: SpanKind) => void;
}

function SpanRow({ span, totalDuration, segmentSequentialOffset, onSelectSpan, selectedSpanId }: SpanRowProps) {
  const { t } = useTranslation();
  const spanSource = span.source.type === 'span' ? span.source : null;
  const isActive = spanSource ? selectedSpanId === spanSource.span.id : false;

  if (!spanSource) return null;

  // Position span within its segment's sequential range
  // Get the segment's actual start time from the span's source
  const segmentNode = spanSource.trace;
  const segmentStartTime = safeTime(segmentNode.startTime);
  const spanStartTime = safeTime(spanSource.span.startTime);

  // Calculate span's offset within its segment (in milliseconds)
  const spanOffsetWithinSegment = segmentStartTime != null && spanStartTime != null ? Math.max(spanStartTime - segmentStartTime, 0) : 0;

  // Position in the sequential timeline
  const spanAbsoluteOffset = segmentSequentialOffset + spanOffsetWithinSegment;

  const leftOffsetRatio = totalDuration > 0 ? spanAbsoluteOffset / totalDuration : 0;
  const widthRatio = totalDuration > 0 ? span.duration / totalDuration : 0;

  const { left: leftOffset, width } = calculateBarPosition(leftOffsetRatio, widthRatio);

  const spanDisplay = getSpanDisplayLabels(spanSource.span, t);
  const spanKindLabel = t(`traces.common.badges.${spanSource.spanKind}`);
  const normalizedSpanType = normalizeSpanType(spanSource.span.type);
  const SpanIcon = getSpanIcon(normalizedSpanType);
  const toolType = spanSource.span.value?.toolUse?.type;
  const isResponsesCustomTool = normalizeSpanType(toolType) === 'responses_custom_tool';

  const imageUrl = spanSource.span.value?.userImageUrl?.url || spanSource.span.value?.imageUrl?.url;
  const videoUrl = spanSource.span.value?.userVideoUrl?.url || spanSource.span.value?.videoUrl?.url;
  const summaryText = spanDisplay?.secondary;

  return (
    <div className='border-border/40 border-b'>
      <div
        className={cn(
          'hover:bg-accent/30 flex cursor-pointer items-center gap-3 px-3 py-2.5 transition-colors',
          isActive && 'bg-accent/40'
        )}
        style={{ paddingLeft: '48px' }}
        onClick={() => {
          onSelectSpan(spanSource.trace, spanSource.span, spanSource.spanKind);
        }}
      >
        <div className='flex h-4 w-4' />

        <div className='text-muted-foreground flex-shrink-0'>
          <SpanIcon className='text-muted-foreground h-4 w-4' />
        </div>

        <div className='flex min-w-0 flex-1 items-center gap-3'>
          {imageUrl && (
            <img
              src={imageUrl}
              alt=''
              className='h-8 w-8 flex-shrink-0 rounded border object-cover'
            />
          )}
          {!imageUrl && videoUrl && (
            <video
              src={videoUrl}
              className='h-8 w-8 flex-shrink-0 rounded border object-cover'
              muted
              preload='metadata'
            />
          )}
          <span className='truncate text-sm font-medium'>{spanDisplay?.primary || span.name}</span>
          {spanKindLabel && (
            <Badge variant='secondary' className='text-[10px] tracking-wide uppercase'>
              {spanKindLabel}
            </Badge>
          )}
          {isResponsesCustomTool && toolType && (
            <Badge variant='outline' className='text-[10px]'>
              {toolType}
            </Badge>
          )}
          <div className='text-muted-foreground ml-auto min-w-0 flex-1 text-right text-xs'>
            {summaryText && <span className='block truncate'>{summaryText}</span>}
          </div>
        </div>

        <div className='bg-muted/30 relative h-5 w-[180px] min-w-[180px] rounded'>
          <div
            className='absolute inset-y-0 rounded'
            style={{
              left: `${leftOffset}%`,
              width: `${width}%`,
              backgroundColor: span.color,
            }}
          />
        </div>
      </div>
    </div>
  );
}

// Helper function to find the earliest start time across all segments
function findEarliestStart(segment: Segment): number | null {
  const times: number[] = [];

  const collectTimes = (seg: Segment) => {
    const start = safeTime(seg.startTime);
    if (start != null) {
      times.push(start);
    }
    if (seg.children) {
      seg.children.forEach(collectTimes);
    }
  };

  collectTimes(segment);
  return times.length > 0 ? Math.min(...times) : null;
}

// Helper function to find the latest end time across all segments
function findLatestEnd(segment: Segment): number | null {
  const times: number[] = [];

  const collectTimes = (seg: Segment) => {
    const end = safeTime(seg.endTime);
    if (end != null) {
      times.push(end);
    }
    if (seg.children) {
      seg.children.forEach(collectTimes);
    }
  };

  collectTimes(segment);
  return times.length > 0 ? Math.max(...times) : null;
}

export function TraceFlatTimeline({ trace, onSelectSpan, selectedSpanId }: TraceTimelineProps) {
  const { t } = useTranslation();
  const [expandedSegments, setExpandedSegments] = useState<Set<string>>(new Set());
  const [allExpanded, setAllExpanded] = useState(true);
  const [selectedSpanTypes, setSelectedSpanTypes] = useState<Set<string>>(new Set());
  const initializedTraceIdRef = useRef<string | null>(null);

  const timelineData = useMemo(() => {
    const earliestStart = findEarliestStart(trace);
    const latestEnd = findLatestEnd(trace);
    if (earliestStart == null || latestEnd == null) {
      return null;
    }

    const rootNode = buildSegmentNode(trace, earliestStart);
    if (!rootNode) {
      return null;
    }

    const flatSegments = flattenSegments(rootNode, earliestStart);

    // Collect all unique span types
    const allSpanTypes = new Set<string>();
    flatSegments.forEach((seg) => {
      seg.spans.forEach((span) => {
        if (span.source.type === 'span') {
          const normalizedType = normalizeSpanType(span.source.span.type);
          allSpanTypes.add(normalizedType);
        }
      });
    });

    // Total duration is the sum of all segment durations (sequential layout)
    const totalDuration = Math.max(
      flatSegments.reduce((sum, seg) => sum + seg.segment.duration, 0),
      1
    );

    // Count total items (segments + spans)
    const totalItems = flatSegments.reduce((acc, seg) => acc + 1 + seg.spans.length, 0);

    let tokenSum = 0;
    let cachedTokenSum = 0;

    for (const seg of flatSegments) {
      const segmentTokens = seg.segment.metadata?.totalTokens;
      const segmentCachedTokens = seg.segment.metadata?.cachedTokens;
      if (typeof segmentTokens === 'number') {
        tokenSum += segmentTokens;
      }
      if (typeof segmentCachedTokens === 'number') {
        cachedTokenSum += segmentCachedTokens;
      }
    }

    const totalTokens = tokenSum > 0 ? tokenSum : null;
    const totalCachedTokens = cachedTokenSum > 0 ? cachedTokenSum : null;

    return {
      flatSegments,
      totalDuration: Math.max(totalDuration, 1),
      totalItems,
      totalTokens,
      totalCachedTokens,
      allSpanTypes: Array.from(allSpanTypes).sort(),
    };
  }, [trace]);

  useEffect(() => {
    if (!timelineData) return;
    if (initializedTraceIdRef.current === trace.id) return;

    initializedTraceIdRef.current = trace.id;

    const storageKey = `axonhub_traces_flat_timeline_expanded_segments_${trace.id}`;
    const expandableSegments = timelineData.flatSegments.filter((seg) => seg.spans.length > 0).map((seg) => seg.segment.id);
    const expandableSegmentSet = new Set(expandableSegments);

    let nextExpanded = new Set<string>();
    try {
      const raw = localStorage.getItem(storageKey);
      const parsed = raw ? JSON.parse(raw) : null;
      if (Array.isArray(parsed)) {
        nextExpanded = new Set(parsed.filter((id) => typeof id === 'string' && expandableSegmentSet.has(id)));
      }
    } catch (_error) {
      void _error;
    }

    if (nextExpanded.size === 0) {
      timelineData.flatSegments.slice(0, 10).forEach((seg) => {
        if (seg.spans.length > 0) {
          nextExpanded.add(seg.segment.id);
        }
      });
    }

    setExpandedSegments(nextExpanded);
    setAllExpanded(nextExpanded.size === expandableSegments.length && expandableSegments.length > 0);
  }, [timelineData, trace.id]);

  useEffect(() => {
    if (!timelineData) return;
    if (initializedTraceIdRef.current !== trace.id) return;

    const storageKey = `axonhub_traces_flat_timeline_expanded_segments_${trace.id}`;
    const next = Array.from(expandedSegments);
    try {
      localStorage.setItem(storageKey, JSON.stringify(next));
    } catch (_error) {
      void _error;
    }
  }, [expandedSegments, timelineData, trace.id]);

  useEffect(() => {
    if (!timelineData) return;
    const expandableCount = timelineData.flatSegments.filter((seg) => seg.spans.length > 0).length;
    setAllExpanded(expandableCount > 0 && expandedSegments.size === expandableCount);
  }, [expandedSegments, timelineData]);

  const handleToggleAll = () => {
    if (!timelineData) return;

    if (allExpanded) {
      // Collapse all
      setExpandedSegments(new Set());
    } else {
      // Expand all
      const allSegmentIds = new Set(timelineData.flatSegments.filter((seg) => seg.spans.length > 0).map((seg) => seg.segment.id));
      setExpandedSegments(allSegmentIds);
    }
  };

  const handleToggleSegment = (segmentId: string) => {
    setExpandedSegments((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(segmentId)) {
        newSet.delete(segmentId);
      } else {
        newSet.add(segmentId);
      }
      return newSet;
    });
  };

  const handleToggleSpanType = (spanType: string) => {
    setSelectedSpanTypes((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(spanType)) {
        newSet.delete(spanType);
      } else {
        newSet.add(spanType);
      }
      return newSet;
    });
  };

  const handleClearSpanTypeFilter = () => {
    setSelectedSpanTypes(new Set());
  };

  const handleSelectAllSpanTypes = () => {
    if (!timelineData) return;
    setSelectedSpanTypes(new Set(timelineData.allSpanTypes));
  };

  // Filter segments based on selected span types
  const filteredSegments = useMemo(() => {
    if (!timelineData || selectedSpanTypes.size === 0) {
      return timelineData?.flatSegments || [];
    }

    return timelineData.flatSegments
      .map((seg) => {
        // Filter spans by selected types
        const filteredSpans = seg.spans.filter((span) => {
          if (span.source.type === 'span') {
            const normalizedType = normalizeSpanType(span.source.span.type);
            return selectedSpanTypes.has(normalizedType);
          }
          return false;
        });

        return {
          ...seg,
          spans: filteredSpans,
        };
      })
      .filter((seg) => seg.spans.length > 0); // Only keep segments that have matching spans
  }, [timelineData, selectedSpanTypes]);

  if (!timelineData) {
    return (
      <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>{t('traces.timeline.emptyDescription')}</div>
    );
  }

  const { totalDuration, totalItems, totalTokens, totalCachedTokens, allSpanTypes } = timelineData;
  const activeFilterCount = selectedSpanTypes.size;

  return (
    <div className='flex h-full flex-col'>
      <div className='border-border/60 mb-4 border-b pb-4'>
        <div className='flex items-center justify-between'>
          <div className='flex items-center gap-3'>
            <h2 className='text-lg font-semibold'>{t('traces.timeline.title')}</h2>
            {/* <span className='text-muted-foreground text-sm'>{formatDuration(totalDuration)}</span> */}
            <span className='text-muted-foreground text-sm'>
              {t('traces.timeline.summary.duration', {
                value: formatDuration(totalDuration),
              })}
            </span>
            {totalTokens != null && (
              <span className='text-muted-foreground text-sm'>
                {t('traces.timeline.summary.tokenCount', {
                  value: formatNumber(totalTokens),
                })}
              </span>
            )}
            {totalCachedTokens != null && <span className='text-muted-foreground text-sm'>({formatNumber(totalCachedTokens)} cached)</span>}
          </div>
          <div className='flex items-center gap-3'>
            <div className='text-muted-foreground text-sm'>{t('traces.timeline.itemsCount', { count: totalItems })}</div>

            {/* Span Type Filter */}
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className={cn(
                    'h-7 gap-1.5 px-2 text-xs',
                    activeFilterCount > 0 && 'text-primary'
                  )}
                >
                  <Filter className='h-3.5 w-3.5' />
                  {activeFilterCount > 0 ? (
                    <span>{activeFilterCount}</span>
                  ) : (
                    <span>{t('traces.timeline.filter.spanType')}</span>
                  )}
                </Button>
              </PopoverTrigger>
              <PopoverContent className='w-64 p-0' align='end'>
                <div className='border-border/60 flex items-center justify-between border-b px-3 py-2'>
                  <span className='text-sm font-medium'>{t('traces.timeline.filter.spanType')}</span>
                  <div className='flex gap-2'>
                    {activeFilterCount > 0 && (
                      <Button
                        variant='ghost'
                        size='sm'
                        className='h-7 px-2 text-xs'
                        onClick={handleClearSpanTypeFilter}
                      >
                        {t('traces.timeline.filter.clear')}
                      </Button>
                    )}
                    <Button
                      variant='ghost'
                      size='sm'
                      className='h-7 px-2 text-xs'
                      onClick={handleSelectAllSpanTypes}
                    >
                      {t('traces.timeline.filter.selectAll')}
                    </Button>
                  </div>
                </div>
                <div className='max-h-[320px] overflow-y-auto p-2'>
                  {allSpanTypes.length > 0 ? (
                    <div className='space-y-1'>
                      {allSpanTypes.map((spanType) => {
                        const SpanIcon = getSpanIcon(spanType);
                        const isChecked = selectedSpanTypes.has(spanType);
                        return (
                          <label
                            key={spanType}
                            className='hover:bg-accent flex cursor-pointer items-center gap-2 rounded-md px-2 py-2 transition-colors'
                          >
                            <Checkbox
                              checked={isChecked}
                              onCheckedChange={() => handleToggleSpanType(spanType)}
                            />
                            <SpanIcon className='text-muted-foreground h-4 w-4 flex-shrink-0' />
                            <span className='flex-1 text-sm'>
                              {t(`traces.timeline.spanTypes.${spanType}`, spanType)}
                            </span>
                          </label>
                        );
                      })}
                    </div>
                  ) : (
                    <div className='text-muted-foreground px-2 py-4 text-center text-sm'>
                      {t('traces.timeline.filter.noSpanTypes')}
                    </div>
                  )}
                </div>
              </PopoverContent>
            </Popover>

            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={handleToggleAll}
              className='bg-primary/10 hover:bg-primary/20 h-8 w-8 rounded-lg p-0'
              title={allExpanded ? t('traces.timeline.collapseAll') : t('traces.timeline.expandAll')}
            >
              <ChevronsDownUp className={cn('text-primary h-4 w-4 transition-transform', !allExpanded && 'rotate-180')} />
            </Button>
          </div>
        </div>
      </div>
      <div className='border-border/40 bg-card/50 flex-1 overflow-auto rounded-lg border'>
        {filteredSegments.map((flatSegment) => (
          <SegmentRow
            key={flatSegment.segment.id}
            segment={flatSegment.segment}
            spans={flatSegment.spans}
            totalDuration={totalDuration}
            sequentialOffset={flatSegment.sequentialOffset}
            onSelectSpan={onSelectSpan}
            selectedSpanId={selectedSpanId}
            isExpanded={expandedSegments.has(flatSegment.segment.id)}
            onToggleExpand={() => handleToggleSegment(flatSegment.segment.id)}
          />
        ))}
      </div>
    </div>
  );
}
