import type { TFunction } from 'i18next';
import type { Span } from '../data/schema';

const DEFAULT_SPAN_TYPE_KEY = 'default';

const spanTypeTranslationKeyMap: Record<string, string> = {
  user_query: 'userQuery',
  userquery: 'userQuery',
  user_image_url: 'userImageUrl',
  userimageurl: 'userImageUrl',
  text: 'text',
  thinking: 'thinking',
  image_url: 'imageUrl',
  imageurl: 'imageUrl',
  function_call: 'functionCall',
  functioncall: 'functionCall',
  function_result: 'functionResult',
  functionresult: 'functionResult',
  tool_use: 'toolUse',
  tooluse: 'toolUse',
  tool_result: 'toolResult',
  toolresult: 'toolResult',
  message: 'message',
  retrieve: 'retrieve',
  embedding: 'embedding',
  synthesize: 'synthesize',
  chunking: 'chunking',
  templating: 'templating',
  llm: 'llm',
  system_instruction: 'systemInstruction',
  systeminstruction: 'systemInstruction',
};

function createFallbackLabel(type?: string | null): string {
  if (!type) {
    return 'Span';
  }

  return type
    .split(/[_-]+/g)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');
}

export function normalizeSpanType(type?: string | null): string {
  if (!type) {
    return DEFAULT_SPAN_TYPE_KEY;
  }

  return type
    .trim()
    .toLowerCase()
    .replace(/[-\s]+/g, '_');
}

export function getSpanTypeTranslationKey(type?: string | null): string {
  const normalized = normalizeSpanType(type);
  return spanTypeTranslationKeyMap[normalized] ?? spanTypeTranslationKeyMap[normalized.replace(/_/g, '')] ?? DEFAULT_SPAN_TYPE_KEY;
}

export function getLocalizedSpanType(type: string | null | undefined, t: TFunction): string {
  const key = getSpanTypeTranslationKey(type);
  return t(`traces.timeline.spanTypes.${key}`, createFallbackLabel(type));
}

export function getSpanDisplayLabels(span: Span, t: TFunction): { primary: string; secondary?: string } {
  const typeLabel = getLocalizedSpanType(span.type, t);
  const normalizedType = normalizeSpanType(span.type);

  if (normalizedType === 'tool_use') {
    const toolName = span.value?.toolUse?.name;
    if (toolName) {
      return { primary: toolName, secondary: typeLabel };
    }
  }

  if (normalizedType === 'tool_result') {
    const resultId = span.value?.toolResult?.toolCallID;
    if (resultId) {
      return { primary: resultId, secondary: typeLabel };
    }
  }
  return { primary: typeLabel };
}
