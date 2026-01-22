import { useEffect, type RefObject } from 'react'

export function useClickOutside(
  ref: RefObject<HTMLElement | null>,
  onOutside: () => void,
  enabled = true
) {
  useEffect(() => {
    if (!enabled) return

    function handler(event: MouseEvent) {
      const target = event.target as Node
      if (!ref.current) return
      if (!ref.current.contains(target)) onOutside()
    }

    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [ref, onOutside, enabled])
}
