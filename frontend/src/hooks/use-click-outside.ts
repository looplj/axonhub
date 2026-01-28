import { useEffect, type RefObject } from 'react'

export function useClickOutside(
  ref: RefObject<HTMLElement | null>,
  onOutside: () => void,
  enabled = true
) {
  useEffect(() => {
    if (!enabled) return

    function handler(event: Event) {
      const target = event.target as Node
      if (!ref.current) return
      if (!ref.current.contains(target)) onOutside()
    }

    const usePointer = typeof window !== 'undefined' && 'PointerEvent' in window
    const primaryEvent = usePointer ? 'pointerdown' : 'mousedown'
    const fallbackEvent = usePointer ? null : 'touchstart'

    document.addEventListener(primaryEvent, handler)
    if (fallbackEvent) document.addEventListener(fallbackEvent, handler)
    return () => {
      document.removeEventListener(primaryEvent, handler)
      if (fallbackEvent) document.removeEventListener(fallbackEvent, handler)
    }
  }, [ref, onOutside, enabled])
}
