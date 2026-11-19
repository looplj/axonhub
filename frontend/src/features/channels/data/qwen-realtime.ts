// 职责: 校验并生成百炼（Bailian）Qwen Realtime 的 WebSocket 端点地址。
// 关联文件:
//   frontend/src/features/channels/data/schema.ts
//   frontend/src/features/channels/data/protocol-options.ts
//   frontend/src/features/channels/components/channels-endpoints-dialog.tsx
//   frontend/src/locales/en/channels.json
//   frontend/src/locales/zh-CN/channels.json
// 注意事项:
//   Workspace 会被拼进 DNS 主机名，因此只允许小写字母、数字和内部连字符。
//   保存的 Base URL 只有 wss://{workspace}.{region}.maas.aliyuncs.com，路径固定。
import type { ChannelEndpoint, ChannelType } from './schema';

export const QWEN_REALTIME_API_FORMAT = 'qwen/realtime-ws';
export const QWEN_REALTIME_PATH = '/api-ws/v1/realtime';
export const QWEN_REALTIME_REGIONS = ['cn-beijing', 'ap-southeast-1'] as const;

export type QwenRealtimeRegion = (typeof QWEN_REALTIME_REGIONS)[number];

const QWEN_REALTIME_CHANNEL_TYPE: ChannelType = 'bailian';

// A DNS label: 1-63 characters, lowercase letters and digits, hyphens only between.
const WORKSPACE_PATTERN = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
const BASE_URL_PATTERN = /^wss:\/\/([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)\.(cn-beijing|ap-southeast-1)\.maas\.aliyuncs\.com\/?$/i;

export function isValidQwenRealtimeRegion(region: string | null | undefined): region is QwenRealtimeRegion {
  return typeof region === 'string' && (QWEN_REALTIME_REGIONS as readonly string[]).includes(region);
}

export function isValidQwenRealtimeWorkspace(workspace: string | null | undefined): boolean {
  return typeof workspace === 'string' && WORKSPACE_PATTERN.test(workspace.trim().toLowerCase());
}

/** Builds the only upstream host shape Bailian accepts for a workspace. */
export function buildQwenRealtimeBaseURL(workspace: string, region: string): string {
  const normalizedWorkspace = workspace.trim().toLowerCase();
  if (!isValidQwenRealtimeWorkspace(normalizedWorkspace)) {
    throw new Error('Invalid Qwen Realtime workspace');
  }
  if (!isValidQwenRealtimeRegion(region)) {
    throw new Error('Unsupported Qwen Realtime region');
  }
  return `wss://${normalizedWorkspace}.${region}.maas.aliyuncs.com`;
}

/** Reads region and workspace back from a generated host for the edit echo. */
export function parseQwenRealtimeBaseURL(baseURL: string | null | undefined): { workspace: string; region: QwenRealtimeRegion } | null {
  if (typeof baseURL !== 'string') return null;

  const match = BASE_URL_PATTERN.exec(baseURL.trim());
  if (!match) return null;

  const workspace = match[1].toLowerCase();
  const region = match[2].toLowerCase();
  if (!isValidQwenRealtimeWorkspace(workspace) || !isValidQwenRealtimeRegion(region)) {
    return null;
  }
  return { workspace, region };
}

export function isQwenRealtimeEndpointAllowed(channelType: ChannelType | null | undefined): boolean {
  return channelType === QWEN_REALTIME_CHANNEL_TYPE;
}

export function buildQwenRealtimeEndpoint(workspace: string, region: string): ChannelEndpoint {
  return {
    apiFormat: QWEN_REALTIME_API_FORMAT,
    baseURL: buildQwenRealtimeBaseURL(workspace, region),
    path: QWEN_REALTIME_PATH,
    transport: 'websocket',
  };
}
