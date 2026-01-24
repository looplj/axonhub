import { useQueryClient } from '@tanstack/react-query'
import { useSSE } from '@/hooks/useSSE'

interface RequestEventPayload {
  request_id: number
  project_id: number
  status: string
  model_id?: string
  source?: string
  stream: boolean
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

  const handleMessage = (event: MessageEvent) => {
    try {
      const payload: RequestEventPayload = JSON.parse(event.data)

      // Route to appropriate handler based on event type
      if (event.type === 'request.created') {
        onRequestCreated?.(payload)

        // Invalidate queries to refresh the list
        queryClient.invalidateQueries({ queryKey: ['requests'] })
      } else if (event.type === 'request.updated') {
        onRequestUpdated?.(payload)

        // Update specific request in cache if we have it
        // Otherwise invalidate to refresh
      queryClient.invalidateQueries({ queryKey: ['requests'] })
      } else if (event.type === 'request.completed') {
        onRequestCompleted?.(payload)

        // Update the request in cache
        queryClient.invalidateQueries({ queryKey: ['requests'] })
      }
    } catch (_error) {
      // Silently ignore parse errors
    }
  }

  const sseState = useSSE({
    url: `/admin/events/requests?project_id=${projectId}`,
    enabled,
    onMessage: handleMessage,
  })

  return sseState
}
