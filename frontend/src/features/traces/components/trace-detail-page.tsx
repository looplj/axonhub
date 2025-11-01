import { useMemo, useState } from 'react'
import { format } from 'date-fns'
import { useParams, useNavigate } from '@tanstack/react-router'
import { zhCN, enUS } from 'date-fns/locale'
import { ArrowLeft, FileText, Activity } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { extractNumberID } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet'
import { useTraceWithRequestTraces } from '../data'
import { TraceTreeView } from './trace-tree-view'
import { TraceTimeline } from './trace-timeline'
import { RequestTrace, Span } from '../data/schema'

export default function TraceDetailPage() {
  const { t, i18n } = useTranslation()
  const { traceId } = useParams({ from: '/_authenticated/project/traces/$traceId' })
  const navigate = useNavigate()
  const locale = i18n.language === 'zh' ? zhCN : enUS
  const [selectedTrace, setSelectedTrace] = useState<RequestTrace | null>(null)
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null)
  const [selectedSpanType, setSelectedSpanType] = useState<'request' | 'response' | null>(null)
  const [isDrawerOpen, setIsDrawerOpen] = useState(false)

  const { data: trace, isLoading } = useTraceWithRequestTraces(traceId)

  const handleSpanSelect = (parentTrace: RequestTrace, span: Span, type: 'request' | 'response') => {
    setSelectedTrace(parentTrace)
    setSelectedSpan(span)
    setSelectedSpanType(type)
    setIsDrawerOpen(true)
  }

  const handleDrawerChange = (open: boolean) => {
    setIsDrawerOpen(open)
    if (!open) {
      setSelectedSpan(null)
      setSelectedTrace(null)
      setSelectedSpanType(null)
    }
  }

  const spanSections = useMemo(() => {
    if (!selectedSpan?.value) return []

    const sections: { title: string; content: React.ReactNode }[] = []
    const { query, text, thinking, functionCall, functionResult, imageUrl } = selectedSpan.value

    if (query && (query.modelId || query.prompt)) {
      sections.push({
        title: t('traces.detail.requestQuery', 'Request'),
        content: (
          <div className='space-y-3'>
            {query.modelId && (
              <div className='flex items-center justify-between rounded-lg border bg-background/70 px-3 py-2 text-sm'>
                <span className='text-muted-foreground'>Model</span>
                <span className='font-medium'>{query.modelId}</span>
              </div>
            )}
            {query.prompt && (
              <div>
                <p className='text-xs uppercase tracking-wide text-muted-foreground'>Prompt</p>
                <pre className='mt-2 max-h-72 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
                  {query.prompt}
                </pre>
              </div>
            )}
          </div>
        ),
      })
    }

    if (text?.text) {
      sections.push({
        title: t('traces.detail.textOutput', 'Text Output'),
        content: (
          <pre className='max-h-80 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
            {text.text}
          </pre>
        ),
      })
    }

    if (thinking?.thinking) {
      sections.push({
        title: t('traces.detail.thinking', 'Thinking'),
        content: (
          <pre className='max-h-80 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/30 p-3 text-sm italic'>
            {thinking.thinking}
          </pre>
        ),
      })
    }

    if (functionCall) {
      sections.push({
        title: t('traces.detail.functionCall', 'Function Call'),
        content: (
          <div className='space-y-3'>
            <div className='flex items-center justify-between rounded-lg border bg-background/70 px-3 py-2 text-sm'>
              <span className='text-muted-foreground'>Name</span>
              <span className='font-medium'>{functionCall.name}</span>
            </div>
            {functionCall.arguments && (
              <div>
                <p className='text-xs uppercase tracking-wide text-muted-foreground'>Arguments</p>
                <pre className='mt-2 max-h-80 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
                  {functionCall.arguments}
                </pre>
              </div>
            )}
          </div>
        ),
      })
    }

    if (functionResult) {
      sections.push({
        title: t('traces.detail.functionResult', 'Function Result'),
        content: (
          <div className='space-y-3'>
            {functionResult.isError && (
              <Badge variant='destructive' className='w-fit text-xs'>
                {t('traces.detail.error', 'Error')}
              </Badge>
            )}
            {functionResult.text && (
              <pre className='max-h-80 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
                {functionResult.text}
              </pre>
            )}
          </div>
        ),
      })
    }

    if (imageUrl?.url) {
      sections.push({
        title: t('traces.detail.image', 'Generated Image'),
        content: (
          <img
            src={imageUrl.url || ''}
            alt='Span image'
            className='max-h-96 w-full rounded-lg border object-contain'
          />
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

      <Main className='flex-1 overflow-auto'>
        <div className='mx-auto flex w-full max-w-[1400px] flex-col gap-6 p-6'>
          {trace.rootRequestTrace ? (
            <div className='flex w-full flex-col gap-6'>
              {/* <Card className='border-0 shadow-sm'>
                <CardHeader className='pb-4'>
                  <CardTitle className='flex items-center justify-between'>
                    <div className='flex items-center gap-3'>
                      <div className='bg-primary/10 flex h-10 w-10 items-center justify-center rounded-lg'>
                        <Activity className='text-primary h-5 w-5' />
                      </div>
                      <span className='text-xl'>{t('traces.detail.requestTrace')}</span>
                    </div>
                    <Badge className='bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300' variant='secondary'>
                      {t('traces.detail.traceTree')}
                    </Badge>
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <TraceTreeView
                    trace={trace.rootRequestTrace}
                    onSpanSelect={handleSpanSelect}
                    selectedSpanId={selectedSpan?.id}
                  />
                </CardContent>
              </Card> */}

              <TraceTimeline
                trace={trace.rootRequestTrace}
                onSelectSpan={(selectedTrace, span, type) => handleSpanSelect(selectedTrace, span, type)}
                selectedSpanId={selectedSpan?.id}
              />
            </div>
          ) : (
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
          )}
        </div>

        {/* Trace Detail Drawer */}
        <Sheet open={isDrawerOpen} onOpenChange={handleDrawerChange}>
          <SheetContent className='w-full sm:max-w-3xl overflow-y-auto border-l border-border bg-background p-0'>
            {selectedTrace && selectedSpan && (
              <>
                <SheetHeader className='space-y-3 border-b border-border bg-background/95 px-6 py-5 text-left'>
                  <SheetTitle className='flex flex-col gap-2'>
                    <div className='flex items-center justify-between gap-3'>
                      <div className='flex items-center gap-3'>
                        <div className='flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10'>
                          <Activity className='h-6 w-6 text-primary' />
                        </div>
                        <div>
                          <p className='text-xs uppercase tracking-[0.2em] text-muted-foreground'>
                            {t('traces.detail.spanDetail', 'Span Detail')}
                          </p>
                          <span className='text-xl font-semibold leading-tight'>{selectedSpan.name || selectedSpan.type}</span>
                        </div>
                      </div>
                      <Badge variant='outline' className='text-xs capitalize'>
                        {selectedTrace.model}
                      </Badge>
                    </div>
                    <div className='flex flex-wrap items-center gap-3 text-sm text-muted-foreground'>
                      {selectedTrace.startTime && (
                        <span>
                          {format(new Date(selectedTrace.startTime), 'yyyy-MM-dd HH:mm:ss.SSS', { locale })}
                        </span>
                      )}
                      {selectedSpan.startTime && selectedSpan.endTime && (
                        <>
                          <Separator orientation='vertical' className='h-4' />
                          <span>
                            {t('traces.detail.duration')}: {((new Date(selectedSpan.endTime).getTime() - new Date(selectedSpan.startTime).getTime()) / 1000).toFixed(3)}s
                          </span>
                        </>
                      )}
                      {selectedSpanType && (
                        <>
                          <Separator orientation='vertical' className='h-4' />
                          <span className='capitalize'>{selectedSpanType}</span>
                        </>
                      )}
                    </div>
                  </SheetTitle>
                </SheetHeader>

                <div className='flex-1 space-y-6 overflow-y-auto px-6 py-6'>
                  <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
                    <div className='rounded-xl border bg-background/70 p-4 shadow-sm'>
                      <p className='text-xs uppercase tracking-wide text-muted-foreground'>
                        {t('traces.detail.duration')}
                      </p>
                      <p className='mt-2 text-lg font-semibold'>
                        {selectedSpan.startTime && selectedSpan.endTime
                          ? `${((new Date(selectedSpan.endTime).getTime() - new Date(selectedSpan.startTime).getTime()) / 1000).toFixed(3)}s`
                          : selectedTrace.duration || '—'}
                      </p>
                    </div>

                    <div className='rounded-xl border bg-background/70 p-4 shadow-sm'>
                      <p className='text-xs uppercase tracking-wide text-muted-foreground'>
                        {t('traces.detail.tokens')}
                      </p>
                      <p className='mt-2 text-lg font-semibold'>
                        {selectedTrace.metadata?.tokens
                          ? selectedTrace.metadata.tokens.toLocaleString()
                          : '—'}
                      </p>
                    </div>

                    <div className='rounded-xl border bg-background/70 p-4 shadow-sm'>
                      <p className='text-xs uppercase tracking-wide text-muted-foreground'>
                        {t('traces.detail.cost')}
                      </p>
                      <p className='mt-2 text-lg font-semibold'>
                        {selectedTrace.metadata?.cost
                          ? `$${selectedTrace.metadata.cost.toFixed(4)}`
                          : '—'}
                      </p>
                    </div>

                    <div className='rounded-xl border bg-background/70 p-4 shadow-sm'>
                      <p className='text-xs uppercase tracking-wide text-muted-foreground'>
                        {t('traces.detail.items')}
                      </p>
                      <p className='mt-2 text-lg font-semibold'>
                        {selectedTrace.metadata?.itemCount ?? '—'}
                      </p>
                    </div>
                  </div>

                  <Separator />

                  {spanSections.length > 0 ? (
                    <div className='space-y-5'>
                      {spanSections.map((section) => (
                        <Card key={section.title} className='border border-border/60 shadow-sm'>
                          <CardHeader className='border-b border-border/50 py-3'>
                            <CardTitle className='text-sm font-semibold text-muted-foreground'>
                              {section.title}
                            </CardTitle>
                          </CardHeader>
                          <CardContent className='py-5 text-sm'>
                            {section.content}
                          </CardContent>
                        </Card>
                      ))}
                    </div>
                  ) : (
                    <div className='flex min-h-[200px] flex-col items-center justify-center rounded-xl border border-dashed bg-muted/30 p-6 text-center text-sm text-muted-foreground'>
                      {t('traces.detail.noSpanContent', 'No additional content for this span.')}
                    </div>
                  )}
                </div>
              </>
            )}
            {!selectedSpan && (
              <div className='flex h-full min-h-[320px] items-center justify-center px-6 py-12 text-sm text-muted-foreground'>
                {t('traces.detail.selectSpanHint', 'Select a span from the timeline to inspect its detail.')}
              </div>
            )}
          </SheetContent>
        </Sheet>
      </Main>
    </div>
  )
}
