export interface ParsedResponse {
  content: string;
  reasoning: string;
  toolCalls: any[];
}

/**
 * Parses various AI response formats (OpenAI, Anthropic, Gemini, AI SDK)
 * including final bodies and streaming chunks.
 */
export function parseResponse(body?: any, chunks?: any[] | null): ParsedResponse {
  let fullContent = '';
  let fullReasoning = '';
  let collectedToolCalls: any[] = [];
  const normalizedChunks = chunks ?? [];

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
          } else if (part.type === 'tool_use') {
            // Anthropic tool_use: normalize to OpenAI-compatible structure
            collectedToolCalls.push({
              id: part.id,
              type: 'function',
              function: {
                name: part.name || 'unknown',
                arguments: typeof part.input === 'string' ? part.input : JSON.stringify(part.input || {}),
              },
            });
          }
        });
      } else if (typeof message.content === 'string') {
        if (!fullContent) fullContent = message.content;
      }

      if (message.reasoning_content && !fullReasoning) {
        fullReasoning = message.reasoning_content;
      }

      if (Array.isArray(message.tool_calls)) {
        collectedToolCalls = message.tool_calls;
      }
    }

    // 1.3 Handle Google Gemini format (candidates[0].content.parts)
    if (!fullContent && !fullReasoning && Array.isArray(body.candidates) && body.candidates.length > 0) {
      const contentObj = body.candidates[0].content;
      if (contentObj && Array.isArray(contentObj.parts)) {
        contentObj.parts.forEach((part: any) => {
          if (part.thought) {
            fullReasoning += part.text || '';
          } else {
            fullContent += part.text || '';
          }
        });
      }
    }

    // 1.4 Handle direct content if it's just a string or has a content field
    if (!fullContent && typeof body.content === 'string') {
      fullContent = body.content;
    }
  }

  // 2. Fallback to chunks aggregation (for live streaming or when body is not formatted)
  if (!fullContent && !fullReasoning && collectedToolCalls.length === 0 && normalizedChunks.length > 0) {
    const openaiToolCallMap = new Map<number, any>();

    // Anthropic content block state: keyed by block index
    // Each block: { type: 'thinking' | 'text' | 'tool_use', content: string, id?: string, name?: string }
    const anthropicBlockMap = new Map<number, { type: string; content: string; id?: string; name?: string }>();
    let isAnthropicFormat = false;

    normalizedChunks.forEach((chunk: any) => {
      const data = chunk.data || chunk;

      // --- Anthropic event-driven format ---
      if (data.type === 'message_start' || chunk.event === 'message_start') {
        isAnthropicFormat = true;
        return;
      }

      if (data.type === 'content_block_start' || chunk.event === 'content_block_start') {
        isAnthropicFormat = true;
        const index = data.index ?? 0;
        const block = data.content_block || {};
        anthropicBlockMap.set(index, {
          type: block.type || 'text',
          content: '',
          id: block.id,
          name: block.name,
        });
        return;
      }

      if (data.type === 'content_block_delta' || chunk.event === 'content_block_delta') {
        isAnthropicFormat = true;
        const index = data.index ?? 0;
        const delta = data.delta || {};

        // Initialize block if somehow missed the start event
        if (!anthropicBlockMap.has(index)) {
          const blockType = delta.type === 'thinking_delta' ? 'thinking' : delta.type === 'input_json_delta' ? 'tool_use' : 'text';
          anthropicBlockMap.set(index, { type: blockType, content: '' });
        }

        const block = anthropicBlockMap.get(index)!;

        if (delta.type === 'thinking_delta') {
          block.content += delta.thinking || '';
        } else if (delta.type === 'text_delta') {
          block.content += delta.text || '';
        } else if (delta.type === 'input_json_delta') {
          block.content += delta.partial_json || '';
        }
        return;
      }

      if (data.type === 'content_block_stop' || chunk.event === 'content_block_stop'
        || data.type === 'message_delta' || chunk.event === 'message_delta'
        || data.type === 'message_stop' || chunk.event === 'message_stop') {
        isAnthropicFormat = true;
        return;
      }

      // --- Custom AxonHub / AI SDK format ---
      if (data.type === 'text-delta' && typeof data.delta === 'string') {
        fullContent += data.delta;
      } else if (data.type === 'reasoning-delta' && typeof data.delta === 'string') {
        fullReasoning += data.delta;
      } else if (data.choices?.[0]?.delta) {
        // --- Standard OpenAI format ---
        const delta = data.choices[0].delta;
        if (delta.content) fullContent += delta.content;
        if (delta.reasoning_content) fullReasoning += delta.reasoning_content;

        if (Array.isArray(delta.tool_calls)) {
          delta.tool_calls.forEach((tc: any) => {
            const index = tc.index ?? 0;
            if (!openaiToolCallMap.has(index)) {
              openaiToolCallMap.set(index, {
                ...tc,
                function: tc.function ? { ...tc.function } : { name: '', arguments: '' }
              });
            } else {
              const existing = openaiToolCallMap.get(index);
              if (tc.id) existing.id = tc.id;
              if (tc.function?.name) existing.function.name = tc.function.name;
              if (tc.function?.arguments) {
                existing.function.arguments = (existing.function.arguments || '') + tc.function.arguments;
              }
            }
          });
        }
      } else if (typeof chunk === 'string') {
        fullContent += chunk;
      }
    });

    // Aggregate Anthropic blocks into final output
    if (isAnthropicFormat && anthropicBlockMap.size > 0) {
      const sortedBlocks = Array.from(anthropicBlockMap.entries()).sort(([a], [b]) => a - b);
      for (const [, block] of sortedBlocks) {
        if (block.type === 'thinking') {
          fullReasoning += block.content;
        } else if (block.type === 'text') {
          fullContent += block.content;
        } else if (block.type === 'tool_use') {
          collectedToolCalls.push({
            id: block.id,
            type: 'function',
            function: {
              name: block.name || 'unknown',
              arguments: block.content,
            },
          });
        }
      }
    }

    // Aggregate OpenAI tool calls
    if (openaiToolCallMap.size > 0 && collectedToolCalls.length === 0) {
      collectedToolCalls = Array.from(openaiToolCallMap.values()).sort((a, b) => (a.index || 0) - (b.index || 0));
    }
  }

  return {
    content: fullContent,
    reasoning: fullReasoning,
    toolCalls: collectedToolCalls,
  };
}
