import { apiRequest } from '@/lib/api-client'

export interface DeviceFlowStartResult {
  session_id: string
  user_code: string
  verification_uri: string
  expires_in: number
  interval: number
}

export interface DeviceFlowPollResult {
  access_token?: string
  token_type?: string
  scope?: string
  status?: string
  message?: string
}

export interface DeviceFlowPollInput {
  session_id: string
}

export async function copilotOAuthStart(headers?: Record<string, string>): Promise<DeviceFlowStartResult> {
  return apiRequest('/admin/copilot/oauth/start', {
    method: 'POST',
    body: {},
    headers,
    requireAuth: true,
  })
}

export async function copilotOAuthPoll(
  input: DeviceFlowPollInput,
  headers?: Record<string, string>
): Promise<DeviceFlowPollResult> {
  return apiRequest('/admin/copilot/oauth/poll', {
    method: 'POST',
    body: input,
    headers,
    requireAuth: true,
  })
}
