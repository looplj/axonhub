import { useMemo } from 'react';
import { format } from 'date-fns';
import { Activity } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { Badge } from '@/components/ui/badge';
import { JsonViewer } from '@/components/json-tree-view';
import type { Segment, Span } from '../data/schema';
import { getSpanDisplayLabels, getLocalizedSpanType } from '../utils/span-display';

interface SpanSectionProps {
  selectedTrace: Segment | null;
  selectedSpan: Span | null;
  selectedSpanType: 'request' | 'response' | null;
}

export function SpanSection({ selectedTrace, selectedSpan, selectedSpanType }: SpanSectionProps) {
  const { t } = useTranslation();

  const spanSections = useMemo(() => {
    if (!selectedSpan?.value) return [];

    const sections: { title: string; content: React.ReactNode }[] = [];
    const { userQuery: query, userImageUrl, text, thinking, toolUse, toolResult, imageUrl, systemInstruction } = selectedSpan.value;

    if (query?.text) {
      sections.push({
        title: t('traces.detail.requestQuery'),
        content: (
          <div className='space-y-3'>
            <div>
              <p className='text-muted-foreground text-xs tracking-wide uppercase'>{t('traces.detail.promptLabel')}</p>
              <pre className='bg-muted/40 mt-2 max-h-160 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>{query.text}</pre>
            </div>
          </div>
        ),
      });
    }

    if (text?.text) {
      sections.push({
        title: t('traces.detail.textOutput'),
        content: <pre className='bg-muted/40 max-h-160 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>{text.text}</pre>,
      });
    }

    if (thinking?.thinking) {
      sections.push({
        title: t('traces.detail.thinking'),
        content: (
          <pre className='bg-muted/30 max-h-160 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap italic'>{thinking.thinking}</pre>
        ),
      });
    }

    if (toolUse) {
      sections.push({
        title: t('traces.detail.functionCall'),
        content: (
          <div className='space-y-3'>
            <div className='bg-background/70 flex items-center justify-between rounded-lg border px-3 py-2 text-sm'>
              <span className='text-muted-foreground'>{t('traces.detail.nameLabel')}</span>
              <span className='font-medium'>{toolUse.name}</span>
            </div>
            {toolUse.id && (
              <div className='bg-background/70 flex items-center justify-between rounded-lg border px-3 py-2 text-sm'>
                <span className='text-muted-foreground'>{t('traces.detail.idLabel')}</span>
                <span className='font-mono text-xs'>{toolUse.id}</span>
              </div>
            )}
            {toolUse.arguments && (
              <div>
                <p className='text-muted-foreground text-xs tracking-wide uppercase'>{t('traces.detail.argumentsLabel')}</p>
                <div className='bg-muted/40 mt-2 max-h-80 overflow-auto rounded-lg p-3'>
                  <JsonViewer
                    data={(() => {
                      try {
                        return JSON.parse(toolUse.arguments);
                      } catch {
                        return toolUse.arguments;
                      }
                    })()}
                    rootName=''
                    defaultExpanded={true}
                    className='text-sm'
                  />
                </div>
              </div>
            )}
          </div>
        ),
      });
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
              <pre className='bg-muted/40 max-h-80 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>{toolResult.text}</pre>
            )}
          </div>
        ),
      });
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
      });
    }

    if (imageUrl?.url) {
      sections.push({
        title: t('traces.detail.image'),
        content: (
          <img src={imageUrl.url || ''} alt={t('traces.detail.imageAlt')} className='max-h-96 w-full rounded-lg border object-contain' />
        ),
      });
    }

    if (systemInstruction?.instruction) {
      sections.push({
        title: t('traces.detail.systemInstruction'),
        content: (
          <pre className='bg-muted/40 max-h-160 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>
            {systemInstruction.instruction}
          </pre>
        ),
      });
    }

    return sections;
  }, [selectedSpan, t]);

  if (!selectedTrace || !selectedSpan) {
    return (
      <div className='text-muted-foreground flex h-full items-center justify-center px-6 py-12 text-sm'>
        {t('traces.detail.selectSpanHint')}
      </div>
    );
  }

  return (
    <>
      <div className='border-border bg-background/95 sticky top-0 z-10 space-y-3 border-b px-6 py-5 backdrop-blur'>
        <div className='flex flex-col gap-2'>
          <div className='flex items-center justify-between gap-3'>
            <div className='flex items-center gap-3'>
              <div className='bg-primary/10 flex h-10 w-10 items-center justify-center rounded-lg'>
                <Activity className='text-primary h-5 w-5' />
              </div>
              <div>
                <p className='text-muted-foreground text-xs tracking-wide uppercase'>
                  {selectedSpanType ? t(`traces.common.badges.${selectedSpanType}`) : t('traces.common.badges.trace')}
                </p>
                <span className='text-lg leading-tight font-semibold'>{getSpanDisplayLabels(selectedSpan, t).primary}</span>
                <div className='text-muted-foreground text-xs'>{getLocalizedSpanType(selectedSpan.type, t)}</div>
              </div>
            </div>
            <Badge variant='outline' className='text-xs capitalize'>
              {selectedTrace.model}
            </Badge>
          </div>
          <div className='space-y-1'>
            <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-xs'>
              {selectedSpan.startTime && selectedSpan.endTime && (
                <span>{((new Date(selectedSpan.endTime).getTime() - new Date(selectedSpan.startTime).getTime()) / 1000).toFixed(3)}s</span>
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
            <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-xs'>
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
              <h3 className='text-foreground text-sm font-semibold'>{section.title}</h3>
              <div className='text-sm'>{section.content}</div>
            </div>
          ))
        ) : (
          <div className='bg-muted/30 text-muted-foreground flex min-h-[200px] flex-col items-center justify-center rounded-lg border border-dashed p-6 text-center text-sm'>
            {t('traces.detail.noSpanContent')}
          </div>
        )}
      </div>
    </>
  );
}
