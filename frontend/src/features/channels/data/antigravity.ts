import { apiRequest } from '@/lib/api-client'
import { ProxyConfig } from '../hooks/use-oauth-flow'

export async function antigravityOAuthStart(headers?: Record<string, string>, projectId?: string): Promise<{ session_id: string; auth_url: string }> {
  return apiRequest('/admin/antigravity/oauth/start', {
    method: 'POST',
    body: {
        project_id: projectId
    },
    headers,
    requireAuth: true,
  })
}

export async function antigravityOAuthExchange(
  input: {
    session_id: string
    callback_url: string
    proxy?: ProxyConfig
  },
  headers?: Record<string, string>
): Promise<{ credentials: string }> {
  return apiRequest('/admin/antigravity/oauth/exchange', {
    method: 'POST',
    body: input,
    headers,
    requireAuth: true,
  })
}

export const ANTIGRAVITY_BASE_URL_DAILY_SANDBOX = 'https://daily-cloudcode-pa.sandbox.googleapis.com';
export const ANTIGRAVITY_BASE_URL_DAILY = 'https://daily-cloudcode-pa.googleapis.com';
export const ANTIGRAVITY_BASE_URL_PROD = 'https://cloudcode-pa.googleapis.com';
export const ANTIGRAVITY_BASE_URL_AUTOPUSH = 'https://autopush-cloudcode-pa.sandbox.googleapis.com';

export const ANTIGRAVITY_BASE_URLS = [
  {
    label: 'Daily Sandbox (https://daily-cloudcode-pa.sandbox.googleapis.com)',
    value: ANTIGRAVITY_BASE_URL_DAILY_SANDBOX,
  },
  {
    label: 'Daily (https://daily-cloudcode-pa.googleapis.com)',
    value: ANTIGRAVITY_BASE_URL_DAILY,
  },
  {
    label: 'Production (https://cloudcode-pa.googleapis.com)',
    value: ANTIGRAVITY_BASE_URL_PROD,
  },
  {
    label: 'Autopush Sandbox (https://autopush-cloudcode-pa.sandbox.googleapis.com)',
    value: ANTIGRAVITY_BASE_URL_AUTOPUSH,
  },
];
