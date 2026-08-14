import { apiRequest } from '@/lib/api-client';
import type { ProxyConfig } from '../hooks/use-oauth-flow';

export async function xaiOAuthStart(headers?: Record<string, string>): Promise<{ session_id: string; auth_url: string }> {
  return apiRequest('/admin/xai/oauth/start', {
    method: 'POST',
    body: {},
    headers,
    requireAuth: true,
  });
}

export async function xaiOAuthExchange(
  input: {
    readonly session_id: string;
    readonly callback_url: string;
    readonly proxy?: ProxyConfig;
  },
  headers?: Record<string, string>
): Promise<{ credentials: string }> {
  return apiRequest('/admin/xai/oauth/exchange', {
    method: 'POST',
    body: input,
    headers,
    requireAuth: true,
  });
}

export async function xaiDecodeSSO(
  input: {
    readonly sso_token: string;
    readonly proxy?: ProxyConfig;
  },
  headers?: Record<string, string>
): Promise<{ credentials: string }> {
  return apiRequest('/admin/xai/oauth/sso', {
    method: 'POST',
    body: input,
    headers,
    requireAuth: true,
  });
}
