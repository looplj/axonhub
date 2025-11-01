import { useState } from 'react'
import { ChevronRight, ChevronDown, Clock, Zap } from 'lucide-react'
import { format } from 'date-fns'
import { useTranslation } from 'react-i18next'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Segment, Span } from '../data/schema'
import { cn } from '@/lib/utils'

interface TraceTreeViewProps {
  trace: Segment
  level?: number
  onSpanSelect?: (trace: Segment, span: Span, type: 'request' | 'response') => void
  selectedSpanId?: string
}

function SpanItem({
  span,
  type,
  onSelect,
  isActive,
}: {
  span: Span
  type: 'request' | 'response'
  onSelect?: () => void
  isActive?: boolean
}) {
  const { t } = useTranslation()
  const getSpanIcon = () => {
    switch (span.type) {
      case 'user_query':
        return '🔍'
      case 'text':
        return '📝'
      case 'thinking':
        return '💭'
      case 'tool_use':
        return '🔧'
      case 'tool_result':
        return '✅'
      case 'user_image_url':
      case 'image_url':
        return '🖼️'
      default:
        return '•'
    }
  }

  const duration = span.startTime && span.endTime
    ? `${((new Date(span.endTime).getTime() - new Date(span.startTime).getTime()) / 1000).toFixed(3)}s`
    : null

  return (
    <button
      type='button'
      onClick={(event) => {
        event.stopPropagation()
        onSelect?.()
      }}
      className={cn(
        'w-full rounded-lg border px-4 py-3 text-left transition-colors',
        'bg-muted/20 hover:bg-muted/40 hover:border-primary/40 flex flex-col gap-1',
        isActive && 'border-primary bg-primary/10 shadow-sm'
      )}
    >
      <div className='flex items-center justify-between gap-4'>
        <div className='flex items-center gap-2 min-w-0'>
          <span className='text-lg'>{getSpanIcon()}</span>
          <span className='truncate text-sm font-medium'>{span.type}</span>
          <Badge variant='secondary' className='text-xs capitalize'>
            {t(`traces.common.badges.${type}`)}
          </Badge>
        </div>
        <ChevronRight className='h-4 w-4 flex-shrink-0 text-muted-foreground' />
      </div>
      {duration && <span className='text-xs text-muted-foreground'>{duration}</span>}
    </button>
  )
}

export function TraceTreeView({ trace, level = 0, onSpanSelect, selectedSpanId }: TraceTreeViewProps) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(level === 0)

  const duration = trace.duration ? `${(trace.duration / 1000).toFixed(3)}s` : '0s'
  const hasChildren = trace.children && trace.children.length > 0
  const hasSpans = (trace.requestSpans && trace.requestSpans.length > 0) || 
                   (trace.responseSpans && trace.responseSpans.length > 0)

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
  ].filter(Boolean) as { label: string; value: string }[]

  return (
    <div className={cn('space-y-2', level > 0 && 'ml-6')}>
      <Card className='border-l-4 border-l-primary/70 hover:shadow-md transition-shadow'>
        <CardContent className='p-4'>
          <div 
            className='flex items-start gap-3'
            onClick={(e) => {
              e.stopPropagation()
              setExpanded(!expanded)
            }}
          >
            {(hasChildren || hasSpans) && (
              expanded ? <ChevronDown className='h-5 w-5 mt-0.5 flex-shrink-0' /> : <ChevronRight className='h-5 w-5 mt-0.5 flex-shrink-0' />
            )}
            <div className='flex-1 min-w-0'>
              <div className='flex items-center gap-2 flex-wrap mb-2'>
                <Zap className='h-4 w-4 text-primary' />
                <span className='font-semibold'>{trace.model}</span>
                <Badge variant='secondary' className='text-xs'>
                  {t('traces.detail.levelBadge', { level })}
                </Badge>
              </div>
              
              <div className='grid grid-cols-2 md:grid-cols-4 gap-3 text-sm'>
                <div className='flex items-center gap-2'>
                  <Clock className='h-3 w-3 text-muted-foreground' />
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
                <div className='text-xs text-muted-foreground mt-2'>
                  {format(new Date(trace.startTime), 'yyyy-MM-dd HH:mm:ss.SSS')}
                </div>
              )}
            </div>
          </div>

          {expanded && (
            <div className='mt-4 space-y-4'>
              {/* Request Spans */}
              {trace.requestSpans && trace.requestSpans.length > 0 && (
                <div className='space-y-2'>
                  <h4 className='text-sm font-semibold text-primary flex items-center gap-2'>
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
                  <h4 className='text-sm font-semibold text-primary flex items-center gap-2'>
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
                <TraceTreeView
                  key={child.id}
                  trace={child}
                  level={level + 1}
                  onSpanSelect={onSpanSelect}
                  selectedSpanId={selectedSpanId}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
