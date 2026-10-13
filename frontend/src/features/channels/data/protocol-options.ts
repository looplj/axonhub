import type { ApiFormat, ChannelEndpoint, ChannelType, ModelProtocol } from './schema';

type ProtocolEndpoint = {
  readonly apiFormat?: string | null;
};

export function getAvailableProtocolFormats(
  defaultEndpoints: readonly ProtocolEndpoint[],
  endpoints: readonly ProtocolEndpoint[]
): string[] {
  const formats = new Set<string>();

  for (const endpoint of [...defaultEndpoints, ...endpoints]) {
    if (typeof endpoint.apiFormat === 'string' && endpoint.apiFormat.length > 0) {
      formats.add(endpoint.apiFormat);
    }
  }

  return Array.from(formats);
}

type ProviderConfig = {
  readonly channelTypes: readonly ChannelType[];
};

type ChannelConfig = {
  readonly apiFormat?: ApiFormat | null;
};

export type ProtocolConfigs = {
  readonly providerConfigs: Readonly<Record<string, ProviderConfig>>;
  readonly channelConfigs: Readonly<Record<string, ChannelConfig>>;
};

export function getApiFormatsForProvider(provider: string, configs: ProtocolConfigs): ApiFormat[] {
  const providerConfig = configs.providerConfigs[provider];
  if (!providerConfig) return [];

  const formats: ApiFormat[] = [];
  for (const channelType of providerConfig.channelTypes) {
    const apiFormat = configs.channelConfigs[channelType]?.apiFormat;
    if (apiFormat && !formats.includes(apiFormat)) {
      formats.push(apiFormat);
    }
  }

  return formats;
}

/**
 * Custom endpoint formats the endpoints dialog may offer for a channel.
 * Native video is available as a custom endpoint on every ZenMux channel type.
 * The OpenAI Decisions endpoint is opt-in for the OpenAI channel only; the
 * backend still accepts manual configuration for other channel types.
 * Codex Responses endpoints are managed by the WebSocket checkbox instead.
 * Other custom endpoint formats remain available everywhere.
 */
export function getConfigurableApiFormatsForChannelType(
  channelType: ChannelType,
  configurableFormats: readonly string[]
): string[] {
  if (channelType === 'typesafe') {
    return ['typesafe/systemone'];
  }
  const filtered = configurableFormats
    .filter((format) => format !== 'typesafe/systemone')
    .filter((format) => channelType === 'openai' || format !== 'openai/decisions')
    .filter((format) => channelType !== 'codex' || !isResponsesFamilyFormat(format));
  if (['zenmux', 'zenmux_responses', 'zenmux_anthropic', 'zenmux_gemini', 'zenmux_video'].includes(channelType)) {
    return [...filtered];
  }
  return filtered.filter((format) => format !== 'zenmux/video');
}

export function getChannelTypeForApiFormat(provider: string, apiFormat: ApiFormat, configs: ProtocolConfigs): ChannelType | undefined {
  const providerConfig = configs.providerConfigs[provider];
  if (!providerConfig) return undefined;

  // Native video is a ZenMux channel type, so it is only mappable back to a
  // channel type when the provider config actually includes the ZenMux
  // channel type.
  if (apiFormat === 'zenmux/video') {
    return providerConfig.channelTypes.includes('zenmux_video') ? 'zenmux_video' : undefined;
  }

  for (const channelType of providerConfig.channelTypes) {
    if (configs.channelConfigs[channelType]?.apiFormat === apiFormat) {
      return channelType;
    }
  }

  return undefined;
}

export function getModelProtocolsForApiFormat(
  apiFormat: ApiFormat,
  models: readonly string[],
  existingProtocols: readonly ModelProtocol[] | null | undefined = []
): ModelProtocol[] {
  if (apiFormat !== 'zenmux/video') {
    return (existingProtocols ?? [])
      .map((protocol) => ({
        ...protocol,
        apiFormats: protocol.apiFormats.filter((format) => format !== 'zenmux/video'),
      }))
      .filter((protocol) => protocol.apiFormats.length > 0);
  }

  // Native video selection must not clobber per-model protocol overrides:
  // preserve any existing explicit selection for a model and only assign the
  // video format to models the user has not pinned to a protocol yet.
  const existingByModel = new Map((existingProtocols ?? []).map((protocol) => [protocol.model, protocol]));
  return models.map((model) => {
    const existing = existingByModel.get(model);
    if (existing) {
      return { ...existing };
    }
    return { model, apiFormats: [apiFormat], enabled: true };
  });
}

export function getInitialApiFormatForChannel(
  channelType: ChannelType,
  defaultApiFormat: ApiFormat,
  modelProtocols: readonly ModelProtocol[] | null | undefined
): ApiFormat {
  if (
    channelType === 'zenmux' &&
    modelProtocols?.some((protocol) => protocol.enabled !== false && protocol.apiFormats.includes('zenmux/video'))
  ) {
    return 'zenmux/video';
  }

  return defaultApiFormat;
}


const OPENAI_RESPONSES_FORMAT = 'openai/responses';
const OPENAI_RESPONSES_WEBSOCKET_FORMAT = 'openai/responses-ws';

type CodexResponsesChannel = {
  readonly baseURL?: string | null;
  readonly endpoints?: readonly ChannelEndpoint[] | null;
};

export function isResponsesFamilyFormat(apiFormat: string): boolean {
  return apiFormat === OPENAI_RESPONSES_FORMAT || apiFormat === OPENAI_RESPONSES_WEBSOCKET_FORMAT;
}

function isHttpURL(url: string | null | undefined): url is string {
  return /^https?:\/\//i.test(url?.trim() ?? '');
}

function isWebSocketURL(url: string | null | undefined): boolean {
  return /^wss?:\/\//i.test(url?.trim() ?? '');
}

// Mirrors the backend NormalizeEndpoints: legacy explicit transport wins over
// the scheme inherited from the endpoint or channel Base URL.
function resolveResponsesFormat(endpoint: ChannelEndpoint, channelBaseURL: string | null | undefined): string {
  if (endpoint.apiFormat !== OPENAI_RESPONSES_FORMAT) return endpoint.apiFormat;
  if (endpoint.transport === 'websocket') return OPENAI_RESPONSES_WEBSOCKET_FORMAT;
  if (endpoint.transport === 'http') return OPENAI_RESPONSES_FORMAT;
  return isWebSocketURL(endpoint.baseURL || channelBaseURL) ? OPENAI_RESPONSES_WEBSOCKET_FORMAT : OPENAI_RESPONSES_FORMAT;
}

// The HTTP Responses endpoint's path and HTTP(S) base overrides are shared by
// the WebSocket endpoint, so its URL is derived from the same HTTP endpoint.
// Legacy WebSocket addresses are never reused.
function getSharedResponsesOverrides(endpoints: readonly ChannelEndpoint[] | null | undefined): Pick<ChannelEndpoint, 'path' | 'baseURL'> {
  const http = endpoints?.find((endpoint) => endpoint.apiFormat === OPENAI_RESPONSES_FORMAT);
  return {
    ...(http?.path ? { path: http.path } : {}),
    ...(isHttpURL(http?.baseURL) ? { baseURL: http.baseURL } : {}),
  };
}

/**
 * Projects a Codex channel's persisted Responses endpoints onto the form's
 * WebSocket checkbox. New channels default to HTTP + WebSocket. A WebSocket-only
 * channel reads as checked; saving the form upgrades it to HTTP + WebSocket.
 */
export function getCodexResponsesWebSocketState(channel?: CodexResponsesChannel | null): { enabled: boolean; webSocketOnly: boolean } {
  if (!channel) return { enabled: true, webSocketOnly: false };

  const formats = new Set(
    (channel.endpoints ?? [])
      .filter((endpoint) => isResponsesFamilyFormat(endpoint.apiFormat))
      .map((endpoint) => resolveResponsesFormat(endpoint, channel.baseURL))
  );
  if (formats.size === 0) {
    // Without family entries the backend defaults to HTTP + WebSocket, or to
    // WebSocket only for a legacy WS(S) channel Base URL.
    return { enabled: true, webSocketOnly: isWebSocketURL(channel.baseURL) };
  }

  const webSocket = formats.has(OPENAI_RESPONSES_WEBSOCKET_FORMAT);
  return { enabled: webSocket, webSocketOnly: webSocket && !formats.has(OPENAI_RESPONSES_FORMAT) };
}

export function hasCodexResponsesWebSocketOverrideReplacement(channel?: CodexResponsesChannel | null): boolean {
  if (!channel) return false;

  const http = channel.endpoints?.find((endpoint) => resolveResponsesFormat(endpoint, channel.baseURL) === OPENAI_RESPONSES_FORMAT);
  const webSocket = channel.endpoints?.find((endpoint) => resolveResponsesFormat(endpoint, channel.baseURL) === OPENAI_RESPONSES_WEBSOCKET_FORMAT);
  if (!http || !webSocket) return false;

  return http.baseURL !== webSocket.baseURL || http.path !== webSocket.path;
}

/**
 * The Responses formats a channel effectively exposes, for display. Defaults are
 * already resolved by the backend; configured legacy entries are read in their
 * resolved transport identity.
 */
export function getEffectiveResponsesFormats(
  channel: CodexResponsesChannel & { readonly defaultEndpoints?: readonly ChannelEndpoint[] | null }
): string[] {
  const formats = new Set([
    ...(channel.defaultEndpoints ?? []).map((endpoint) => endpoint.apiFormat),
    ...(channel.endpoints ?? []).map((endpoint) => resolveResponsesFormat(endpoint, channel.baseURL)),
  ]);
  return [OPENAI_RESPONSES_FORMAT, OPENAI_RESPONSES_WEBSOCKET_FORMAT].filter((format) => formats.has(format));
}

/**
 * Canonical Codex endpoints for the WebSocket checkbox: explicit HTTP Responses,
 * plus WebSocket Responses when checked, without the legacy transport field.
 * Unrelated endpoints such as Images keep their position and configuration.
 */
export function buildCodexResponsesEndpoints(
  endpoints: readonly ChannelEndpoint[] | null | undefined,
  webSocketEnabled: boolean
): ChannelEndpoint[] {
  const shared = getSharedResponsesOverrides(endpoints);
  const family: ChannelEndpoint[] = [
    { apiFormat: OPENAI_RESPONSES_FORMAT, ...shared },
    ...(webSocketEnabled ? [{ apiFormat: OPENAI_RESPONSES_WEBSOCKET_FORMAT, ...shared }] : []),
  ];

  let placed = false;
  const result = (endpoints ?? []).flatMap((endpoint) => {
    if (!isResponsesFamilyFormat(endpoint.apiFormat)) return [endpoint];
    if (placed) return [];
    placed = true;
    return family;
  });
  return placed ? result : [...result, ...family];
}

/** Legacy WS(S) Base URLs are shown and saved as their HTTP(S) equivalent. */
export function toHttpBaseURL(baseURL: string): string {
  return baseURL.replace(/^ws(s?):\/\//i, (_match, secure: string) => `http${secure.toLowerCase()}://`);
}

// Mirrors the backend Responses URL rules: a trailing ## is a raw URL, a
// trailing # suppresses the /v1 suffix, and a path override skips it.
function buildResponsesHTTPURL(baseURL: string, endpointPath: string | undefined): string {
  if (baseURL.endsWith('##')) return baseURL.slice(0, -2);
  if (baseURL.endsWith('#')) return baseURL.slice(0, -1).replace(/\/+$/, '') + (endpointPath || '/responses');

  const base = baseURL.replace(/\/+$/, '');
  if (endpointPath) return base + endpointPath;
  const hasVersion = baseURL.endsWith('/v1') || baseURL.includes('/v1/');
  return `${hasVersion ? base : `${base}/v1`}/responses`;
}

/**
 * The WebSocket Responses URL the Codex channel uses: the resolved HTTP(S)
 * Responses endpoint with a WS(S) scheme, keeping host, port and path. The
 * fragment marker is never part of the wire URL.
 */
export function getCodexResponsesWebSocketURL(
  baseURL: string | null | undefined,
  endpoints?: readonly ChannelEndpoint[] | null
): string | undefined {
  const shared = getSharedResponsesOverrides(endpoints);
  const httpBaseURL = (shared.baseURL ?? baseURL ?? '').trim();
  if (!isHttpURL(httpBaseURL)) return undefined;

  try {
    const url = new URL(buildResponsesHTTPURL(httpBaseURL, shared.path));
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    url.hash = '';
    return url.toString();
  } catch {
    return undefined;
  }
}

// Mirrors the backend legacy identity: what `openai/responses` resolved to
// in the stored configuration before the form saves canonical endpoints.
function getLegacyResponsesFormat(channel: CodexResponsesChannel): string {
  const endpoints = channel.endpoints ?? [];
  const http = endpoints.find((endpoint) => endpoint.apiFormat === OPENAI_RESPONSES_FORMAT);
  if (http) return resolveResponsesFormat(http, channel.baseURL);
  if (endpoints.some((endpoint) => endpoint.apiFormat === OPENAI_RESPONSES_WEBSOCKET_FORMAT)) return OPENAI_RESPONSES_FORMAT;
  return resolveResponsesFormat({ apiFormat: OPENAI_RESPONSES_FORMAT }, channel.baseURL);
}

/**
 * Saving canonical endpoints changes what `openai/responses` means for a legacy
 * WebSocket-only channel, so its model restrictions move to the WebSocket
 * format (mirrors the backend legacy identity). Returns undefined when nothing
 * changes.
 */
export function translateLegacyCodexModelProtocols(
  channel: CodexResponsesChannel,
  modelProtocols: readonly ModelProtocol[] | null | undefined
): ModelProtocol[] | undefined {
  if (
    !modelProtocols?.some((protocol) => protocol.apiFormats.includes(OPENAI_RESPONSES_FORMAT)) ||
    getLegacyResponsesFormat(channel) !== OPENAI_RESPONSES_WEBSOCKET_FORMAT
  ) {
    return undefined;
  }

  return modelProtocols.map((protocol) => ({
    ...protocol,
    apiFormats: Array.from(
      new Set(protocol.apiFormats.map((format) => (format === OPENAI_RESPONSES_FORMAT ? OPENAI_RESPONSES_WEBSOCKET_FORMAT : format)))
    ),
  }));
}
