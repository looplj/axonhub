import { apiRequest } from '@/lib/api-client'

export interface DeviceFlowStartResult {
  device_code: string
  user_code: string
  verification_uri: string
  expires_in: number
  interval: number
}

export interface DeviceFlowPollResult {
  access_token?: string
  error?: string
}

export interface DeviceFlowPollInput {
  device_code: string
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
