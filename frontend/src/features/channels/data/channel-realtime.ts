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

export function getRealtimeHandshakeTimeoutInputValue(timeoutMs: number | null | undefined): string {
  return String(timeoutMs ?? DEFAULT_REALTIME_HANDSHAKE_TIMEOUT_MS);
}

export function getChannelBaseURLInputValue(format: string, channelBaseURL: string | null | undefined, qwenRealtimeHost: string): string {
  if (format === 'openai/realtime-ws') return OPENAI_REALTIME_HOST;
  if (format === 'qwen/realtime-ws') return qwenRealtimeHost;
  return channelBaseURL ?? '';
}

export function getModelFetchBaseURL(format: string, channelBaseURL: string | null | undefined, qwenRealtimeHost: string): string {
  if (format !== 'openai/realtime-ws' && format !== 'qwen/realtime-ws') return channelBaseURL ?? '';
  return getChannelBaseURLInputValue(format, channelBaseURL, qwenRealtimeHost).replace(/^wss:/i, 'https:').replace(/^ws:/i, 'http:');
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

export function realtimeForSelection(format: string, current: ChannelRealtime | null | undefined, host: string, timeout: string): ChannelRealtime | undefined {
  if (!isRealtimeProtocol(format)) return current ? { ...current, enabled: false } : undefined;
  if (format === 'openai/realtime') throw new Error('channels.realtime.invalidHost');
  let baseURL = (host.trim() || (format === 'openai/realtime-ws' ? 'wss://api.openai.com' : '')).replace(/\/$/, '');
  if (format === 'openai/realtime-ws' && baseURL === 'wss://api.openai.com/v1/realtime') baseURL = 'wss://api.openai.com';
  const openai = format === 'openai/realtime-ws';
  if (openai ? baseURL !== 'wss://api.openai.com' : !/^wss:\/\/[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.(?:cn-beijing|ap-southeast-1)\.maas\.aliyuncs\.com$/.test(baseURL)) {
    throw new Error('channels.realtime.invalidHost');
  }
  const handshakeTimeoutMs = timeout.trim() === '' ? undefined : Number(timeout);
  if (handshakeTimeoutMs !== undefined && (!Number.isInteger(handshakeTimeoutMs) || handshakeTimeoutMs < 1000 || handshakeTimeoutMs > 60000)) {
    throw new Error('channels.endpoints.realtime.invalidTimeout');
  }
  return { enabled: true, protocol: openai ? 'openai/realtime-ws' : 'qwen/realtime-ws', baseURL, path: openai ? '/v1/realtime' : '/api-ws/v1/realtime', ...(handshakeTimeoutMs !== undefined ? { handshakeTimeoutMs } : {}) };
}
