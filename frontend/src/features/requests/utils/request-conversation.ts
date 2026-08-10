export interface ConversationToolCall {
  id?: string;
  name: string;
  arguments: string;
}

export interface ConversationMessage {
  index: number;
  role: string;
  content: string;
  contentParts?: any[];
  reasoning?: string;
  toolCalls?: ConversationToolCall[];
  toolCallId?: string;
  raw: any;
}

export interface ConversationTool {
  name: string;
  description: string;
  parameters: any;
}

export interface ConversationData {
  model?: string;
  maxTokens?: number;
  stream?: boolean;
  toolChoice?: any;
  messages: ConversationMessage[];
  tools: ConversationTool[];
}

const isRecord = (v: unknown): v is Record<string, any> => typeof v === 'object' && v !== null && !Array.isArray(v);

function stringify(v: unknown): string {
  if (v === undefined || v === null) return '';
  if (typeof v === 'string') return v;
  try {
    return JSON.stringify(v);
  } catch {
    return String(v);
  }
}

function prettyJson(v: unknown): string {
  if (typeof v === 'string') {
    try {
      return JSON.stringify(JSON.parse(v), null, 2);
    } catch {
      return v;
    }
  }
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
}

/** OpenAI message content -> text + raw parts. */
function extractOpenAIContent(content: unknown): { text: string; parts?: any[] } {
  if (typeof content === 'string') return { text: content };
  if (Array.isArray(content)) {
    const text = content
      .map((p) => {
        if (typeof p === 'string') return p;
        if (isRecord(p)) {
          if (p.type === 'text' && typeof p.text === 'string') return p.text;
          if (p.type === 'input_text' && typeof p.text === 'string') return p.text;
          if (p.type === 'output_text' && typeof p.text === 'string') return p.text;
        }
        return '';
      })
      .filter(Boolean)
      .join('');
    return { text, parts: content };
  }
  return { text: '' };
}

function normalizeToolCalls(toolCalls: unknown): ConversationToolCall[] | undefined {
  if (!Array.isArray(toolCalls)) return undefined;
  const calls = toolCalls
    .map((tc) => {
      if (!isRecord(tc)) return null;
      const fn = isRecord(tc.function) ? tc.function : {};
      if (!fn.name) return null;
      return {
        id: typeof tc.id === 'string' ? tc.id : undefined,
        name: String(fn.name),
        arguments: prettyJson(fn.arguments),
      };
    })
    .filter((c): c is ConversationToolCall => c !== null);
  return calls.length > 0 ? calls : undefined;
}

function normalizeOpenAITools(tools: unknown): ConversationTool[] {
  if (!Array.isArray(tools)) return [];
  return tools
    .map((t) => {
      if (!isRecord(t)) return null;
      const fn = isRecord(t.function) ? t.function : t;
      if (!fn.name) return null;
      return {
        name: String(fn.name),
        description: typeof fn.description === 'string' ? fn.description : '',
        parameters: fn.parameters ?? {},
      };
    })
    .filter((t): t is ConversationTool => t !== null);
}

/** Anthropic message content blocks. */
function parseAnthropicContent(content: unknown): { text: string; reasoning?: string; toolCalls?: ConversationToolCall[]; toolResult?: { id: string; content: string }; parts?: any[] } {
  if (typeof content === 'string') return { text: content };
  if (!Array.isArray(content)) return { text: '' };

  let text = '';
  let reasoning = '';
  const toolCalls: ConversationToolCall[] = [];
  let toolResult: { id: string; content: string } | undefined;

  content.forEach((block) => {
    if (!isRecord(block)) {
      text += stringify(block);
      return;
    }
    switch (block.type) {
      case 'text':
        text += stringify(block.text);
        break;
      case 'thinking':
        reasoning += stringify(block.thinking);
        break;
      case 'redacted_thinking':
        reasoning += stringify(block.data);
        break;
      case 'tool_use':
        toolCalls.push({
          id: stringify(block.id) || undefined,
          name: stringify(block.name),
          arguments: prettyJson(block.input),
        });
        break;
      case 'tool_result':
        toolResult = {
          id: stringify(block.tool_use_id),
          content: prettyJson(block.content),
        };
        break;
      default:
        text += stringify(block.text ?? block);
    }
  });

  return {
    text,
    reasoning: reasoning || undefined,
    toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
    toolResult,
    parts: content,
  };
}

function normalizeAnthropicTools(tools: unknown): ConversationTool[] {
  if (!Array.isArray(tools)) return [];
  return tools
    .map((t) => {
      if (!isRecord(t)) return null;
      if (!t.name) return null;
      return {
        name: String(t.name),
        description: typeof t.description === 'string' ? t.description : '',
        parameters: t.input_schema ?? {},
      };
    })
    .filter((t): t is ConversationTool => t !== null);
}

/** Gemini content parts. */
function parseGeminiContent(parts: unknown): { text: string; reasoning?: string; toolCalls?: ConversationToolCall[]; toolResult?: { id: string; content: string }; partsRaw?: any[] } {
  if (!Array.isArray(parts)) return { text: '' };

  let text = '';
  let reasoning = '';
  const toolCalls: ConversationToolCall[] = [];
  let toolResult: { id: string; content: string } | undefined;

  parts.forEach((part, i) => {
    if (!isRecord(part)) {
      text += stringify(part);
      return;
    }
    if (typeof part.text === 'string') {
      if (part.thought) reasoning += part.text;
      else text += part.text;
      return;
    }
    if (isRecord(part.functionCall)) {
      toolCalls.push({
        id: stringify(part.functionCall.id) || `gemini-fn-${i}`,
        name: stringify(part.functionCall.name),
        arguments: prettyJson(part.functionCall.args),
      });
      return;
    }
    if (isRecord(part.functionResponse)) {
      toolResult = {
        id: stringify(part.functionResponse.id) || stringify(part.functionResponse.name),
        content: prettyJson(part.functionResponse.response),
      };
      return;
    }
    text += prettyJson(part);
  });

  return {
    text,
    reasoning: reasoning || undefined,
    toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
    toolResult,
    partsRaw: parts,
  };
}

function normalizeGeminiTools(tools: unknown): ConversationTool[] {
  if (!Array.isArray(tools)) return [];
  const result: ConversationTool[] = [];
  tools.forEach((t) => {
    if (!isRecord(t)) return;
    const decls = Array.isArray(t.functionDeclarations) ? t.functionDeclarations : [];
    decls.forEach((d) => {
      if (!isRecord(d)) return;
      if (!d.name) return;
      result.push({
        name: String(d.name),
        description: typeof d.description === 'string' ? d.description : '',
        parameters: d.parameters ?? {},
      });
    });
  });
  return result;
}

/** AI SDK message parts. */
function parseAiSdkContent(parts: unknown): { text: string; reasoning?: string; toolCalls?: ConversationToolCall[]; toolResults?: { id: string; content: string }[]; partsRaw?: any[] } {
  if (!Array.isArray(parts)) return { text: '' };

  let text = '';
  let reasoning = '';
  const toolCalls: ConversationToolCall[] = [];
  const toolResults: { id: string; content: string }[] = [];

  parts.forEach((p) => {
    if (!isRecord(p)) {
      text += stringify(p);
      return;
    }
    switch (p.type) {
      case 'text':
        text += stringify(p.text);
        break;
      case 'reasoning':
      case 'reasoning-summary':
        reasoning += stringify(p.text);
        break;
      case 'tool-call':
        toolCalls.push({
          id: stringify(p.toolCallId) || undefined,
          name: stringify(p.toolName),
          arguments: prettyJson(p.args),
        });
        break;
      case 'tool-result':
        toolResults.push({
          id: stringify(p.toolCallId),
          content: prettyJson(p.result),
        });
        break;
      default:
        if (typeof p.text === 'string') text += p.text;
        else if (typeof p.content === 'string') text += p.content;
    }
  });

  return {
    text,
    reasoning: reasoning || undefined,
    toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
    toolResults: toolResults.length > 0 ? toolResults : undefined,
    partsRaw: parts,
  };
}

function isAiSdkShape(messages: unknown): boolean {
  if (!Array.isArray(messages)) return false;
  return messages.some((m) => isRecord(m) && Array.isArray(m.parts) && m.parts.length > 0 && m.parts.some((p) => isRecord(p) && typeof p.type === 'string'));
}

function isAnthropicShape(messages: unknown): boolean {
  if (!Array.isArray(messages)) return false;
  return messages.some((m) => isRecord(m) && Array.isArray(m.content) && m.content.some((b) => isRecord(b) && typeof b.type === 'string' && ['thinking', 'tool_use', 'tool_result', 'redacted_thinking'].includes(b.type)));
}

/**
 * Parse a request body into a normalized conversation for preview.
 * Supports OpenAI chat/responses, Anthropic, Gemini, AI SDK and Ollama formats.
 */
export function parseRequestConversation(body: unknown, format?: string): ConversationData | null {
  if (!isRecord(body)) return null;

  const fmt = format || '';
  const messages: ConversationMessage[] = [];
  let tools: ConversationTool[] = [];

  // System instruction extraction (Anthropic / Gemini).
  let systemText = '';

  if (fmt.startsWith('anthropic') || isAnthropicShape(body.messages)) {
    if (typeof body.system === 'string') systemText = body.system;
    else if (Array.isArray(body.system)) {
      systemText = body.system
        .map((s) => (isRecord(s) ? stringify(s.text) : stringify(s)))
        .filter(Boolean)
        .join('');
    }
    if (systemText) messages.push({ index: 0, role: 'system', content: systemText, raw: { role: 'system', content: systemText } });

    if (Array.isArray(body.messages)) {
      body.messages.forEach((m) => {
        if (!isRecord(m)) return;
        const parsed = parseAnthropicContent(m.content);
        const role = m.role === 'assistant' ? 'assistant' : m.role === 'user' ? 'user' : m.role === 'system' ? 'system' : stringify(m.role) || 'user';
        messages.push({
          index: messages.length,
          role,
          content: parsed.text,
          contentParts: parsed.parts,
          reasoning: parsed.reasoning,
          toolCalls: parsed.toolCalls,
          raw: m,
        });
        if (parsed.toolResult) {
          messages.push({
            index: messages.length,
            role: 'tool',
            content: parsed.toolResult.content,
            toolCallId: parsed.toolResult.id,
            raw: m,
          });
        }
      });
    }
    tools = normalizeAnthropicTools(body.tools);
  } else if (fmt.startsWith('gemini') || Array.isArray(body.contents)) {
    if (isRecord(body.systemInstruction) && Array.isArray(body.systemInstruction.parts)) {
      systemText = body.systemInstruction.parts
        .map((p: unknown) => (isRecord(p) ? stringify(p.text) : ''))
        .filter(Boolean)
        .join('');
    }
    if (systemText) messages.push({ index: 0, role: 'system', content: systemText, raw: { role: 'system', content: systemText } });

    if (Array.isArray(body.contents)) {
      body.contents.forEach((c) => {
        if (!isRecord(c)) return;
        const parsed = parseGeminiContent(c.parts);
        const role = c.role === 'model' ? 'assistant' : c.role === 'user' ? 'user' : c.role === 'system' ? 'system' : stringify(c.role) || 'user';
        messages.push({
          index: messages.length,
          role,
          content: parsed.text,
          contentParts: parsed.partsRaw,
          reasoning: parsed.reasoning,
          toolCalls: parsed.toolCalls,
          raw: c,
        });
        if (parsed.toolResult) {
          messages.push({
            index: messages.length,
            role: 'tool',
            content: parsed.toolResult.content,
            toolCallId: parsed.toolResult.id,
            raw: c,
          });
        }
      });
    }
    tools = normalizeGeminiTools(body.tools);
  } else if (fmt.startsWith('aisdk') || (Array.isArray(body.messages) && isAiSdkShape(body.messages))) {
    if (typeof body.system === 'string' && body.system) {
      systemText = body.system;
      messages.push({ index: 0, role: 'system', content: systemText, raw: { role: 'system', content: systemText } });
    }
    if (Array.isArray(body.messages)) {
      body.messages.forEach((m) => {
        if (!isRecord(m)) return;
        const parsed = parseAiSdkContent(m.parts);
        const role = m.role === 'system' ? 'system' : m.role === 'user' ? 'user' : m.role === 'assistant' ? 'assistant' : stringify(m.role);
        if (parsed.text || parsed.reasoning || (parsed.toolCalls && parsed.toolCalls.length > 0) || role === 'system') {
          messages.push({
            index: messages.length,
            role,
            content: parsed.text,
            contentParts: parsed.partsRaw,
            reasoning: parsed.reasoning,
            toolCalls: parsed.toolCalls,
            raw: m,
          });
        }
        if (parsed.toolResults) {
          parsed.toolResults.forEach((tr) => {
            messages.push({
              index: messages.length,
              role: 'tool',
              content: tr.content,
              toolCallId: tr.id,
              raw: m,
            });
          });
        }
      });
    }
    tools = normalizeOpenAITools(body.tools);
  } else if (fmt.startsWith('openai/responses') || Array.isArray(body.input)) {
    const input = Array.isArray(body.input) ? body.input : [];
    input.forEach((item) => {
      if (!isRecord(item)) return;
      if (item.type === 'function_call') {
        messages.push({
          index: messages.length,
          role: 'assistant',
          content: '',
          toolCalls: [
            {
              id: stringify(item.call_id) || undefined,
              name: stringify(item.name),
              arguments: prettyJson(item.arguments),
            },
          ],
          raw: item,
        });
        return;
      }
      if (item.type === 'function_call_output') {
        messages.push({
          index: messages.length,
          role: 'tool',
          content: prettyJson(item.output),
          toolCallId: stringify(item.call_id),
          raw: item,
        });
        return;
      }
      const extracted = extractOpenAIContent(item.content);
      messages.push({
        index: messages.length,
        role: item.role === 'assistant' ? 'assistant' : item.role === 'user' ? 'user' : item.role === 'system' ? 'system' : stringify(item.role) || 'user',
        content: extracted.text,
        contentParts: extracted.parts,
        raw: item,
      });
    });
    tools = normalizeOpenAITools(body.tools);
  } else {
    // OpenAI chat completions / Ollama / generic messages[].
    if (Array.isArray(body.messages)) {
      body.messages.forEach((m) => {
        if (!isRecord(m)) return;
        const extracted = extractOpenAIContent(m.content);
        messages.push({
          index: messages.length,
          role: stringify(m.role) || 'user',
          content: extracted.text,
          contentParts: extracted.parts,
          reasoning: typeof m.reasoning_content === 'string' ? m.reasoning_content : undefined,
          toolCalls: normalizeToolCalls(m.tool_calls),
          toolCallId: typeof m.tool_call_id === 'string' ? m.tool_call_id : undefined,
          raw: m,
        });
      });
    }
    tools = normalizeOpenAITools(body.tools);
  }

  if (messages.length === 0) return null;

  return {
    model: typeof body.model === 'string' ? body.model : undefined,
    maxTokens: typeof body.max_tokens === 'number' ? body.max_tokens : typeof body.maxTokens === 'number' ? body.maxTokens : undefined,
    stream: typeof body.stream === 'boolean' ? body.stream : undefined,
    toolChoice: body.tool_choice,
    messages,
    tools,
  };
}
