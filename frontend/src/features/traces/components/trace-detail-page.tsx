import { useMemo, useState, useEffect } from 'react'
import { format } from 'date-fns'
import { useParams, useNavigate } from '@tanstack/react-router'
import { zhCN, enUS } from 'date-fns/locale'
import { ArrowLeft, FileText, Activity } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { extractNumberID } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { useTraceWithSegments } from '../data'
import { TraceTimeline } from './trace-timeline'
import { Segment, Span } from '../data/schema'
import { getLocalizedSpanType, getSpanDisplayLabels } from '../utils/span-display'

export default function TraceDetailPage() {
  const { t, i18n } = useTranslation()
  const { traceId } = useParams({ from: '/_authenticated/project/traces/$traceId' })
  const navigate = useNavigate()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const [selectedTrace, setSelectedTrace] = useState<Segment | null>(null)
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null)
  const [selectedSpanType, setSelectedSpanType] = useState<'request' | 'response' | null>(null)

  const { data: trace, isLoading } = useTraceWithSegments(traceId)

  // Auto-select first span when trace loads
  useEffect(() => {
    if (trace?.rootSegment && !selectedSpan) {
      const firstSpan = trace.rootSegment.requestSpans?.[0] || trace.rootSegment.responseSpans?.[0]
      if (firstSpan) {
        const spanType = trace.rootSegment.requestSpans?.[0] ? 'request' : 'response'
        setSelectedTrace(trace.rootSegment)
        setSelectedSpan(firstSpan)
        setSelectedSpanType(spanType)
      }
    }
  }, [trace, selectedSpan])

  const handleSpanSelect = (parentTrace: Segment, span: Span, type: 'request' | 'response') => {
    setSelectedTrace(parentTrace)
    setSelectedSpan(span)
    setSelectedSpanType(type)
  }

  const spanSections = useMemo(() => {
    if (!selectedSpan?.value) return []

    const sections: { title: string; content: React.ReactNode }[] = []
    const { userQuery: query, text, thinking, toolUse, toolResult, imageUrl, systemInstruction } = selectedSpan.value

    if (query?.text) {
      sections.push({
        title: t('traces.detail.requestQuery'),
        content: (
          <div className='space-y-3'>
            <div>
              <p className='text-xs uppercase tracking-wide text-muted-foreground'>
                {t('traces.detail.promptLabel')}
              </p>
              <pre className='mt-2 max-h-72 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
                {query.text}
              </pre>
            </div>
          </div>
        ),
      })
    }

    if (text?.text) {
      sections.push({
        title: t('traces.detail.textOutput'),
        content: (
          <pre className='max-h-80 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
            {text.text}
          </pre>
        ),
      })
    }

    if (thinking?.thinking) {
      sections.push({
        title: t('traces.detail.thinking'),
        content: (
          <pre className='max-h-80 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/30 p-3 text-sm italic'>
            {thinking.thinking}
          </pre>
        ),
      })
    }

    if (toolUse) {
      sections.push({
        title: t('traces.detail.functionCall'),
        content: (
          <div className='space-y-3'>
            <div className='flex items-center justify-between rounded-lg border bg-background/70 px-3 py-2 text-sm'>
              <span className='text-muted-foreground'>{t('traces.detail.nameLabel')}</span>
              <span className='font-medium'>{toolUse.name}</span>
            </div>
            {toolUse.arguments && (
              <div>
                <p className='text-xs uppercase tracking-wide text-muted-foreground'>
                  {t('traces.detail.argumentsLabel')}
                </p>
                <pre className='mt-2 max-h-80 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
                  {toolUse.arguments}
                </pre>
              </div>
            )}
          </div>
        ),
      })
    }

    if (toolResult) {
      sections.push({
        title: t('traces.detail.functionResult'),
        content: (
          <div className='space-y-3'>
            {toolResult.isError && (
              <Badge variant='destructive' className='w-fit text-xs'>
                {t('traces.detail.error')}
              </Badge>
            )}
            {toolResult.text && (
              <pre className='max-h-80 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
                {toolResult.text}
              </pre>
            )}
          </div>
        ),
      })
    }

    if (imageUrl?.url) {
      sections.push({
        title: t('traces.detail.image'),
        content: (
          <img
            src={imageUrl.url || ''}
            alt={t('traces.detail.imageAlt')}
            className='max-h-96 w-full rounded-lg border object-contain'
          />
        ),
      })
    }

    if (systemInstruction?.instruction) {
      sections.push({
        title: t('traces.detail.systemInstruction'),
        content: (
          <pre className='max-h-80 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
            {systemInstruction.instruction}
          </pre>
        ),
      })
    }

    return sections
  }, [selectedSpan, t])

  const handleBack = () => {
    navigate({ to: '/project/traces' })
  }

  if (isLoading) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b'>
        </Header>
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
        <Header className='border-b'>
        </Header>
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-6 text-center'>
              <div className='space-y-2'>
                <Activity className='text-muted-foreground mx-auto h-16 w-16' />
                <p className='text-muted-foreground text-xl font-medium'>
                  {t('traces.detail.notFound')}
                </p>
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
              <div className='flex items-center gap-2 mt-1'>
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
        {trace.rootSegment ? (
          <div className='flex h-full'>
            {/* Left: Timeline */}
            <div className='flex-1 overflow-auto p-6'>
              <TraceTimeline
                trace={trace.rootSegment}
                onSelectSpan={(selectedTrace, span, type) => handleSpanSelect(selectedTrace, span, type)}
                selectedSpanId={selectedSpan?.id}
              />
            </div>

            {/* Right: Span Detail */}
            <div className='w-[500px] border-l border-border bg-background overflow-y-auto'>
              {selectedTrace && selectedSpan ? (
                <>
                  <div className='sticky top-0 z-10 space-y-3 border-b border-border bg-background/95 backdrop-blur px-6 py-5'>
                    <div className='flex flex-col gap-2'>
                      <div className='flex items-center justify-between gap-3'>
                        <div className='flex items-center gap-3'>
                          <div className='flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10'>
                            <Activity className='h-5 w-5 text-primary' />
                          </div>
                          <div>
                            <p className='text-xs uppercase tracking-wide text-muted-foreground'>
                              {selectedSpanType
                                ? t(`traces.common.badges.${selectedSpanType}`)
                                : t('traces.common.badges.trace')}
                            </p>
                            <span className='text-lg font-semibold leading-tight'>
                              {getSpanDisplayLabels(selectedSpan, t).primary}
                            </span>
                            <div className='text-xs text-muted-foreground'>
                              {getLocalizedSpanType(selectedSpan.type, t)}
                            </div>
                          </div>
                        </div>
                        <Badge variant='outline' className='text-xs capitalize'>
                          {selectedTrace.model}
                        </Badge>
                      </div>
                      <div className='flex flex-wrap items-center gap-2 text-xs text-muted-foreground'>
                        {selectedSpan.startTime && selectedSpan.endTime && (
                          <span>
                            {((new Date(selectedSpan.endTime).getTime() - new Date(selectedSpan.startTime).getTime()) / 1000).toFixed(3)}s
                          </span>
                        )}
                        {selectedTrace.metadata?.inputTokens && (
                          <>
                            <span>•</span>
                            <span>
                              {t('traces.detail.tokenSummary.input', {
                                value: selectedTrace.metadata.inputTokens.toLocaleString(),
                              })}
                            </span>
                          </>
                        )}
                        {selectedTrace.metadata?.outputTokens && (
                          <>
                            <span>•</span>
                            <span>
                              {t('traces.detail.tokenSummary.output', {
                                value: selectedTrace.metadata.outputTokens.toLocaleString(),
                              })}
                            </span>
                          </>
                        )}
                      </div>
                    </div>
                  </div>

                  <div className='space-y-4 px-6 py-6'>
                    {spanSections.length > 0 ? (
                      spanSections.map((section) => (
                        <div key={section.title} className='space-y-2'>
                          <h3 className='text-sm font-semibold text-foreground'>
                            {section.title}
                          </h3>
                          <div className='text-sm'>
                            {section.content}
                          </div>
                        </div>
                      ))
                    ) : (
                      <div className='flex min-h-[200px] flex-col items-center justify-center rounded-lg border border-dashed bg-muted/30 p-6 text-center text-sm text-muted-foreground'>
                        {t('traces.detail.noSpanContent')}
                      </div>
                    )}
                  </div>
                </>
              ) : (
                <div className='flex h-full items-center justify-center px-6 py-12 text-sm text-muted-foreground'>
                  {t('traces.detail.selectSpanHint')}
                </div>
              )}
            </div>
          </div>
        ) : (
          <div className='flex h-full items-center justify-center p-6'>
            <Card className='border-0 shadow-sm'>
              <CardContent className='py-16'>
                <div className='flex h-full items-center justify-center'>
                  <div className='space-y-4 text-center'>
                    <FileText className='text-muted-foreground mx-auto h-16 w-16' />
                    <p className='text-muted-foreground text-lg'>
                      {t('traces.detail.noTraceData')}
                    </p>
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
