import { useEffect, useRef, useState } from 'react'
import { fetchEventSource, EventSourceMessage } from '@microsoft/fetch-event-source'

export interface UseSSEOptions {
  url: string
  enabled: boolean
  headers?: Record<string, string>
  onMessage?: (event: MessageEvent | EventSourceMessage) => void
  onOpen?: () => void
  onError?: (error: Error) => void
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
    headers = {},
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

  const abortControllerRef = useRef<AbortController | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const isMountedRef = useRef(true)

  const optionsRef = useRef(options)
  optionsRef.current = options

  const stateRef = useRef(state)
  stateRef.current = state

  useEffect(() => {
    isMountedRef.current = true
    return () => {
      isMountedRef.current = false
    }
  }, [])

  const cleanup = () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      abortControllerRef.current = null
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

    const abortController = new AbortController()
    abortControllerRef.current = abortController

    fetchEventSource(url, {
      headers,
      signal: abortController.signal,
      openWhenHidden: true,

      onopen: async (response) => {
        if (!isMountedRef.current) {
          abortController.abort()
          return
        }

        if (response.ok) {
          setState({
            isConnected: true,
            isConnecting: false,
            error: null,
            reconnectAttempt: 0,
          })
          onOpen?.()
        } else {
          const error = new Error(`SSE connection failed: ${response.status} ${response.statusText}`)
          setState((prev) => ({
            ...prev,
            isConnected: false,
            isConnecting: false,
            error,
          }))
          onError?.(error)
          throw error
        }
      },

      onmessage: (event: EventSourceMessage) => {
        if (!isMountedRef.current) return

        if (event.event === 'connected') {
          return
        }

        if (event.event === 'heartbeat') {
          return
        }

        const messageEvent = event as unknown as MessageEvent
        onMessage?.(messageEvent)
      },

      onerror: (err) => {
        if (!isMountedRef.current) {
          abortController.abort()
          return
        }

        const currentState = stateRef.current
        const error = err instanceof Error ? err : new Error(String(err))
        setState((prev) => ({
          ...prev,
          isConnected: false,
          isConnecting: false,
          error,
        }))

        onError?.(error)

        if (currentState.reconnectAttempt >= maxReconnectAttempts) {
          abortController.abort()
          return
        }

        const backoff = Math.min(
          reconnectInterval * Math.pow(2, currentState.reconnectAttempt),
          30000
        )

        reconnectTimeoutRef.current = setTimeout(() => {
          if (!isMountedRef.current) return
          setState((prev) => ({
            ...prev,
            reconnectAttempt: prev.reconnectAttempt + 1,
          }))
          connect()
        }, backoff)
      },

      onclose: () => {
        if (!isMountedRef.current) return

        if (!optionsRef.current.enabled) {
          return
        }

        setState((prev) => ({
          ...prev,
          isConnected: false,
          isConnecting: false,
        }))

        const currentState = stateRef.current

        if (currentState.reconnectAttempt < maxReconnectAttempts) {
          const backoff = Math.min(
            reconnectInterval * Math.pow(2, currentState.reconnectAttempt),
            30000
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
      },
    })
  }

  const headersString = JSON.stringify(headers)

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
    // connect is intentionally excluded - it's stable within this hook's lifecycle
    // and uses refs to avoid stale closures
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url, enabled, headersString])

  return state
}
