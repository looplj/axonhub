'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronRight, ChevronDown, Workflow, ChevronsDownUp, ExternalLink, Filter } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { AnimatePresence, motion } from 'framer-motion';
import { buildGUID, cn } from '@/lib/utils';
import { formatNumber } from '@/utils/format-number';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { formatDuration } from '../../../utils/format-duration';
import type { Segment, Span } from '../data/schema';
import { normalizeSpanType, getSpanDisplayLabels } from '../utils/span-display';
import { getSpanIcon } from './constant';

type SpanKind = 'request' | 'response';

export interface TraceTreeTimelineProps {
  trace: Segment;
  level?: number;
  onSelectSpan: (trace: Segment, span: Span, type: SpanKind) => void;
  selectedSpanId?: string;
}

interface TreeNode {
  id: string;
  segment: Segment;
  level: number;
  children: TreeNode[];
  spans: { span: Span; kind: SpanKind }[];
}

// 使用普通 Map 缓存，并限制缓存大小以防止内存占用过高
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

function getSegmentTimelineColor(segmentId: string): string {
  const hue = hashStringToHue(segmentId);
  return `hsla(${hue}, 70%, 56%, 0.75)`;
}

function getSpanTimelineColor(segmentId: string, kind: SpanKind): string {
  const hue = hashStringToHue(segmentId);
  const lightness = kind === 'request' ? 64 : 70;
  const alpha = kind === 'request' ? 0.62 : 0.55;
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

function calculateBarPosition(startRatio: number, widthRatio: number): { left: number; width: number } {
  const leftOffset = Math.min(Math.max(startRatio * 100, 0), 100);
  const maxAvailableWidth = Math.max(100 - leftOffset, 0);

  let width = Math.min(Math.max(widthRatio * 100, 0), maxAvailableWidth);
  if (widthRatio > 0 && width < 0.5) {
    width = Math.min(0.5, maxAvailableWidth);
  }

  return { left: leftOffset, width };
}

function buildTree(segment: Segment, level = 0): TreeNode {
  const spans: { span: Span; kind: SpanKind }[] = [
    ...(segment.requestSpans || []).map((s: Span) => ({ span: s, kind: 'request' as SpanKind })),
    ...(segment.responseSpans || []).map((s: Span) => ({ span: s, kind: 'response' as SpanKind })),
  ];

  // Sort spans by start time
  spans.sort((a, b) => {
    const aTime = safeTime(a.span.startTime) || 0;
    const bTime = safeTime(b.span.startTime) || 0;
    return aTime - bTime;
  });

  return {
    id: String(segment.id),
    segment,
    level,
    spans,
    children: (segment.children || []).map((child: Segment) => buildTree(child, level + 1)),
  };
}

// Find earliest and latest times across all segments and spans
function findTimeRange(node: TreeNode): { start: number; end: number } | null {
  const times: { start: number; end: number }[] = [];

  const collect = (n: TreeNode) => {
    const segStart = safeTime(n.segment.startTime);
    const segEnd = safeTime(n.segment.endTime);
    if (segStart != null && segEnd != null) {
      times.push({ start: segStart, end: segEnd });
    }

    n.spans.forEach(({ span }) => {
      const spanStart = safeTime(span.startTime);
      const spanEnd = safeTime(span.endTime);
      if (spanStart != null && spanEnd != null) {
        times.push({ start: spanStart, end: spanEnd });
      }
    });

    n.children.forEach(collect);
  };

  collect(node);

  if (times.length === 0) return null;

  return {
    start: Math.min(...times.map((t) => t.start)),
    end: Math.max(...times.map((t) => t.end)),
  };
}

interface SegmentRowProps {
  node: TreeNode;
  totalDuration: number;
  timeRange: { start: number; end: number };
  selectedSpanId?: string;
  onSelectSpan: (trace: Segment, span: Span, kind: SpanKind) => void;
  expandedNodes: Set<string>;
  onToggleNode: (nodeId: string) => void;
}

function SegmentRow({
  node,
  totalDuration,
  timeRange,
  selectedSpanId,
  onSelectSpan,
  expandedNodes,
  onToggleNode,
}: SegmentRowProps) {
  const { t } = useTranslation();
  const { segment, level, spans, children } = node;
  const hasContent = spans.length > 0 || children.length > 0;
  const isExpanded = expandedNodes.has(node.id);

  const handleViewRequest = (e: React.MouseEvent) => {
    e.stopPropagation();
    const url = `/project/requests/${encodeURIComponent(buildGUID('Request', String(segment.id)))}`;
    window.open(url, '_blank', 'noopener,noreferrer');
  };

  // Calculate timeline bar position
  const segStart = safeTime(segment.startTime);
  const segEnd = safeTime(segment.endTime);
  const segDuration = segment.duration || 0;

  let leftOffset = 0;
  let width = 0;

  if (segStart != null && segEnd != null && totalDuration > 0) {
    const startOffset = Math.max(segStart - timeRange.start, 0);
    const leftOffsetRatio = startOffset / totalDuration;
    const widthRatio = segDuration / totalDuration;
    const pos = calculateBarPosition(leftOffsetRatio, widthRatio);
    leftOffset = pos.left;
    width = pos.width;
  }

  const color = getSegmentTimelineColor(String(segment.id));
  const indentPadding = level * 16;

  return (
    <>
      {/* Segment Row */}
      <div className='border-border/40 border-b'>
        <div
          className='flex cursor-default items-center gap-3 px-3 py-2.5 transition-colors hover:bg-accent/30'
          style={{ paddingLeft: `${12 + indentPadding}px` }}
        >
          <button
            onClick={(event) => {
              event.stopPropagation();
              if (hasContent) {
                onToggleNode(node.id);
              }
            }}
            className={cn(
              'flex h-4 w-4 items-center justify-center rounded transition-colors',
              hasContent ? 'hover:bg-accent text-muted-foreground' : 'opacity-0'
            )}
            aria-label={
              hasContent ? (isExpanded ? t('traces.timeline.aria.collapseRow') : t('traces.timeline.aria.expandRow')) : undefined
            }
          >
            {hasContent && (isExpanded ? <ChevronDown className='h-3 w-3' /> : <ChevronRight className='h-3 w-3' />)}
          </button>

          <div className='text-muted-foreground flex-shrink-0'>
            <Workflow className='text-primary h-4 w-4' />
          </div>

          <div className='flex min-w-0 flex-1 items-center gap-3'>
            <div className='flex min-w-0 flex-1 items-center gap-2'>
              <Badge variant='secondary' className='text-xs font-medium'>
                {segment.model}
              </Badge>
              <span className='text-muted-foreground text-[11px]'>
                {t('traces.timeline.summary.duration', {
                  value: formatDuration(segDuration),
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
                backgroundColor: color,
              }}
            />
          </div>
        </div>
      </div>

      {/* Spans */}
      <AnimatePresence>
        {isExpanded && spans.length > 0 && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.2, ease: 'easeInOut' }}
          >
            {spans.map(({ span, kind }) => (
              <SpanRow
                key={span.id}
                span={span}
                kind={kind}
                segment={segment}
                totalDuration={totalDuration}
                timeRange={timeRange}
                onSelectSpan={onSelectSpan}
                selectedSpanId={selectedSpanId}
                level={level}
              />
            ))}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Children Segments */}
      <AnimatePresence>
        {isExpanded &&
          children.map((child) => (
            <motion.div
              key={child.id}
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: 'auto', opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
              transition={{ duration: 0.2, ease: 'easeInOut' }}
            >
              <SegmentRow
                node={child}
                totalDuration={totalDuration}
                timeRange={timeRange}
                selectedSpanId={selectedSpanId}
                onSelectSpan={onSelectSpan}
                expandedNodes={expandedNodes}
                onToggleNode={onToggleNode}
              />
            </motion.div>
          ))}
      </AnimatePresence>
    </>
  );
}

interface SpanRowProps {
  span: Span;
  kind: SpanKind;
  segment: Segment;
  totalDuration: number;
  timeRange: { start: number; end: number };
  selectedSpanId?: string;
  onSelectSpan: (trace: Segment, span: Span, kind: SpanKind) => void;
  level: number;
}

function SpanRow({ span, kind, segment, totalDuration, timeRange, selectedSpanId, onSelectSpan, level }: SpanRowProps) {
  const { t } = useTranslation();
  const isActive = selectedSpanId === span.id;

  const spanStart = safeTime(span.startTime);
  const spanEnd = safeTime(span.endTime);
  const spanDuration = spanStart != null && spanEnd != null ? Math.max(spanEnd - spanStart, 0) : 0;

  let leftOffset = 0;
  let width = 0;

  if (spanStart != null && spanEnd != null && totalDuration > 0) {
    const startOffset = Math.max(spanStart - timeRange.start, 0);
    const leftOffsetRatio = startOffset / totalDuration;
    const widthRatio = spanDuration / totalDuration;
    const pos = calculateBarPosition(leftOffsetRatio, widthRatio);
    leftOffset = pos.left;
    width = pos.width;
  }

  const color = getSpanTimelineColor(String(segment.id), kind);
  const spanDisplay = getSpanDisplayLabels(span, t);
  const normalizedSpanType = normalizeSpanType(span.type);
  const SpanIcon = getSpanIcon(normalizedSpanType);
  const spanKindLabel = t(`traces.common.badges.${kind}`);
  const toolType = span.value?.toolUse?.type;
  const isResponsesCustomTool = toolType === 'responses_custom_tool';

  const imageUrl = span.value?.userImageUrl?.url || span.value?.imageUrl?.url;
  const videoUrl = span.value?.userVideoUrl?.url || span.value?.videoUrl?.url;
  const summaryText = spanDisplay?.secondary;

  const indentPadding = (level + 1) * 16 + 28; // Extra 28px for span indentation

  return (
    <div className='border-border/40 border-b'>
      <div
        className={cn(
          'hover:bg-accent/30 flex cursor-pointer items-center gap-3 px-3 py-2 transition-colors',
          isActive && 'bg-accent/40'
        )}
        style={{ paddingLeft: `${indentPadding}px` }}
        onClick={() => onSelectSpan(segment, span, kind)}
      >
        <div className='flex h-4 w-4' />

        <div className='text-muted-foreground flex-shrink-0'>
          <SpanIcon className='text-muted-foreground h-4 w-4' />
        </div>

        <div className='flex min-w-0 flex-1 items-center gap-3'>
          {imageUrl && (
            <img src={imageUrl} alt='' className='h-8 w-8 flex-shrink-0 rounded border object-cover' />
          )}
          {!imageUrl && videoUrl && (
            <video src={videoUrl} className='h-8 w-8 flex-shrink-0 rounded border object-cover' muted preload='metadata' />
          )}
          <span className='truncate text-sm font-medium'>{spanDisplay?.primary || span.type}</span>
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
              backgroundColor: color,
            }}
          />
        </div>
      </div>
    </div>
  );
}

// Collect all nodes for expansion management
function collectAllNodes(node: TreeNode, result: TreeNode[] = []) {
  result.push(node);
  node.children.forEach((child) => collectAllNodes(child, result));
  return result;
}

// Collect all span types from the tree
function collectSpanTypes(node: TreeNode, result: Set<string> = new Set()) {
  node.spans.forEach(({ span }) => {
    result.add(normalizeSpanType(span.type));
  });
  node.children.forEach((child) => collectSpanTypes(child, result));
  return result;
}

// Filter tree by span types
function filterTreeBySpanTypes(node: TreeNode, selectedTypes: Set<string>): TreeNode | null {
  const filteredSpans = node.spans.filter(({ span }) => selectedTypes.has(normalizeSpanType(span.type)));

  const filteredChildren = node.children
    .map((child) => filterTreeBySpanTypes(child, selectedTypes))
    .filter(Boolean) as TreeNode[];

  // Keep node if it has matching spans or any child has matching content
  if (filteredSpans.length > 0 || filteredChildren.length > 0 || selectedTypes.size === 0) {
    return {
      ...node,
      spans: filteredSpans,
      children: filteredChildren,
    };
  }

  return null;
}

export function TraceTreeTimeline(props: TraceTreeTimelineProps) {
  const { trace, onSelectSpan, selectedSpanId } = props;
  const { t } = useTranslation();

  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());
  const [allExpanded, setAllExpanded] = useState(true);
  const [selectedSpanTypes, setSelectedSpanTypes] = useState<Set<string>>(new Set());
  const initializedTraceIdRef = useRef<string | null>(null);

  const treeData = useMemo(() => buildTree(trace), [trace]);

  const timeRange = useMemo(() => findTimeRange(treeData), [treeData]);

  const totalDuration = useMemo(() => {
    if (!timeRange) return 0;
    return Math.max(timeRange.end - timeRange.start, 1);
  }, [timeRange]);

  const allSpanTypes = useMemo(() => Array.from(collectSpanTypes(treeData)).sort(), [treeData]);

  const filteredTree = useMemo(() => {
    if (selectedSpanTypes.size === 0) return treeData;
    return filterTreeBySpanTypes(treeData, selectedSpanTypes) || treeData;
  }, [treeData, selectedSpanTypes]);

  // Calculate total stats
  const stats = useMemo(() => {
    const nodes = collectAllNodes(treeData);
    let totalTokens = 0;
    let cachedTokens = 0;
    let itemCount = 0;

    nodes.forEach((node) => {
      itemCount += 1 + node.spans.length;
      const tokens = node.segment.metadata?.totalTokens;
      const cached = node.segment.metadata?.cachedTokens;
      if (typeof tokens === 'number') totalTokens += tokens;
      if (typeof cached === 'number') cachedTokens += cached;
    });

    return {
      totalTokens: totalTokens > 0 ? totalTokens : null,
      cachedTokens: cachedTokens > 0 ? cachedTokens : null,
      itemCount,
    };
  }, [treeData]);

  // Initialize expanded state from localStorage
  useEffect(() => {
    if (initializedTraceIdRef.current === trace.id) return;
    initializedTraceIdRef.current = trace.id;

    const storageKey = `axonhub_traces_tree_expanded_nodes_${trace.id}`;
    const allNodes = collectAllNodes(treeData);
    const expandableNodes = allNodes.filter((n) => n.spans.length > 0 || n.children.length > 0).map((n) => n.id);
    const expandableSet = new Set(expandableNodes);

    let nextExpanded = new Set<string>();
    try {
      const raw = localStorage.getItem(storageKey);
      const parsed = raw ? JSON.parse(raw) : null;
      if (Array.isArray(parsed)) {
        nextExpanded = new Set(parsed.filter((id) => typeof id === 'string' && expandableSet.has(id)));
      }
    } catch {
      // ignore
    }

    if (nextExpanded.size === 0) {
      // Default expand first few nodes
      expandableNodes.slice(0, 5).forEach((id) => nextExpanded.add(id));
    }

    setExpandedNodes(nextExpanded);
    setAllExpanded(nextExpanded.size === expandableNodes.length && expandableNodes.length > 0);
  }, [treeData, trace.id]);

  // Persist expanded state
  useEffect(() => {
    if (initializedTraceIdRef.current !== trace.id) return;

    const storageKey = `axonhub_traces_tree_expanded_nodes_${trace.id}`;
    try {
      localStorage.setItem(storageKey, JSON.stringify(Array.from(expandedNodes)));
    } catch {
      // ignore
    }
  }, [expandedNodes, trace.id]);

  // Update allExpanded when expandedNodes changes
  useEffect(() => {
    const allNodes = collectAllNodes(treeData);
    const expandableNodes = allNodes.filter((n) => n.spans.length > 0 || n.children.length > 0);
    setAllExpanded(expandableNodes.length > 0 && expandableNodes.every((n) => expandedNodes.has(n.id)));
  }, [expandedNodes, treeData]);

  const handleToggleNode = (nodeId: string) => {
    setExpandedNodes((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(nodeId)) {
        newSet.delete(nodeId);
      } else {
        newSet.add(nodeId);
      }
      return newSet;
    });
  };

  const handleToggleAll = () => {
    const allNodes = collectAllNodes(treeData);
    const expandableNodes = allNodes.filter((n) => n.spans.length > 0 || n.children.length > 0);

    if (allExpanded) {
      setExpandedNodes(new Set());
    } else {
      setExpandedNodes(new Set(expandableNodes.map((n) => n.id)));
    }
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
    setSelectedSpanTypes(new Set(allSpanTypes));
  };

  const activeFilterCount = selectedSpanTypes.size;

  if (!timeRange) {
    return (
      <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
        {t('traces.timeline.emptyDescription')}
      </div>
    );
  }

  return (
    <div className='flex h-full flex-col'>
      {/* Header */}
      <div className='border-border/60 mb-4 border-b pb-4'>
        <div className='flex items-center justify-between'>
          <div className='flex items-center gap-3'>
            <h2 className='text-lg font-semibold'>{t('traces.treeTimeline.title')}</h2>
            <span className='text-muted-foreground text-sm'>
              {t('traces.timeline.summary.duration', {
                value: formatDuration(totalDuration),
              })}
            </span>
            {stats.totalTokens != null && (
              <span className='text-muted-foreground text-sm'>
                {t('traces.timeline.summary.tokenCount', {
                  value: formatNumber(stats.totalTokens),
                })}
              </span>
            )}
            {stats.cachedTokens != null && (
              <span className='text-muted-foreground text-sm'>({formatNumber(stats.cachedTokens)} cached)</span>
            )}
          </div>
          <div className='flex items-center gap-3'>
            <div className='text-muted-foreground text-sm'>{t('traces.timeline.itemsCount', { count: stats.itemCount })}</div>

            {/* Span Type Filter */}
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className={cn('h-7 gap-1.5 px-2 text-xs', activeFilterCount > 0 && 'text-primary')}
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
                      <Button variant='ghost' size='sm' className='h-7 px-2 text-xs' onClick={handleClearSpanTypeFilter}>
                        {t('traces.timeline.filter.clear')}
                      </Button>
                    )}
                    <Button variant='ghost' size='sm' className='h-7 px-2 text-xs' onClick={handleSelectAllSpanTypes}>
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
                            <Checkbox checked={isChecked} onCheckedChange={() => handleToggleSpanType(spanType)} />
                            <SpanIcon className='text-muted-foreground h-4 w-4 flex-shrink-0' />
                            <span className='flex-1 text-sm'>{t(`traces.timeline.spanTypes.${spanType}`, spanType)}</span>
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

      {/* Tree Content */}
      <div className='border-border/40 bg-card/50 flex-1 overflow-auto rounded-lg border'>
        <SegmentRow
          node={filteredTree}
          totalDuration={totalDuration}
          timeRange={timeRange}
          selectedSpanId={selectedSpanId}
          onSelectSpan={onSelectSpan}
          expandedNodes={expandedNodes}
          onToggleNode={handleToggleNode}
        />
      </div>
    </div>
  );
}
