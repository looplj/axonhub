const API_FORMAT_PATHS: Record<string, string> = {
  'openai/chat_completions': '/v1/chat/completions',
  'openai/responses': '/v1/responses',
  'openai/responses-ws': '/v1/responses',
  'openai/image_generation': '/v1/images/generations',
  'openai/image_edit': '/v1/images/edits',
  'openai/image_variation': '/v1/images/variations',
  'openai/embeddings': '/v1/embeddings',
  'openai/moderations': '/v1/moderations',
  'openai/alpha_search': '/v1/alpha/search',
  'openai/video': '/v1/videos',
  'zenmux/video': '/v1/videos',
  'seedance/video': '/api/v3/contents/generations/tasks',
  'openai/audio_speech': '/v1/audio/speech',
  'openai/audio_transcriptions': '/v1/audio/transcriptions',
  'openai/audio_translations': '/v1/audio/translations',
  'anthropic/messages': '/v1/messages',
  'gemini/contents': '/v1beta/models/{model}:generateContent',
  'aisdk/text': '/api/chat',
  'aisdk/datastream': '/api/datastream',
  'jina/rerank': '/v1/rerank',
  'jina/embeddings': '/jina/v1/embeddings',
};

export function getApiPath(apiFormat?: string, body?: unknown, channelType?: string): string {
  if (!apiFormat) {
    return '/v1/chat/completions';
  }

  let path = API_FORMAT_PATHS[apiFormat] || '/v1/chat/completions';

  if (apiFormat === 'gemini/contents' && isRecord(body) && typeof body.model === 'string') {
    if (channelType === 'gemini_vertex') {
      path = '/v1/publishers/google/models/{model}:generateContent';
    }
    path = path.replace('{model}', body.model);
  }

  return path;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
