import { useState, useEffect, useRef } from 'react';

/**
 * Hook that ensures a loading state stays true for a minimum duration.
 * This provides better UX by ensuring users see loading feedback even for fast operations.
 * 
 * @param isLoading - The actual loading state
 * @param minimumMs - Minimum duration in milliseconds (default: 500)
 * @returns The delayed loading state
 */
export function useMinimumLoadingTime(isLoading: boolean, minimumMs: number = 500): boolean {
  const [delayedLoading, setDelayedLoading] = useState(false);
  const loadingStartTimeRef = useRef<number | null>(null);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    // When loading starts
    if (isLoading && !delayedLoading) {
      loadingStartTimeRef.current = Date.now();
      setDelayedLoading(true);
    }

    // When loading ends
    if (!isLoading && delayedLoading) {
      const elapsed = Date.now() - (loadingStartTimeRef.current || 0);
      const remaining = Math.max(0, minimumMs - elapsed);

      if (remaining > 0) {
        // Keep showing loading for the remaining time
        timeoutRef.current = setTimeout(() => {
          setDelayedLoading(false);
          loadingStartTimeRef.current = null;
        }, remaining);
      } else {
        // Minimum time already elapsed, stop immediately
        setDelayedLoading(false);
        loadingStartTimeRef.current = null;
      }
    }

    // Cleanup timeout on unmount or when dependencies change
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }
    };
  }, [isLoading, delayedLoading, minimumMs]);

  return delayedLoading;
}
