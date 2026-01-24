import { useEffect, useRef, useState } from 'react'

export interface UseSSEOptions {
  url: string
  enabled: boolean
  onMessage?: (event: MessageEvent) => void
  onOpen?: () => void
  onError?: (error: Event) => void
  reconnectInterval?: number
  maxReconnectAttempts?: number
}

export interface SSEState {
  isConnected: boolean
  isConnecting: boolean
  error: Error | null
  reconnectAttempt: number
}

export function useSSE(options: UseSSEOptions): SSEState {
  const {
    url,
    enabled,
    onMessage,
    onOpen,
    onError,
    reconnectInterval = 3000,
    maxReconnectAttempts = 10,
  } = options

  const [state, setState] = useState<SSEState>({
    isConnected: false,
    isConnecting: false,
    error: null,
    reconnectAttempt: 0,
  })

  const eventSourceRef = useRef<EventSource | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const isMountedRef = useRef(true)

  useEffect(() => {
    isMountedRef.current = true
    return () => {
      isMountedRef.current = false
    }
  }, [])

  const cleanup = () => {
    if (eventSourceRef.current) {
   eventSourceRef.current.close()
      eventSourceRef.current = null
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }
  }
  const connect = () => {
    if (!enabled || !isMountedRef.current) {
      return
    }

    cleanup()

    setState((prev) => ({ ...prev, isConnecting: true, error: null }))

    const eventSource = new EventSource(url)
    eventSourceRef.current = eventSource

    eventSource.addEventListener('open', () => {
      if (!isMountedRef.current) return
      setState({
        isConnected: true,
     isConnecting: false,
        error: null,
        reconnectAttempt: 0,
      })
      onOpen?.()
    })

    eventSource.addEventListener('message', (event) => {
      if (!isMountedRef.current) return
      onMessage?.(event)
    })

    eventSource.addEventListener('error', (event) => {
      if (!isMountedRef.current) return

      const error = new Error('SSE connection error')
      setState((prev) => ({
        ...prev,
        isConnected: false,
        isConnecting: false,
        error,
      }))

      onError?.(event)
      eventSource.close()

      // Attempt reconnection with exponential backoff
      if (state.reconnectAttempt < maxReconnectAttempts) {
        const backoff = Math.min(
       reconnectInterval * Math.pow(2, state.reconnectAttempt),
          300 // max 30 seconds
        )

        reconnectTimeoutRef.current = setTimeout(() => {
          if (!isMountedRef.current) return
       setState((prev) => ({
            ...prev,
         reconnectAttempt: prev.reconnectAttempt + 1,
        }))
          connect()
        }, backoff)
      }
    })

    // Listen for custom event types
    eventSource.addEventListener('request.created', (event) => {
      if (!isMountedRef.current) return
      onMessage?.(event as MessageEvent)
    })

    eventSource.addEventListener('request.updated', (event) => {
      if (!isMountedRef.current) return
      onMessage?.(event as MessageEvent)
    })

    eventSource.addEventListener('request.completed', (event) => {
      if (!isMountedRef.current) return
      onMessage?.(event as MessageEvent)
    })
  }

  useEffect(() => {
    if (enabled) {
    connect()
    } else {
      cleanup()
      setState({
        isConnected: false,
        isConnecting: false,
        error: null,
      reconnectAttempt: 0,
      })
    }

    return cleanup
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url, enabled])

  return state
}
