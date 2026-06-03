import { ApiFormat } from '@/features/channels/data/schema';
import { CHANNEL_CONFIGS } from '@/features/channels/data/config_channels';

export type ChannelType = keyof typeof CHANNEL_CONFIGS;

export interface CurlGeneratorOptions {
  headers?: Record<string, any>;
  body?: any;
  baseUrl?: string;
  apiFormat?: ApiFormat;
  channelType?: ChannelType;
}

const API_FORMAT_PATHS: Record<ApiFormat, string> = {
  'openai/chat_completions': '/v1/chat/completions',
  'openai/responses': '/v1/responses',
  'openai/image_generation': '/v1/images/generations',
  'openai/image_edit': '/v1/images/edits',
  'openai/image_variation': '/v1/images/variations',
  'openai/embeddings': '/v1/embeddings',
  'openai/video': '/v1/videos',
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

function getApiPath(apiFormat?: ApiFormat, body?: any, channelType?: ChannelType): string {
  if (!apiFormat) {
    return '/v1/chat/completions';
  }

  let path = API_FORMAT_PATHS[apiFormat] || '/v1/chat/completions';

  if (apiFormat === 'gemini/contents' && body?.model) {
    if (channelType === 'gemini_vertex') {
      path = '/v1/publishers/google/models/{model}:generateContent';
    }
    path = path.replace('{model}', body.model);
  }

  return path;
}

function getApiFormatFromChannelType(channelType?: ChannelType): ApiFormat | undefined {
  if (!channelType) return undefined;
  return CHANNEL_CONFIGS[channelType]?.apiFormat;
}

export function generateCurlCommand(options: CurlGeneratorOptions): string {
  const { headers, body, baseUrl, apiFormat, channelType } = options;

  const resolvedApiFormat = apiFormat || getApiFormatFromChannelType(channelType);
  const apiPath = getApiPath(resolvedApiFormat, body, channelType);

  let url: string;
  if (baseUrl) {
    const cleanBaseUrl = baseUrl.replace(/\/+$/, '');
    // Avoid path duplication: if baseUrl ends with a prefix of apiPath, strip the overlap.
    // e.g. baseUrl="https://api.openai.com/v1" + apiPath="/v1/chat/completions"
    //   -> "https://api.openai.com/v1/chat/completions" (not .../v1/v1/chat/completions)
    let combinedPath = apiPath;
    for (let i = 1; i <= apiPath.length; i++) {
      const prefix = apiPath.substring(0, i);
      if (cleanBaseUrl.endsWith(prefix)) {
        combinedPath = apiPath.substring(i);
      }
    }
    url = `${cleanBaseUrl}${combinedPath}`;
  } else {
    url = `${typeof window !== 'undefined' ? window.location.origin : ''}${apiPath}`;
  }

  const curlParts = [`curl '${url}'`];

  // Audio transcription/translation use multipart/form-data, not JSON.
  const isMultipartAudio =
    resolvedApiFormat === 'openai/audio_transcriptions' || resolvedApiFormat === 'openai/audio_translations';

  if (headers && typeof headers === 'object') {
    const skipHeaders = ['content-length', 'host', 'connection', 'accept-encoding', 'transfer-encoding'];
    // For multipart, curl -F generates its own Content-Type with a fresh boundary;
    // the logged header carries a stale boundary and must be dropped.
    if (isMultipartAudio) {
      skipHeaders.push('content-type');
    }
    Object.entries(headers).forEach(([key, value]) => {
      if (!skipHeaders.includes(key.toLowerCase()) && value) {
        const headerValue = String(value).replace(/'/g, "'\\''");
        curlParts.push(`  -H '${key}: ${headerValue}'`);
      }
    });
  }

  if (body && isMultipartAudio) {
    // The logged body replaces the binary file with a placeholder; emit -F flags
    // so the generated cURL is reproducible (user supplies a local file path).
    const parsed = typeof body === 'string' ? safeJsonParse(body) : body;
    if (parsed && typeof parsed === 'object') {
      Object.entries(parsed).forEach(([key, value]) => {
        if (key === 'file') {
          curlParts.push(`  -F 'file=@/path/to/audio.mp3'`);
          return;
        }
        const values = Array.isArray(value) ? value : [value];
        values.forEach((v) => {
          const fieldValue = String(v).replace(/'/g, "'\\''");
          curlParts.push(`  -F '${key}=${fieldValue}'`);
        });
      });
    }
  } else if (body) {
    const bodyStr = typeof body === 'string' ? body : JSON.stringify(body);
    const escapedBody = bodyStr.replace(/'/g, "'\\''");
    curlParts.push(`  -d '${escapedBody}'`);
  }

  return curlParts.join(' \\\n');
}

function safeJsonParse(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    return undefined;
  }
}

export function generateRequestCurl(headers: any, body: any, apiFormat?: ApiFormat): string {
  return generateCurlCommand({
    headers,
    body,
    apiFormat: apiFormat || 'openai/chat_completions',
  });
}

export function generateExecutionCurl(
  headers: any,
  body: any,
  channel?: { baseURL?: string; type?: ChannelType },
  apiFormat?: ApiFormat
): string {
  return generateCurlCommand({
    headers,
    body,
    baseUrl: channel?.baseURL,
    channelType: channel?.type,
    apiFormat,
  });
}
