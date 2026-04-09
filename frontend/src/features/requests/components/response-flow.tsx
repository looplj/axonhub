import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Bot } from 'lucide-react';
import { Reasoning, ReasoningTrigger, ReasoningContent } from '@/components/ai-elements/reasoning';
import { Response as UIResponse } from '@/components/ai-elements/response';
import { Message, MessageContent } from '@/components/ai-elements/message';

interface ResponseFlowProps {
  chunks?: any[] | null;
  body?: any;
  isLive?: boolean;
}

export function ResponseFlow({ chunks, body, isLive }: ResponseFlowProps) {
  const { t } = useTranslation();

  const normalizedChunks = chunks ?? [];

  const { content, reasoning } = useMemo(() => {
    let fullContent = '';
    let fullReasoning = '';

    // 1. Try to parse from body first (final result)
    if (body) {
      // Handle AxonHub / AI SDK 'parts' format
      if (Array.isArray(body.parts)) {
        body.parts.forEach((part: any) => {
          if (part.type === 'text') fullContent += part.text || '';
          if (part.type === 'reasoning') fullReasoning += part.text || '';
        });
      }
      
      // Handle standard OpenAI message format
      const message = body.choices?.[0]?.message || body.message;
      if (message) {
        if (message.content && !fullContent) fullContent = message.content;
        if (message.reasoning_content && !fullReasoning) fullReasoning = message.reasoning_content;
      }

      // Handle direct content if it's just a string or has a content field
      if (!fullContent && typeof body.content === 'string') {
        fullContent = body.content;
      }

      if (fullContent || fullReasoning) {
        return { content: fullContent, reasoning: fullReasoning };
      }
    }

    // 2. Fallback to chunks aggregation (for live streaming)
    normalizedChunks.forEach((chunk) => {
      // Handle different formats
      const data = chunk.data || chunk;

      // Custom AxonHub format: data.type === 'text-delta'/'reasoning-delta'
      if (data.type === 'text-delta' && typeof data.delta === 'string') {
        fullContent += data.delta;
      } else if (data.type === 'reasoning-delta' && typeof data.delta === 'string') {
        fullReasoning += data.delta;
      } else if (typeof chunk === 'string') {
        // Fallback for raw string chunks
        fullContent += chunk;
      }
    });

    return { content: fullContent, reasoning: fullReasoning };
  }, [normalizedChunks, body]);

  if (!content && !reasoning) {
    return null;
  }

  return (
    <div className='bg-muted/10 rounded-xl border p-6'>
      <div className='mb-4 flex items-center gap-2'>
        <div className='bg-primary/10 flex h-7 w-7 items-center justify-center rounded-lg'>
          <Bot className='text-primary h-4 w-4' />
        </div>
        <h4 className='text-sm font-semibold'>{t('requests.detail.messagePreview')}</h4>
      </div>

      <Message from='assistant'>
        <MessageContent>
          {reasoning && (
            <Reasoning isStreaming={isLive}>
              <ReasoningTrigger />
              <ReasoningContent>{reasoning}</ReasoningContent>
            </Reasoning>
          )}
          {content ? (
            <UIResponse>{content}</UIResponse>
          ) : isLive ? (
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
