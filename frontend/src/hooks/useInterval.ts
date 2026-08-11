import { useEffect, useRef, useState } from 'react';

const RESUME_DELAY_TOLERANCE_MS = 1000;

interface UseIntervalOptions {
  refreshOnResume?: boolean;
}

const useInterval = (callback: () => void, delay?: number | null, options?: UseIntervalOptions) => {
  const savedCallback = useRef(callback);
  const [resumeKey, setResumeKey] = useState(0);
  const refreshOnResume = options?.refreshOnResume ?? false;

  useEffect(() => {
    savedCallback.current = callback;
  });

  useEffect(() => {
    if (delay == null) {
      return undefined;
    }

    if (!refreshOnResume) {
      const interval = setInterval(() => savedCallback.current(), delay);
      return () => clearInterval(interval);
    }

    let interval: ReturnType<typeof setInterval> | undefined;
    let lastTickAt = Date.now();
    let pausedForVisibility = document.visibilityState === 'hidden';

    const stopInterval = () => {
      if (interval !== undefined) {
        clearInterval(interval);
        interval = undefined;
      }
    };

    const runCallback = (detectDelayedTick = false) => {
      const now = Date.now();
      if (detectDelayedTick && now - lastTickAt > delay + RESUME_DELAY_TOLERANCE_MS) {
        setResumeKey((current) => current + 1);
      }
      lastTickAt = now;
      savedCallback.current();
    };

    const startInterval = () => {
      stopInterval();
      interval = setInterval(() => runCallback(true), delay);
    };

    const resume = () => {
      if (document.visibilityState === 'hidden') return;

      const resumedAfterDelay = Date.now() - lastTickAt > delay + RESUME_DELAY_TOLERANCE_MS;
      if (!pausedForVisibility && !resumedAfterDelay) return;

      pausedForVisibility = false;
      setResumeKey((current) => current + 1);
      runCallback();
      startInterval();
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        pausedForVisibility = true;
        stopInterval();
        return;
      }

      resume();
    };

    if (!pausedForVisibility) {
      startInterval();
    }

    document.addEventListener('visibilitychange', handleVisibilityChange);
    window.addEventListener('focus', resume);

    return () => {
      stopInterval();
      document.removeEventListener('visibilitychange', handleVisibilityChange);
      window.removeEventListener('focus', resume);
    };
  }, [delay, refreshOnResume]);

  return resumeKey;
};

export default useInterval;
