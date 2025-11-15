import { useMemo, useState, useEffect } from 'react'
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { useTraceWithSegments } from '@/features/traces/data'
import { Segment, Span, parseRawRootSegment } from '@/features/traces/data/schema'
import { SpanSection } from '@/features/traces/components/span-section'
import { TraceFlatTimeline } from '@/features/traces/components/trace-flat-timeline'

interface TraceDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  traceId: string | null
}

export function TraceDrawer({ open, onOpenChange, traceId }: TraceDrawerProps) {
  const { t } = useTranslation()
  const [selectedTrace, setSelectedTrace] = useState<Segment | null>(null)
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null)
  const [selectedSpanType, setSelectedSpanType] = useState<'request' | 'response' | null>(null)

  const { data: trace, isLoading } = useTraceWithSegments(traceId || '')

  // Parse rawRootSegment JSON once per trace
  const effectiveRootSegment = useMemo(() => {
    if (!trace?.rawRootSegment) return null
    return parseRawRootSegment(trace.rawRootSegment)
  }, [trace])

  // Auto-select first span when trace loads
  useEffect(() => {
    if (effectiveRootSegment && !selectedSpan) {
      const firstSpan = effectiveRootSegment.requestSpans?.[0] || effectiveRootSegment.responseSpans?.[0]
      if (firstSpan) {
        const spanType = effectiveRootSegment.requestSpans?.[0] ? 'request' : 'response'
        setSelectedTrace(effectiveRootSegment)
        setSelectedSpan(firstSpan)
        setSelectedSpanType(spanType)
      }
    }
  }, [effectiveRootSegment, selectedSpan])

  const handleSpanSelect = (parentTrace: Segment, span: Span, type: 'request' | 'response') => {
    setSelectedTrace(parentTrace)
    setSelectedSpan(span)
    setSelectedSpanType(type)
  }

  // Reset state when drawer closes
  useEffect(() => {
    if (!open) {
      setSelectedTrace(null)
      setSelectedSpan(null)
      setSelectedSpanType(null)
    }
  }, [open])

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side='right' className='w-full sm:max-w-[min(90vw,calc(100vw-400px))] p-0'>
        <SheetHeader className='border-b px-6 py-4'>
          <div className='flex items-center justify-between'>
            <SheetTitle>{t('traces.detail.title')}</SheetTitle>
            <Button variant='ghost' size='sm' onClick={() => onOpenChange(false)}>
              <X className='h-4 w-4' />
            </Button>
          </div>
        </SheetHeader>

        {isLoading ? (
          <div className='flex h-[calc(100vh-80px)] items-center justify-center'>
            <div className='space-y-4 text-center'>
              <div className='border-primary mx-auto h-12 w-12 animate-spin rounded-full border-b-2'></div>
              <p className='text-muted-foreground text-lg'>{t('common.loading')}</p>
            </div>
          </div>
        ) : effectiveRootSegment ? (
          <div className='flex h-[calc(100vh-80px)]'>
            {/* Left: Timeline */}
            <div className='flex-1 overflow-auto p-6'>
              <TraceFlatTimeline
                trace={effectiveRootSegment}
                onSelectSpan={(selectedTrace, span, type) => handleSpanSelect(selectedTrace, span, type)}
                selectedSpanId={selectedSpan?.id}
              />
            </div>

            {/* Right: Span Detail */}
            <div className='border-border bg-background w-[500px] overflow-y-auto border-l'>
              <SpanSection
                selectedTrace={selectedTrace}
                selectedSpan={selectedSpan}
                selectedSpanType={selectedSpanType}
              />
            </div>
          </div>
        ) : (
          <div className='flex h-[calc(100vh-80px)] items-center justify-center'>
            <p className='text-muted-foreground text-lg'>{t('traces.detail.noTraceData')}</p>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
