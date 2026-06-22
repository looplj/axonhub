import { useEffect, useRef, type RefObject } from 'react';

export function useHorizontalScroll<T extends HTMLElement>(): RefObject<T | null> {
  const ref = useRef<T>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const handler = (e: WheelEvent) => {
      // Ignore pinch-zoom / browser zoom gestures
      if (e.ctrlKey || e.metaKey) return;

      // Let the browser handle Shift+wheel (native horizontal scroll)
      if (e.shiftKey) return;

      // Don't interfere with native horizontal scroll (trackpad swipe)
      if (Math.abs(e.deltaX) > Math.abs(e.deltaY)) return;

      // Only intercept when the container actually scrolls horizontally
      const style = window.getComputedStyle(el);
      const overflowX = style.overflowX;
      if (overflowX !== 'auto' && overflowX !== 'scroll') return;

      if (el.scrollWidth <= el.clientWidth) return;

      // Only preventDefault if the container can scroll further in the wheel direction
      const atStart = el.scrollLeft <= 0;
      const atEnd = el.scrollLeft >= el.scrollWidth - el.clientWidth;

      if (e.deltaY < 0 && atStart) return;
      if (e.deltaY > 0 && atEnd) return;

      e.preventDefault();

      // Normalize delta to pixels (handle LINE and PAGE delta modes)
      let delta = e.deltaY;
      if (e.deltaMode === WheelEvent.DOM_DELTA_LINE) {
        delta *= 16; // approximate line height
      } else if (e.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
        delta *= el.clientWidth;
      }

      el.scrollLeft += delta;
    };

    el.addEventListener('wheel', handler, { passive: false });
    return () => el.removeEventListener('wheel', handler);
  }, []);

  return ref;
}