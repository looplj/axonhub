import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { CopyIcon } from 'lucide-react';
import { Reasoning, ReasoningTrigger, ReasoningContent } from '@/components/ai-elements/reasoning';
import { Response as UIResponse } from '@/components/ai-elements/response';
import { Message, MessageContent } from '@/components/ai-elements/message';
import { Tool, ToolHeader, ToolContent } from '@/components/ai-elements/tool';
import { CodeBlock } from '@/components/ai-elements/code-block';
import { Badge } from '@/components/ui/badge';

import { parseResponse } from '../utils/response-parser';

interface ResponseFlowProps {
  chunks?: any[] | null;
  body?: any;
  isLive?: boolean;
  reasoningDurationMs?: number | null;
}

export function ResponseFlow({ chunks, body, isLive, reasoningDurationMs }: ResponseFlowProps) {
  const { t } = useTranslation();

  const { content, reasoning, toolCalls } = useMemo(
    () => parseResponse(body, chunks),
    [chunks, body]
  );

  if (!content && !reasoning && toolCalls.length === 0) {
    if (isLive) {
      return (
        <div className='flex min-h-[200px] w-full items-center justify-center rounded-xl border border-dashed bg-muted/5'>
            <div className='space-y-4 text-center'>
              <div className='border-primary mx-auto h-12 w-12 animate-spin rounded-full border-b-2'></div>
              <p className='text-muted-foreground text-lg'>{t('common.loading')}</p>
            </div>
        </div>
      );
    }
    return null;
  }

  const parseJson = (text: string) => {
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  };

  return (
    <div className='bg-muted/10 rounded-xl border p-6'>
      {isLive && (
        <div className='mb-4 flex justify-end'>
          <Badge className='bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300 gap-1.5 border-none px-2 py-0.5'>
            <span className='h-2 w-2 rounded-full bg-green-500 animate-pulse' />
            Live
          </Badge>
        </div>
      )}

      <Message from='assistant' fullWidth={true}>
        <MessageContent>
          {reasoning && (
            <Reasoning isStreaming={isLive} duration={reasoningDurationMs ? Math.ceil(reasoningDurationMs / 1000) : undefined}>
              <ReasoningTrigger />
              <ReasoningContent>{reasoning}</ReasoningContent>
            </Reasoning>
          )}

          {content && <UIResponse>{content}</UIResponse>}

          {toolCalls.length > 0 && (
            <div className='mt-4 space-y-3'>
              {toolCalls.map((tc, index) => (
                <Tool key={tc.id || index} defaultOpen={true}>
                  <ToolHeader 
                    title={tc.function?.name || 'tool'} 
                    type='tool-call' 
                    state={isLive ? 'input-available' : 'output-available'} 
                  />
                  <ToolContent>
                    {tc.id && (
                      <div className='px-4 pt-3 pb-1'>
                        <span className='text-muted-foreground font-mono text-xs'>ID: {tc.id}</span>
                      </div>
                    )}
                    <div className='space-y-2 overflow-hidden p-4'>
                      <div className='flex items-center justify-between'>
                        <h4 className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>Parameters</h4>
                        <button
                          type='button'
                          className='text-muted-foreground hover:text-foreground text-xs flex items-center gap-1 transition-colors cursor-pointer'
                          onClick={() => {
                            const text = typeof tc.function?.arguments === 'string'
                              ? tc.function.arguments
                              : JSON.stringify(parseJson(tc.function?.arguments || '{}'), null, 2);
                            navigator.clipboard.writeText(text);
                          }}
                        >
                          <CopyIcon className='size-3' />
                          Copy
                        </button>
                      </div>
                      <div className='bg-muted/50 rounded-md'>
                        <CodeBlock code={JSON.stringify(parseJson(tc.function?.arguments || '{}'), null, 2)} language='json' />
                      </div>
                    </div>
                  </ToolContent>
                </Tool>
              ))}
            </div>
          )}

          {!content && !toolCalls.length && isLive ? (
            <div className='flex items-center gap-2 text-sm text-muted-foreground italic'>
               <span className='h-1.5 w-1.5 animate-pulse rounded-full bg-primary' />
               {t('common.loading')}...
            </div>
          ) : null}
        </MessageContent>
      </Message>
    </div>
  );
}
