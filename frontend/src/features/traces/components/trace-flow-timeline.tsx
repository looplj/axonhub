'use client';

import { useMemo } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  Handle,
  Position,
  type Node,
  type Edge,
  type NodeTypes,
  type NodeProps,
  ReactFlowProvider,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Workflow, GitBranch, ExternalLink } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { buildGUID, cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { formatDuration } from '../../../utils/format-duration';
import type { Segment, Span } from '../data/schema';
import { getSpanDisplayLabels, normalizeSpanType } from '../utils/span-display';
import { getSpanIcon } from './constant';

type SpanKind = 'request' | 'response';

export interface TraceFlowTimelineProps {
  trace: Segment;
  onSelectSpan: (trace: Segment, span: Span, type: SpanKind) => void;
  selectedSpanId?: string;
  isFullscreen?: boolean;
}

// ─── Layout constants ─────────────────────────────────────────────────────────

const NODE_WIDTH = 280;
const NODE_HEADER_HEIGHT = 38; // header row
const SPAN_ROW_HEIGHT = 28;   // each span row
const NODE_FOOTER_HEIGHT = 26; // token stats row
const H_GAP = 40;
const V_GAP = 50;

/** Compute the rendered height for a segment node */
function computeNodeHeight(segment: Segment): number {
  const spanCount = (segment.requestSpans?.length || 0) + (segment.responseSpans?.length || 0);
  const hasTokens = segment.metadata?.inputTokens != null || segment.metadata?.outputTokens != null;
  return NODE_HEADER_HEIGHT + spanCount * SPAN_ROW_HEIGHT + (hasTokens ? NODE_FOOTER_HEIGHT : 8);
}

// ─── Color ────────────────────────────────────────────────────────────────────

function hashStringToHue(value: string): number {
  let hash = 0;
  for (let i = 0; i < value.length; i += 1) {
    hash = (hash * 31 + value.charCodeAt(i)) % 360;
  }
  return (hash + 360) % 360;
}

function getSegmentBorderColor(segmentId: string): string {
  const hue = hashStringToHue(segmentId);
  return `hsla(${hue}, 60%, 55%, 0.7)`;
}

// ─── Tree layout (variable height) ───────────────────────────────────────────

interface LayoutNode {
  id: string;
  segment: Segment;
  children: LayoutNode[];
  x: number;
  y: number;
  height: number;
  subtreeWidth: number;
}

function buildLayoutTree(segment: Segment): LayoutNode {
  const children: LayoutNode[] = (segment.children || []).map(buildLayoutTree);
  return {
    id: String(segment.id),
    segment,
    children,
    x: 0,
    y: 0,
    height: computeNodeHeight(segment),
    subtreeWidth: 0,
  };
}

function computeSubtreeWidth(node: LayoutNode): number {
  if (node.children.length === 0) {
    node.subtreeWidth = NODE_WIDTH;
    return NODE_WIDTH;
  }
  let total = 0;
  for (const child of node.children) total += computeSubtreeWidth(child);
  total += (node.children.length - 1) * H_GAP;
  node.subtreeWidth = Math.max(NODE_WIDTH, total);
  return node.subtreeWidth;
}

function positionNodes(node: LayoutNode, x: number, y: number) {
  node.x = x + node.subtreeWidth / 2 - NODE_WIDTH / 2;
  node.y = y;

  if (node.children.length === 0) return;

  let totalChildrenWidth = 0;
  for (const child of node.children) totalChildrenWidth += child.subtreeWidth;
  totalChildrenWidth += (node.children.length - 1) * H_GAP;

  let childX = x + (node.subtreeWidth - totalChildrenWidth) / 2;
  const childY = y + node.height + V_GAP;

  for (const child of node.children) {
    positionNodes(child, childX, childY);
    childX += child.subtreeWidth + H_GAP;
  }
}

// ─── Span info for node data ─────────────────────────────────────────────────

interface SpanInfo {
  span: Span;
  kind: SpanKind;
}

function getOrderedSpans(segment: Segment): SpanInfo[] {
  const spans: SpanInfo[] = [];
  for (const s of segment.requestSpans || []) spans.push({ span: s, kind: 'request' });
  for (const s of segment.responseSpans || []) spans.push({ span: s, kind: 'response' });
  return spans;
}

// ─── Convert to ReactFlow elements ───────────────────────────────────────────

interface SegmentNodeData {
  segment: Segment;
  borderColor: string;
  hasChildren: boolean;
  spans: SpanInfo[];
  onSelectSpan: (segment: Segment, span: Span, kind: SpanKind) => void;
  selectedSpanId?: string;
  [key: string]: unknown;
}

type SegmentFlowNode = Node<SegmentNodeData, 'segment'>;

function collectFlowElements(
  root: LayoutNode,
  onSelectSpan: (segment: Segment, span: Span, kind: SpanKind) => void,
  selectedSpanId?: string
): { nodes: SegmentFlowNode[]; edges: Edge[] } {
  const nodes: SegmentFlowNode[] = [];
  const edges: Edge[] = [];

  const traverse = (node: LayoutNode) => {
    const seg = node.segment;
    const segId = String(seg.id);

    nodes.push({
      id: segId,
      type: 'segment',
      position: { x: node.x, y: node.y },
      data: {
        segment: seg,
        borderColor: getSegmentBorderColor(segId),
        hasChildren: node.children.length > 0,
        spans: getOrderedSpans(seg),
        onSelectSpan,
        selectedSpanId,
      },
    });

    for (const child of node.children) {
      edges.push({
        id: `e-${segId}-${String(child.segment.id)}`,
        source: segId,
        target: String(child.segment.id),
        type: 'smoothstep',
        style: { stroke: 'var(--border)', strokeWidth: 1.5 },
      });
      traverse(child);
    }
  };

  traverse(root);
  return { nodes, edges };
}

// ─── Custom Segment Node ──────────────────────────────────────────────────────

function SegmentNode({ data, selected }: NodeProps<SegmentFlowNode>) {
  const { t } = useTranslation();
  const seg = data.segment;

  const handleViewRequest = (e: React.MouseEvent) => {
    e.stopPropagation();
    const url = `/project/requests/${encodeURIComponent(buildGUID('Request', seg.id))}`;
    window.open(url, '_blank', 'noopener,noreferrer');
  };

  const inputTokens = seg.metadata?.inputTokens;
  const outputTokens = seg.metadata?.outputTokens;
  const hasTokens = inputTokens != null || outputTokens != null;

  return (
    <>
      <Handle type='target' position={Position.Top} className='!bg-border !h-2 !w-2' />
      <div
        className={cn(
          'bg-card border-border/60 rounded-lg border shadow-sm transition-shadow',
          selected && 'ring-primary/40 ring-2'
        )}
        style={{ borderLeftWidth: 3, borderLeftColor: data.borderColor, width: NODE_WIDTH }}
      >
        {/* Header: model + duration + link */}
        <div className='flex items-center gap-2 px-3 py-2'>
          {data.hasChildren ? (
            <GitBranch className='text-primary h-3.5 w-3.5 flex-shrink-0' />
          ) : (
            <Workflow className='text-primary h-3.5 w-3.5 flex-shrink-0' />
          )}
          <Badge variant='secondary' className='max-w-[120px] truncate text-[11px] font-medium'>
            {seg.model}
          </Badge>
          <span className='text-muted-foreground text-[10px]'>{formatDuration(seg.duration)}</span>
          <div className='flex-1' />
          <Button variant='ghost' size='sm' className='h-5 w-5 p-0' onClick={handleViewRequest}>
            <ExternalLink className='h-3 w-3' />
          </Button>
        </div>

        {/* Span list */}
        {data.spans.length > 0 && (
          <div className='border-border/40 border-t'>
            {data.spans.map((info) => {
              const spanDisplay = getSpanDisplayLabels(info.span, t);
              const normalizedType = normalizeSpanType(info.span.type);
              const SpanIcon = getSpanIcon(normalizedType);
              const isActive = data.selectedSpanId === info.span.id;

              return (
                <div
                  key={info.span.id}
                  className={cn(
                    'hover:bg-accent/40 flex cursor-pointer items-center gap-2 px-3 transition-colors',
                    isActive && 'bg-accent/50'
                  )}
                  style={{ height: SPAN_ROW_HEIGHT }}
                  onClick={(e) => {
                    e.stopPropagation();
                    data.onSelectSpan(seg, info.span, info.kind);
                  }}
                >
                  <SpanIcon className='text-muted-foreground h-3 w-3 flex-shrink-0' />
                  <span className='truncate text-[11px] font-medium'>
                    {spanDisplay?.primary || info.span.type}
                  </span>
                  <Badge
                    variant='outline'
                    className={cn(
                      'ml-auto h-4 px-1 text-[9px]',
                      info.kind === 'request' ? 'text-blue-500 border-blue-500/30' : 'text-green-500 border-green-500/30'
                    )}
                  >
                    {info.kind === 'request' ? 'REQ' : 'RES'}
                  </Badge>
                </div>
              );
            })}
          </div>
        )}

        {/* Footer: token stats */}
        {hasTokens && (
          <div className='border-border/40 text-muted-foreground flex items-center gap-3 border-t px-3 text-[10px]' style={{ height: NODE_FOOTER_HEIGHT }}>
            {inputTokens != null && <span>↑ {inputTokens.toLocaleString()}</span>}
            {outputTokens != null && <span>↓ {outputTokens.toLocaleString()}</span>}
          </div>
        )}
      </div>
      <Handle type='source' position={Position.Bottom} className='!bg-border !h-2 !w-2' />
    </>
  );
}

const nodeTypes: NodeTypes = { segment: SegmentNode };

// ─── Inner component (inside ReactFlowProvider) ──────────────────────────────

function TraceFlowTimelineInner({ trace, onSelectSpan, selectedSpanId, isFullscreen }: TraceFlowTimelineProps) {
  const { t } = useTranslation();

  const { nodes, edges } = useMemo(() => {
    const layoutRoot = buildLayoutTree(trace);
    computeSubtreeWidth(layoutRoot);
    positionNodes(layoutRoot, 0, 0);
    return collectFlowElements(layoutRoot, onSelectSpan, selectedSpanId);
  }, [trace, onSelectSpan, selectedSpanId]);

  return (
    <div className='flex h-full flex-col'>
      <div className='border-border/60 mb-2 border-b pb-2'>
        <h2 className='text-lg font-semibold'>{t('traces.flowTimeline.title')}</h2>
      </div>
      <div
        className={cn(
          'border-border/40 bg-card/30 flex-1 overflow-hidden rounded-lg border',
          isFullscreen && 'rounded-none border-0'
        )}
        style={{ minHeight: isFullscreen ? 'calc(100vh - 220px)' : 400 }}
      >
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.3 }}
          panOnScroll
          zoomOnDoubleClick={false}
          minZoom={0.2}
          maxZoom={2}
          proOptions={{ hideAttribution: true }}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable
        >
          <Background gap={16} size={1} />
          <Controls showInteractive={false} />
        </ReactFlow>
      </div>
    </div>
  );
}

// ─── Export ───────────────────────────────────────────────────────────────────

export function TraceFlowTimeline(props: TraceFlowTimelineProps) {
  return (
    <ReactFlowProvider>
      <TraceFlowTimelineInner {...props} />
    </ReactFlowProvider>
  );
}
