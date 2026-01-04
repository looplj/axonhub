import { useState } from 'react';
import { format } from 'date-fns';
import { ChevronRight, ChevronDown, Clock, Zap } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Segment, Span } from '../data/schema';
import { normalizeSpanType } from '../utils/span-display';
import { getSpanIcon } from './constant';

interface TraceTreeViewProps {
  trace: Segment;
  level?: number;
  onSpanSelect?: (trace: Segment, span: Span, type: 'request' | 'response') => void;
  selectedSpanId?: string;
}

function SpanItem({
  span,
  type,
  onSelect,
  isActive,
}: {
  span: Span;
  type: 'request' | 'response';
  onSelect?: () => void;
  isActive?: boolean;
}) {
  const { t } = useTranslation();
  const normalizedSpanType = normalizeSpanType(span.type);
  const SpanIcon = getSpanIcon(normalizedSpanType);

  const duration =
    span.startTime && span.endTime
      ? `${((new Date(span.endTime).getTime() - new Date(span.startTime).getTime()) / 1000).toFixed(3)}s`
      : null;

  return (
    <button
      type='button'
      onClick={(event) => {
        event.stopPropagation();
        onSelect?.();
      }}
      className={cn(
        'w-full rounded-lg border px-4 py-3 text-left transition-colors',
        'bg-muted/20 hover:bg-muted/40 hover:border-primary/40 flex flex-col gap-1',
        isActive && 'border-primary bg-primary/10 shadow-sm'
      )}
    >
      <div className='flex items-center justify-between gap-4'>
        <div className='flex min-w-0 items-center gap-2'>
          <SpanIcon className='text-muted-foreground h-4 w-4' />
          <span className='truncate text-sm font-medium'>{span.type}</span>
          <Badge variant='secondary' className='text-xs capitalize'>
            {t(`traces.common.badges.${type}`)}
          </Badge>
        </div>
        <ChevronRight className='text-muted-foreground h-4 w-4 flex-shrink-0' />
      </div>
      {duration && <span className='text-muted-foreground text-xs'>{duration}</span>}
    </button>
  );
}

export function TraceTreeView({ trace, level = 0, onSpanSelect, selectedSpanId }: TraceTreeViewProps) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(level === 0);

  const duration = trace.duration ? `${(trace.duration / 1000).toFixed(3)}s` : '0s';
  const hasChildren = trace.children && trace.children.length > 0;
  const hasSpans = (trace.requestSpans && trace.requestSpans.length > 0) || (trace.responseSpans && trace.responseSpans.length > 0);

  const tokenRows = [
    trace.metadata?.inputTokens != null && {
      label: t('traces.detail.inputTokensLabel'),
      value: trace.metadata.inputTokens.toLocaleString(),
    },
    trace.metadata?.outputTokens != null && {
      label: t('traces.detail.outputTokensLabel'),
      value: trace.metadata.outputTokens.toLocaleString(),
    },
    trace.metadata?.totalTokens != null && {
      label: t('traces.detail.totalTokensLabel'),
      value: trace.metadata.totalTokens.toLocaleString(),
    },
    trace.metadata?.cachedTokens != null && {
      label: t('traces.detail.cachedTokensLabel'),
      value: trace.metadata.cachedTokens.toLocaleString(),
    },
  ].filter(Boolean) as { label: string; value: string }[];

  return (
    <div className={cn('space-y-2', level > 0 && 'ml-6')}>
      <Card className='border-l-primary/70 border-l-4 transition-shadow hover:shadow-md'>
        <CardContent className='p-4'>
          <div
            className='flex items-start gap-3'
            onClick={(e) => {
              e.stopPropagation();
              setExpanded(!expanded);
            }}
          >
            {(hasChildren || hasSpans) &&
              (expanded ? (
                <ChevronDown className='mt-0.5 h-5 w-5 flex-shrink-0' />
              ) : (
                <ChevronRight className='mt-0.5 h-5 w-5 flex-shrink-0' />
              ))}
            <div className='min-w-0 flex-1'>
              <div className='mb-2 flex flex-wrap items-center gap-2'>
                <Zap className='text-primary h-4 w-4' />
                <span className='font-semibold'>{trace.model}</span>
                <Badge variant='secondary' className='text-xs'>
                  {t('traces.detail.levelBadge', { level })}
                </Badge>
              </div>

              <div className='grid grid-cols-2 gap-3 text-sm md:grid-cols-4'>
                <div className='flex items-center gap-2'>
                  <Clock className='text-muted-foreground h-3 w-3' />
                  <span className='text-muted-foreground'>{t('traces.detail.durationLabel')}</span>
                  <span className='font-medium'>{duration}</span>
                </div>

                {tokenRows.map((item) => (
                  <div key={item.label} className='flex items-center gap-1'>
                    <span className='text-muted-foreground'>{item.label}</span>
                    <span className='font-medium'>{item.value}</span>
                  </div>
                ))}

                {trace.metadata?.itemCount != null && (
                  <div className='flex items-center gap-1'>
                    <span className='text-muted-foreground'>{t('traces.detail.itemsLabel')}</span>
                    <span className='font-medium'>{trace.metadata.itemCount}</span>
                  </div>
                )}
              </div>

              {trace.startTime && (
                <div className='text-muted-foreground mt-2 text-xs'>{format(new Date(trace.startTime), 'yyyy-MM-dd HH:mm:ss.SSS')}</div>
              )}
            </div>
          </div>

          {expanded && (
            <div className='mt-4 space-y-4'>
              {/* Request Spans */}
              {trace.requestSpans && trace.requestSpans.length > 0 && (
                <div className='space-y-2'>
                  <h4 className='text-primary flex items-center gap-2 text-sm font-semibold'>
                    <span>📤</span> {t('traces.detail.requestSpansHeader', { count: trace.requestSpans.length })}
                  </h4>
                  <div className='space-y-1'>
                    {trace.requestSpans.map((span: Span) => (
                      <SpanItem
                        key={span.id}
                        span={span}
                        type='request'
                        isActive={selectedSpanId === span.id}
                        onSelect={() => onSpanSelect?.(trace, span, 'request')}
                      />
                    ))}
                  </div>
                </div>
              )}

              {/* Response Spans */}
              {trace.responseSpans && trace.responseSpans.length > 0 && (
                <div className='space-y-2'>
                  <h4 className='text-primary flex items-center gap-2 text-sm font-semibold'>
                    <span>📥</span> {t('traces.detail.responseSpansHeader', { count: trace.responseSpans.length })}
                  </h4>
                  <div className='space-y-1'>
                    {trace.responseSpans.map((span: Span) => (
                      <SpanItem
                        key={span.id}
                        span={span}
                        type='response'
                        isActive={selectedSpanId === span.id}
                        onSelect={() => onSpanSelect?.(trace, span, 'response')}
                      />
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Children */}
          {expanded && hasChildren && (
            <div className='space-y-2'>
              {trace.children!.map((child: Segment) => (
                <TraceTreeView key={child.id} trace={child} level={level + 1} onSpanSelect={onSpanSelect} selectedSpanId={selectedSpanId} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
