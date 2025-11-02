import { useMemo, useState, useEffect } from 'react'
import { format } from 'date-fns'
import { useParams, useNavigate } from '@tanstack/react-router'
import { zhCN, enUS } from 'date-fns/locale'
import { ArrowLeft, FileText, Activity } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { extractNumberID } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { useTraceWithSegments } from '../data'
import { Segment, Span, parseRawRootSegment } from '../data/schema'
import { SpanSection } from './span-section'
import { TraceFlatTimeline } from './trace-flat-timeline'

export default function TraceDetailPage() {
  const { t, i18n } = useTranslation()
  const { traceId } = useParams({ from: '/_authenticated/project/traces/$traceId' })
  const navigate = useNavigate()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const [selectedTrace, setSelectedTrace] = useState<Segment | null>(null)
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null)
  const [selectedSpanType, setSelectedSpanType] = useState<'request' | 'response' | null>(null)

  const { data: trace, isLoading } = useTraceWithSegments(traceId)

  // Parse rawRootSegment or fallback to rootSegment
  // 使用 rawRootSegment（完整的 JSON）或回退到旧的 rootSegment 实现
  const effectiveRootSegment = useMemo(() => {
    if (!trace) return null

    // Try to use rawRootSegment first (new implementation)
    // 优先使用 rawRootSegment（新实现）
    if (trace.rawRootSegment) {
      const parsed = parseRawRootSegment(trace.rawRootSegment)
      if (parsed) {
        return parsed
      }
    }

    // Fallback to old rootSegment implementation
    // 回退到旧的 rootSegment 实现
    return trace.rootSegment || null
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

  const handleBack = () => {
    navigate({ to: '/project/traces' })
  }

  if (isLoading) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b'></Header>
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-4 text-center'>
              <div className='border-primary mx-auto h-12 w-12 animate-spin rounded-full border-b-2'></div>
              <p className='text-muted-foreground text-lg'>{t('common.loading')}</p>
            </div>
          </div>
        </Main>
      </div>
    )
  }

  if (!trace) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b'></Header>
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-6 text-center'>
              <div className='space-y-2'>
                <Activity className='text-muted-foreground mx-auto h-16 w-16' />
                <p className='text-muted-foreground text-xl font-medium'>{t('traces.detail.notFound')}</p>
              </div>
              <Button onClick={handleBack} size='lg'>
                <ArrowLeft className='mr-2 h-4 w-4' />
                {t('common.back')}
              </Button>
            </div>
          </div>
        </Main>
      </div>
    )
  }

  return (
    <div className='flex h-screen flex-col'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 border-b backdrop-blur'>
        <div className='flex items-center space-x-4'>
          <Button variant='ghost' size='sm' onClick={handleBack} className='hover:bg-accent'>
            <ArrowLeft className='mr-2 h-4 w-4' />
            {t('common.back')}
          </Button>
          <Separator orientation='vertical' className='h-6' />
          <div className='flex items-center space-x-3'>
            <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-lg'>
              <Activity className='text-primary h-4 w-4' />
            </div>
            <div>
              <h1 className='text-lg leading-none font-semibold'>
                {t('traces.detail.title')} #{extractNumberID(trace.id) || trace.traceID}
              </h1>
              <div className='mt-1 flex items-center gap-2'>
                <p className='text-muted-foreground text-sm'>{trace.traceID}</p>
                <span className='text-muted-foreground text-xs'>•</span>
                <p className='text-muted-foreground text-xs'>
                  {format(new Date(trace.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}
                </p>
              </div>
            </div>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-hidden'>
        {effectiveRootSegment ? (
          <div className='flex h-full'>
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
          <div className='flex h-full items-center justify-center p-6'>
            <Card className='border-0 shadow-sm'>
              <CardContent className='py-16'>
                <div className='flex h-full items-center justify-center'>
                  <div className='space-y-4 text-center'>
                    <FileText className='text-muted-foreground mx-auto h-16 w-16' />
                    <p className='text-muted-foreground text-lg'>{t('traces.detail.noTraceData')}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        )}
      </Main>
    </div>
  )
}
