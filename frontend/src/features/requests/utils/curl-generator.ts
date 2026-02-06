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
  'anthropic/messages': '/v1/messages',
  'gemini/contents': '/v1beta/models/{model}:generateContent',
  'aisdk/text': '/api/chat',
  'aisdk/datastream': '/api/datastream',
  'jina/rerank': '/v1/rerank',
  'jina/embeddings': '/jina/v1/embeddings',
};

function getApiPath(apiFormat?: ApiFormat, body?: any): string {
  if (!apiFormat) {
    return '/v1/chat/completions';
  }

  let path = API_FORMAT_PATHS[apiFormat] || '/v1/chat/completions';

  if (apiFormat === 'gemini/contents' && body?.model) {
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
  const apiPath = getApiPath(resolvedApiFormat, body);

  let url: string;
  if (baseUrl) {
    const cleanBaseUrl = baseUrl.replace(/\/+$/, '');
    url = `${cleanBaseUrl}${apiPath}`;
  } else {
    url = `${typeof window !== 'undefined' ? window.location.origin : ''}${apiPath}`;
  }

  const curlParts = [`curl '${url}'`];

  if (headers && typeof headers === 'object') {
    const skipHeaders = ['content-length', 'host', 'connection', 'accept-encoding', 'transfer-encoding'];
    Object.entries(headers).forEach(([key, value]) => {
      if (!skipHeaders.includes(key.toLowerCase()) && value) {
        const headerValue = String(value).replace(/'/g, "'\\''");
        curlParts.push(`  -H '${key}: ${headerValue}'`);
      }
    });
  }

  if (body) {
    const bodyStr = typeof body === 'string' ? body : JSON.stringify(body);
    const escapedBody = bodyStr.replace(/'/g, "'\\''");
    curlParts.push(`  -d '${escapedBody}'`);
  }

  return curlParts.join(' \\\n');
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
