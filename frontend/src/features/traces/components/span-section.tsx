import { useMemo } from 'react'
import { format } from 'date-fns'
import { useTranslation } from 'react-i18next'
import { Activity } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { JsonViewer } from '@/components/json-tree-view'
import type { Segment, Span } from '../data/schema'
import { getSpanDisplayLabels, getLocalizedSpanType } from '../utils/span-display'
import { formatNumber } from '@/utils/format-number'

interface SpanSectionProps {
  selectedTrace: Segment | null
  selectedSpan: Span | null
  selectedSpanType: 'request' | 'response' | null
}

export function SpanSection({ selectedTrace, selectedSpan, selectedSpanType }: SpanSectionProps) {
  const { t } = useTranslation()

  const spanSections = useMemo(() => {
    if (!selectedSpan?.value) return []

    const sections: { title: string; content: React.ReactNode }[] = []
    const { userQuery: query, userImageUrl, text, thinking, toolUse, toolResult, imageUrl, systemInstruction } = selectedSpan.value

    if (query?.text) {
      sections.push({
        title: t('traces.detail.requestQuery'),
        content: (
          <div className='space-y-3'>
            <div>
              <p className='text-xs uppercase tracking-wide text-muted-foreground'>
                {t('traces.detail.promptLabel')}
              </p>
              <pre className='mt-2 max-h-160 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
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
          <pre className='max-h-160 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
            {text.text}
          </pre>
        ),
      })
    }

    if (thinking?.thinking) {
      sections.push({
        title: t('traces.detail.thinking'),
        content: (
          <pre className='max-h-160 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/30 p-3 text-sm italic'>
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
                <div className='mt-2 max-h-80 overflow-auto rounded-lg bg-muted/40 p-3'>
                  <JsonViewer 
                    data={(() => {
                      try {
                        return JSON.parse(toolUse.arguments)
                      } catch {
                        return toolUse.arguments
                      }
                    })()} 
                    rootName="" 
                    defaultExpanded={true}
                    className="text-sm"
                  />
                </div>
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

    if (userImageUrl?.url) {
      sections.push({
        title: t('traces.detail.userImage'),
        content: (
          <img
            src={userImageUrl.url || ''}
            alt={t('traces.detail.userImageAlt')}
            className='max-h-96 w-full rounded-lg border object-contain'
          />
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
          <pre className='max-h-160 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-sm'>
            {systemInstruction.instruction}
          </pre>
        ),
      })
    }

    return sections
  }, [selectedSpan, t])

  if (!selectedTrace || !selectedSpan) {
    return (
      <div className='flex h-full items-center justify-center px-6 py-12 text-sm text-muted-foreground'>
        {t('traces.detail.selectSpanHint')}
      </div>
    )
  }

  return (
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
          <div className='space-y-1'>
            <div className='flex flex-wrap items-center gap-2 text-xs text-muted-foreground'>
              {selectedSpan.startTime && selectedSpan.endTime && (
                <span>
                  {((new Date(selectedSpan.endTime).getTime() - new Date(selectedSpan.startTime).getTime()) / 1000).toFixed(3)}s
                </span>
              )}
              {selectedTrace.startTime && selectedTrace.endTime && (
                <>
                  <span>•</span>
                  <span>
                    {t('traces.detail.segmentTime', {
                      start: format(new Date(selectedTrace.startTime), 'HH:mm:ss.SSS'),
                      end: format(new Date(selectedTrace.endTime), 'HH:mm:ss.SSS'),
                    })}
                  </span>
                </>
              )}
            </div>
            <div className='flex flex-wrap items-center gap-2 text-xs text-muted-foreground'>
              {selectedTrace.metadata?.inputTokens && (
                <span>
                  {t('traces.detail.tokenSummary.input', {
                    value: formatNumber(selectedTrace.metadata.inputTokens),
                  })}
                </span>
              )}
              {selectedTrace.metadata?.outputTokens && (
                <>
                  <span>•</span>
                  <span>
                    {t('traces.detail.tokenSummary.output', {
                      value: formatNumber(selectedTrace.metadata.outputTokens),
                    })}
                  </span>
                </>
              )}
              {selectedTrace.metadata?.cachedTokens && selectedTrace.metadata.cachedTokens > 0 && (
                <>
                  <span>•</span>
                  <span>
                    {t('traces.detail.tokenSummary.cached', {
                      value: formatNumber(selectedTrace.metadata.cachedTokens),
                    })}
                  </span>
                </>
              )}
            </div>
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
  )
}
