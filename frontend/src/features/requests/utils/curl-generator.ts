import { CHANNEL_CONFIGS } from '@/features/channels/data/config_channels';
import { ApiFormat } from '@/features/channels/data/schema';

export type ChannelType = keyof typeof CHANNEL_CONFIGS;

export interface CurlGeneratorOptions {
  headers?: Record<string, any>;
  body?: any;
  baseUrl?: string;
  requestURL?: string;
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

function resolveExecutionURL(options: CurlGeneratorOptions, apiFormat?: ApiFormat, body?: unknown, channelType?: ChannelType): string {
  if (options.requestURL) {
    return options.requestURL;
  }

  if (options.baseUrl?.endsWith('##')) {
    return options.baseUrl.slice(0, -2).replace(/\/+$/, '');
  }

  const apiPath = getApiPath(apiFormat, body, channelType);

  if (options.baseUrl) {
    const cleanBaseUrl = options.baseUrl.replace(/\/+$/, '');
    // Avoid path duplication: if baseUrl ends with a prefix of apiPath, strip the overlap.
    let combinedPath = apiPath;
    for (let i = 1; i <= apiPath.length; i++) {
      const prefix = apiPath.substring(0, i);
      if (cleanBaseUrl.endsWith(prefix)) {
        combinedPath = apiPath.substring(i);
      }
    }
    return `${cleanBaseUrl}${combinedPath}`;
  }

  return `${typeof window !== 'undefined' ? window.location.origin : ''}${apiPath}`;
}

export function generateCurlCommand(options: CurlGeneratorOptions): string {
  const { headers, body, baseUrl, requestURL, apiFormat, channelType } = options;

  const parsedBody = typeof body === 'string' ? safeJsonParse(body) : body;
  const resolvedApiFormat = inferApiFormat(apiFormat || getApiFormatFromChannelType(channelType), parsedBody);
  const url = resolveExecutionURL({ baseUrl, requestURL }, resolvedApiFormat, parsedBody, channelType);

  const curlParts = [`curl '${url}'`];

  // Audio transcription/translation use multipart/form-data, not JSON.
  const isMultipartAudio = resolvedApiFormat === 'openai/audio_transcriptions' || resolvedApiFormat === 'openai/audio_translations';
  const isMultipartImage = resolvedApiFormat === 'openai/image_edit' || resolvedApiFormat === 'openai/image_variation';

  if (headers && typeof headers === 'object') {
    const skipHeaders = ['content-length', 'host', 'connection', 'accept-encoding', 'transfer-encoding'];
    // For multipart, curl -F generates its own Content-Type with a fresh boundary;
    // the logged header carries a stale boundary and must be dropped.
    if (isMultipartAudio || isMultipartImage) {
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
    if (isRecord(parsedBody)) {
      Object.entries(parsedBody).forEach(([key, value]) => {
        if (key === 'file') {
          curlParts.push(`  -F 'file=@/path/to/audio.mp3'`);
          return;
        }
        const values = Array.isArray(value) ? value : [value];
        values.forEach((v) => {
          const fieldValue = escapeShellValue(formatFormValue(v));
          curlParts.push(`  -F '${key}=${fieldValue}'`);
        });
      });
    }
  } else if (body && isMultipartImage) {
    appendImageFormParts(curlParts, parsedBody, resolvedApiFormat);
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

function inferApiFormat(apiFormat: ApiFormat | undefined, body: unknown): ApiFormat | undefined {
  if (!isImageMultipartBody(body)) {
    return apiFormat;
  }

  if (apiFormat && apiFormat !== 'openai/chat_completions' && apiFormat !== 'openai/responses') {
    return apiFormat;
  }

  return isRecord(body) && typeof body.prompt === 'string' && body.prompt.trim() !== '' ? 'openai/image_edit' : 'openai/image_variation';
}

function isImageMultipartBody(body: unknown): boolean {
  if (!isRecord(body)) {
    return false;
  }

  return 'formFiles' in body || 'image' in body || 'mask' in body;
}

function appendImageFormParts(curlParts: string[], body: unknown, apiFormat: ApiFormat): void {
  if (!isRecord(body)) {
    return;
  }

  const imageCount = getImageCount(body);
  const imageField = apiFormat === 'openai/image_edit' && imageCount > 1 ? 'image[]' : 'image';

  appendImageFileParts(curlParts, imageField, body);

  if ('mask' in body) {
    curlParts.push(`  -F 'mask=@/path/to/mask.png'`);
  }

  Object.entries(body).forEach(([key, value]) => {
    if (key === 'formFiles' || key === 'image' || key === 'mask' || value == null || value === '') {
      return;
    }

    const values = Array.isArray(value) ? value : [value];
    values.forEach((v) => {
      const fieldValue = escapeShellValue(formatFormValue(v));
      curlParts.push(`  -F '${key}=${fieldValue}'`);
    });
  });
}

function appendImageFileParts(curlParts: string[], imageField: string, body: Record<string, unknown>): void {
  if (Array.isArray(body.formFiles)) {
    body.formFiles.forEach((file, index) => {
      const filename =
        isRecord(file) && typeof file.filename === 'string' && file.filename !== '' ? file.filename : `image_${index + 1}.png`;
      curlParts.push(`  -F '${imageField}=@${escapeShellValue(`/path/to/${filename}`)}'`);
    });
    return;
  }

  if (Array.isArray(body.image)) {
    body.image.forEach((_, index) => {
      curlParts.push(`  -F '${imageField}=@/path/to/image_${index + 1}.png'`);
    });
    return;
  }

  if ('image' in body) {
    curlParts.push(`  -F '${imageField}=@/path/to/image.png'`);
  }
}

function getImageCount(body: Record<string, unknown>): number {
  if (Array.isArray(body.formFiles)) {
    return body.formFiles.length;
  }

  if (Array.isArray(body.image)) {
    return body.image.length;
  }

  return 'image' in body ? 1 : 0;
}

function formatFormValue(value: unknown): string {
  if (typeof value === 'string') {
    return value;
  }

  return JSON.stringify(value) ?? String(value);
}

function escapeShellValue(value: string): string {
  return value.replace(/'/g, "'\\''");
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
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
  apiFormat?: ApiFormat,
  requestURL?: string
): string {
  return generateCurlCommand({
    headers,
    body,
    baseUrl: channel?.baseURL,
    channelType: channel?.type,
    apiFormat,
    requestURL,
  });
}
