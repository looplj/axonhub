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

    // 1.4 Handle OpenAI Responses API format (output[] array)
    const outputItems = body.output || body.response?.output;
    if (!fullContent && !fullReasoning && collectedToolCalls.length === 0 && Array.isArray(outputItems)) {
      outputItems.forEach((item: any) => {
        if (item.type === 'reasoning') {
          // Reasoning item with summary text parts
          if (Array.isArray(item.summary)) {
            item.summary.forEach((s: any) => {
              if (s.type === 'summary_text') fullReasoning += s.text || '';
            });
          }
        } else if (item.type === 'message' && Array.isArray(item.content)) {
          // Message item with output_text content parts
          item.content.forEach((c: any) => {
            if (c.type === 'output_text') fullContent += c.text || '';
          });
        } else if (item.type === 'function_call') {
          // Function call item
          collectedToolCalls.push({
            id: item.call_id || item.id,
            type: 'function',
            function: {
              name: item.name || 'unknown',
              arguments: item.arguments || '{}',
            },
          });
        }
      });
    }

    // 1.5 Handle direct content if it's just a string or has a content field
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

    // OpenAI Responses API state: keyed by item_id
    const responsesApiToolMap = new Map<string, { id: string; name: string; arguments: string }>();
    let isResponsesApiFormat = false;

    // Gemini API state
    const geminiToolMap = new Map<string, any>();
    let isGeminiFormat = false;

    normalizedChunks.forEach((chunk: any) => {
      const data = chunk.data || chunk;
      const eventType = data.type || chunk.event;

      // --- OpenAI Responses API format ---
      if (eventType?.startsWith('response.')) {
        isResponsesApiFormat = true;

        if (eventType === 'response.reasoning_summary_text.delta') {
          fullReasoning += data.delta || '';
        } else if (eventType === 'response.output_text.delta') {
          fullContent += data.delta || '';
        } else if (eventType === 'response.function_call_arguments.delta') {
          const itemId = data.item_id || '';
          if (itemId && responsesApiToolMap.has(itemId)) {
            responsesApiToolMap.get(itemId)!.arguments += data.delta || '';
          }
        } else if (eventType === 'response.output_item.added') {
          const item = data.item;
          if (item?.type === 'function_call') {
            responsesApiToolMap.set(item.id || item.call_id, {
              id: item.call_id || item.id,
              name: item.name || 'unknown',
              arguments: item.arguments || '',
            });
          }
        }
        // Skip all other response.* events
        return;
      }

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
      } else if (Array.isArray(data.candidates) && data.candidates.length > 0) {
        // --- Gemini streaming format ---
        isGeminiFormat = true;
        const contentObj = data.candidates[0].content;
        if (contentObj && Array.isArray(contentObj.parts)) {
          contentObj.parts.forEach((part: any, index: number) => {
            if (part.text) {
              if (part.thought) fullReasoning += part.text;
              else fullContent += part.text;
            } else if (part.functionCall) {
              const name = part.functionCall.name || 'unknown';
              const callId = name + '_' + index;
              geminiToolMap.set(callId, {
                id: callId,
                type: 'function',
                function: {
                  name: name,
                  arguments: typeof part.functionCall.args === 'string' 
                    ? part.functionCall.args 
                    : JSON.stringify(part.functionCall.args || {}),
                }
              });
            }
          });
        }
      } else if (typeof chunk === 'string') {
        fullContent += chunk;
      }
    });

    // Aggregate OpenAI Responses API tool calls
    if (isResponsesApiFormat && responsesApiToolMap.size > 0) {
      for (const [, tc] of responsesApiToolMap) {
        collectedToolCalls.push({
          id: tc.id,
          type: 'function',
          function: {
            name: tc.name,
            arguments: tc.arguments,
          },
        });
      }
    }

    // Aggregate Gemini tool calls
    if (isGeminiFormat && geminiToolMap.size > 0) {
      for (const [, tc] of geminiToolMap) {
        collectedToolCalls.push(tc);
      }
    }

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

    // Aggregate OpenAI Chat Completions tool calls
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
