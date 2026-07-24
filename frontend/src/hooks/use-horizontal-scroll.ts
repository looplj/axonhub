import { useCallback, useEffect, useRef, type RefCallback } from 'react';

function canScrollInDirection(element: Element, deltaY: number): boolean {
  const maxScrollTop = element.scrollHeight - element.clientHeight;
  if (maxScrollTop <= 1) return false;
  if (deltaY < 0) return element.scrollTop > 1;
  if (deltaY > 0) return element.scrollTop < maxScrollTop - 1;
  return false;
}

function hasScrollableVerticalAncestor(element: HTMLElement, deltaY: number): boolean {
  for (let ancestor = element.parentElement; ancestor; ancestor = ancestor.parentElement) {
    const { overflowY } = window.getComputedStyle(ancestor);
    if ((overflowY === 'auto' || overflowY === 'scroll') && canScrollInDirection(ancestor, deltaY)) {
      return true;
    }
  }

  const scrollingElement = element.ownerDocument.scrollingElement;
  return scrollingElement ? canScrollInDirection(scrollingElement, deltaY) : false;
}

export function useHorizontalScroll<T extends HTMLElement>(): RefCallback<T> {
  const cleanupRef = useRef<(() => void) | null>(null);

  const setRef = useCallback((el: T | null) => {
    cleanupRef.current?.();
    cleanupRef.current = null;
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
      if (hasScrollableVerticalAncestor(el, e.deltaY)) return;

      // Convert the wheel gesture only when no vertical ancestor can consume it.
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
    cleanupRef.current = () => el.removeEventListener('wheel', handler);
  }, []);

  useEffect(() => () => cleanupRef.current?.(), []);

  return setRef;
}
