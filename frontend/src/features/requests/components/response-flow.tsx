import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Bot } from 'lucide-react';
import { Reasoning, ReasoningTrigger, ReasoningContent } from '@/components/ai-elements/reasoning';
import { Response as UIResponse } from '@/components/ai-elements/response';
import { Message, MessageContent } from '@/components/ai-elements/message';
import { Tool, ToolHeader, ToolInput, ToolContent } from '@/components/ai-elements/tool';
import { Badge } from '@/components/ui/badge';

interface ResponseFlowProps {
  chunks?: any[] | null;
  body?: any;
  isLive?: boolean;
}

export function ResponseFlow({ chunks, body, isLive }: ResponseFlowProps) {
  const { t } = useTranslation();

  const normalizedChunks = chunks ?? [];

  const { content, reasoning, toolCalls } = useMemo(() => {
    let fullContent = '';
    let fullReasoning = '';
    let collectedToolCalls: any[] = [];

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
      const message = body.choices?.[0]?.message || (body.role && body.content ? body : null);

      if (message) {
        if (Array.isArray(message.content)) {
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

        if (message.reasoning_content && !fullReasoning) {
          fullReasoning = message.reasoning_content;
        }

        // Extract tool calls
        if (Array.isArray(message.tool_calls)) {
          collectedToolCalls = message.tool_calls;
        }
      }

      // 1.3 Handle direct content if it's just a string or has a content field
      if (!fullContent && typeof body.content === 'string') {
        fullContent = body.content;
      }
    }

    // 2. Fallback to chunks aggregation (for live streaming)
    if (normalizedChunks.length > 0) {
      const toolCallMap = new Map<number, any>();

      normalizedChunks.forEach((chunk) => {
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

          // Standard OpenAI tool calls delta
          if (Array.isArray(delta.tool_calls)) {
            delta.tool_calls.forEach((tc: any) => {
              const index = tc.index ?? 0;
              if (!toolCallMap.has(index)) {
                toolCallMap.set(index, { 
                  ...tc,
                  function: tc.function ? { ...tc.function } : { name: '', arguments: '' }
                });
              } else {
                const existing = toolCallMap.get(index);
                if (tc.id) existing.id = tc.id;
                if (tc.function?.name) existing.function.name = tc.function.name;
                if (tc.function?.arguments) {
                  existing.function.arguments = (existing.function.arguments || '') + tc.function.arguments;
                }
              }
            });
          }
        } else if (typeof chunk === 'string') {
          // Fallback for raw string chunks
          fullContent += chunk;
        }
      });

      if (toolCallMap.size > 0 && collectedToolCalls.length === 0) {
        collectedToolCalls = Array.from(toolCallMap.values()).sort((a, b) => (a.index || 0) - (b.index || 0));
      }
    }

    return { content: fullContent, reasoning: fullReasoning, toolCalls: collectedToolCalls };
  }, [normalizedChunks, body]);

  if (!content && !reasoning && toolCalls.length === 0) {
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

      <Message from='assistant'>
        <MessageContent>
          {reasoning && (
            <Reasoning isStreaming={isLive}>
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
                    <ToolInput input={parseJson(tc.function?.arguments || '{}')} />
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
