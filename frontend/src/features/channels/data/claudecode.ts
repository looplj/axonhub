import { apiRequest } from '@/lib/api-client'

export async function claudecodeOAuthStart(headers?: Record<string, string>): Promise<{ session_id: string; auth_url: string }> {
  return apiRequest('/admin/claudecode/oauth/start', {
    method: 'POST',
    body: {},
    headers,
    requireAuth: true,
  })
}

export async function claudecodeOAuthExchange(
  input: {
    session_id: string
    callback_url: string
  },
  headers?: Record<string, string>
): Promise<{ credentials: string }> {
  return apiRequest('/admin/claudecode/oauth/exchange', {
    method: 'POST',
    body: input,
    headers,
    requireAuth: true,
  })
}
