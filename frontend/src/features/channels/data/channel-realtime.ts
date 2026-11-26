// 职责: 将渠道协议选择转换成渠道独立的 Realtime 配置并验证可信 WS 地址。
// 关联文件:
//   frontend/src/features/channels/components/channels-action-dialog.tsx
//   frontend/src/features/channels/components/channels-endpoints-dialog.tsx
//   frontend/src/features/channels/data/schema.ts
// 注意事项: Realtime 不写入 endpoints；HTTP Base URL 与供应商凭证保持独立。
import type { ChannelEndpoint, ChannelRealtime, ModelProtocol } from './schema';

const realtimeQuickModels: Readonly<Record<string, readonly string[]>> = {
  'openai/realtime-ws': ['gpt-realtime-2.1', 'gpt-realtime-2.1-mini', 'gpt-realtime-translate'],
  'qwen/realtime-ws': ['qwen3.8-omni-flash-realtime', 'qwen3.5-omni-plus-realtime', 'qwen3.5-omni-flash-realtime'],
};

const REALTIME_PROVIDER_BADGE_CLASSES: Readonly<Record<'openai/realtime-ws' | 'qwen/realtime-ws', string>> = {
  'openai/realtime-ws': 'border-teal-500/30 bg-teal-500/15 text-teal-700 dark:text-teal-300',
  'qwen/realtime-ws': 'border-fuchsia-500/30 bg-fuchsia-500/15 text-fuchsia-700 dark:text-fuchsia-300',
};

export function getRealtimeQuickModels(format: string): readonly string[] | undefined {
  return Object.hasOwn(realtimeQuickModels, format) ? realtimeQuickModels[format] : undefined;
}

export function isRealtimeQuickModel(model: string): boolean {
  return Object.values(realtimeQuickModels).some((models) => models.includes(model));
}

export const OPENAI_REALTIME_HOST = 'wss://api.openai.com';
export const DEFAULT_REALTIME_HANDSHAKE_TIMEOUT_MS = 15000;
const qwenRealtimeHostPattern = /^wss:\/\/[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.(?:cn-beijing|ap-southeast-1)\.maas\.aliyuncs\.com\/?$/i;

export function getRealtimeHandshakeTimeoutInputValue(timeoutMs: number | null | undefined): string {
  return timeoutMs == null ? '' : String(timeoutMs);
}

export function getRealtimeClientEventsPerSecondInputValue(limit: number | null | undefined): string {
  return limit == null ? '' : String(limit);
}

export function getChannelBaseURLInputValue(format: string, channelBaseURL: string | null | undefined, qwenRealtimeHost: string): string {
  if (format === 'openai/realtime-ws') return qwenRealtimeHost && !qwenRealtimeHostPattern.test(qwenRealtimeHost) ? qwenRealtimeHost : OPENAI_REALTIME_HOST;
  if (format === 'qwen/realtime-ws') return qwenRealtimeHost;
  return channelBaseURL ?? '';
}

export function getRealtimeDefaultEndpoint(realtime: ChannelRealtime | null | undefined): ChannelEndpoint | undefined {
  if (!realtime?.enabled || (realtime.protocol !== 'openai/realtime-ws' && realtime.protocol !== 'qwen/realtime-ws')) return undefined;
  return { apiFormat: realtime.protocol };
}

export function getModelFetchBaseURL(format: string, channelBaseURL: string | null | undefined, qwenRealtimeHost: string): string {
  if (format !== 'openai/realtime-ws' && format !== 'qwen/realtime-ws') return channelBaseURL ?? '';
  const realtimeBaseURL = getChannelBaseURLInputValue(format, channelBaseURL, qwenRealtimeHost);
  if (realtimeBaseURL.includes('<workspace-id>')) return '';
  return realtimeBaseURL.replace(/^wss:/i, 'https:');
}

export function getRealtimeProviderBadgeClass(realtime: Pick<ChannelRealtime, 'enabled' | 'protocol'> | null | undefined): string | undefined {
  if (!realtime?.enabled) return undefined;
  return realtime.protocol === 'openai/realtime-ws' || realtime.protocol === 'qwen/realtime-ws'
    ? REALTIME_PROVIDER_BADGE_CLASSES[realtime.protocol]
    : undefined;
}

export function isRealtimeProtocol(format: string): boolean {
  return format === 'openai/realtime-ws' || format === 'qwen/realtime-ws' || format === 'openai/realtime';
}

export function hasLegacyRealtimeEndpoints(endpoints: readonly Pick<ChannelEndpoint, 'apiFormat'>[]): boolean {
  return endpoints.some((endpoint) => isRealtimeProtocol(endpoint.apiFormat));
}

export function withoutRealtimeModelProtocols(protocols: readonly ModelProtocol[] | null | undefined): ModelProtocol[] {
  return (protocols ?? []).map((protocol) => ({ ...protocol, apiFormats: protocol.apiFormats.filter((format) => !isRealtimeProtocol(format)) })).filter((protocol) => protocol.apiFormats.length > 0);
}

export function realtimeForSelection(format: string, current: ChannelRealtime | null | undefined, host: string, timeout: string, clientEventsPerSecond?: string): ChannelRealtime | undefined {
  if (!isRealtimeProtocol(format)) return current ? { ...current, enabled: false } : undefined;
  if (format === 'openai/realtime') throw new Error('channels.realtime.invalidHost');
  const openai = format === 'openai/realtime-ws';
  const baseURL = (host.trim() || (openai ? OPENAI_REALTIME_HOST : '')).replace(/\/$/, '');
  const parsedHost = (() => {
    try { return new URL(baseURL); } catch { return null; }
  })();
  const ipLiteral = parsedHost && (parsedHost.hostname.includes(':') || /^\d{1,3}(?:\.\d{1,3}){3}$/.test(parsedHost.hostname));
  const validWebSocketURL = parsedHost && parsedHost.protocol === 'wss:' && parsedHost.hostname !== '' && !ipLiteral && parsedHost.username === '' && !baseURL.includes('@') && parsedHost.search === '' && parsedHost.hash === '' && (parsedHost.pathname === '' || parsedHost.pathname === '/');
  if (!validWebSocketURL || (!openai && !/^wss:\/\/[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.(?:cn-beijing|ap-southeast-1)\.maas\.aliyuncs\.com$/i.test(baseURL))) {
    throw new Error('channels.realtime.invalidHost');
  }
  const handshakeTimeoutMs = timeout.trim() === '' ? undefined : Number(timeout);
  if (handshakeTimeoutMs !== undefined && (!Number.isInteger(handshakeTimeoutMs) || handshakeTimeoutMs < 1000 || handshakeTimeoutMs > 60000)) {
    throw new Error('channels.endpoints.realtime.invalidTimeout');
  }
  const eventLimit = clientEventsPerSecond == null || clientEventsPerSecond.trim() === '' ? null : Number(clientEventsPerSecond);
  if (eventLimit !== null && (!Number.isInteger(eventLimit) || eventLimit < 256 || eventLimit > 4096)) {
    throw new Error('channels.endpoints.realtime.invalidClientEventsPerSecond');
  }
  return { enabled: true, protocol: openai ? 'openai/realtime-ws' : 'qwen/realtime-ws', baseURL, path: openai ? '/v1/realtime' : '/api-ws/v1/realtime', ...(handshakeTimeoutMs !== undefined ? { handshakeTimeoutMs } : {}), ...(clientEventsPerSecond !== undefined ? { clientEventsPerSecond: eventLimit } : {}) };
}
