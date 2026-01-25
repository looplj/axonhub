import { useCallback, useQueryClient } from '@tanstack/react-query'
import { useSSE } from '@/hooks/useSSE'
import { useAuthStore, getTokenFromStorage } from '@/stores/authStore'
import { EventSourceMessage } from '@microsoft/fetch-event-source'

interface RequestEventPayload {
  request_id: number
  project_id: number
  status: string
  model_id?: string
  source?: string
  stream: boolean
  prompt_tokens?: number
  completion_tokens?: number
  total_tokens?: number
  prompt_cached_tokens?: number
  prompt_write_cached_tokens?: number
  prompt_write_cached_tokens_5m?: number
  prompt_write_cached_tokens_1h?: number
  api_key_id?: number
  channel_id?: number
  created_at: string
}

interface UseRequestsSSEOptions {
  enabled: boolean
  projectId: number
  onRequestCreated?: (payload: RequestEventPayload) => void
  onRequestUpdated?: (payload: RequestEventPayload) => void
  onRequestCompleted?: (payload: RequestEventPayload) => void
}

export function useRequestsSSE(options: UseRequestsSSEOptions) {
  const { enabled, projectId, onRequestCreated, onRequestUpdated, onRequestCompleted } = options
  const queryClient = useQueryClient()
  const { accessToken } = useAuthStore((state) => state.auth)

  // Always prefer localStorage token to avoid race conditions with zustand hydration
  // zustand store might not be hydrated yet on initial render
  const token = accessToken || getTokenFromStorage()

  const handleMessage = useCallback((event: MessageEvent | EventSourceMessage) => {
    const isSSE = 'event' in event && 'data' in event
    const eventType = isSSE ? (event as EventSourceMessage).event : (event as MessageEvent).type
    const eventData = isSSE ? (event as EventSourceMessage).data : (event as MessageEvent).data

    try {
      const payload: RequestEventPayload = JSON.parse(eventData)

      // Route to appropriate handler based on event type
      if (eventType === 'request.created') {
        onRequestCreated?.(payload)
        queryClient.invalidateQueries({ queryKey: ['requests'] })
      } else if (eventType === 'request.updated') {
        onRequestUpdated?.(payload)
        queryClient.invalidateQueries({ queryKey: ['requests'] })
      } else if (eventType === 'request.completed') {
        onRequestCompleted?.(payload)
        queryClient.invalidateQueries({ queryKey: ['requests'] })
      }
    } catch (error) {
      console.warn('[useRequestsSSE] Failed to parse event payload:', error, { eventData, eventType })
    }
  }, [onRequestCreated, onRequestUpdated, onRequestCompleted, queryClient])

  const sseState = useSSE({
    url: `/admin/events/requests?project_id=${projectId}`,
    enabled: enabled && !!token,
    headers: {
      Authorization: `Bearer ${token}`,
    },
    onMessage: handleMessage,
  })

  return sseState
}
