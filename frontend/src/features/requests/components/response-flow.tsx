import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Bot } from 'lucide-react';
import { Reasoning, ReasoningTrigger, ReasoningContent } from '@/components/ai-elements/reasoning';
import { Response as UIResponse } from '@/components/ai-elements/response';
import { Message, MessageContent } from '@/components/ai-elements/message';
import { Badge } from '@/components/ui/badge';

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
      // 1.1 Handle AxonHub / AI SDK 'parts' format
      if (Array.isArray(body.parts)) {
        body.parts.forEach((part: any) => {
          if (part.type === 'text') fullContent += part.text || '';
          if (part.type === 'reasoning') fullReasoning += part.text || '';
        });
      }
      
      // 1.2 Handle standard AI Message format (OpenAI/Anthropic)
      // For Anthropic, the body itself is the message. For OpenAI, it's in choices[0].message.
      const message = body.choices?.[0]?.message || (body.role && body.content ? body : null);
      
      if (message) {
        if (Array.isArray(message.content)) {
          // Handle array of content parts (Anthropic / OpenAI Multi-part)
          message.content.forEach((part: any) => {
            if (part.type === 'text') {
              fullContent += part.text || '';
            } else if (part.type === 'thinking') {
              fullReasoning += part.thinking || '';
            } else if (part.type === 'reasoning') {
              fullReasoning += part.text || part.reasoning || '';
            }
          });
        } else if (typeof message.content === 'string') {
          if (!fullContent) fullContent = message.content;
        }

        // Handle explicit reasoning fields (OpenAI / DeepSeek)
        if (message.reasoning_content && !fullReasoning) {
          fullReasoning = message.reasoning_content;
        }
      }

      // 1.3 Handle direct content if it's just a string or has a content field
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

      // Custom AxonHub / AI SDK format
      if (data.type === 'text-delta' && typeof data.delta === 'string') {
        fullContent += data.delta;
      } else if (data.type === 'reasoning-delta' && typeof data.delta === 'string') {
        fullReasoning += data.delta;
      } else if (data.choices?.[0]?.delta) {
        // Standard OpenAI format
        const delta = data.choices[0].delta;
        if (delta.content) fullContent += delta.content;
        if (delta.reasoning_content) fullReasoning += delta.reasoning_content;
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
      {isLive && (
        <div className='mb-4 flex justify-end'>
          <Badge className='bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300 gap-1.5 border-none px-2 py-0.5'>
            <span className='h-2 w-2 rounded-full bg-green-500 animate-pulse' />
            Live
          </Badge>
        </div>
      )}

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
