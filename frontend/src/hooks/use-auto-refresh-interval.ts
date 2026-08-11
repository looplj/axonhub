import { useCallback, useState } from 'react';

export const AUTO_REFRESH_INTERVALS = [5000, 10000, 30000, 60000] as const;

export type EnabledAutoRefreshInterval = (typeof AUTO_REFRESH_INTERVALS)[number];
export type AutoRefreshInterval = EnabledAutoRefreshInterval | null;

type AutoRefreshStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

export function parseAutoRefreshInterval(value: string | null): AutoRefreshInterval {
  return AUTO_REFRESH_INTERVALS.find((interval) => interval.toString() === value) ?? null;
}

export function readAutoRefreshInterval(storageKey: string, storage: AutoRefreshStorage): AutoRefreshInterval {
  try {
    const storedValue = storage.getItem(storageKey);
    const interval = parseAutoRefreshInterval(storedValue);

    if (storedValue !== null && interval === null) {
      storage.removeItem(storageKey);
    }

    return interval;
  } catch {
    return null;
  }
}

export function writeAutoRefreshInterval(storageKey: string, interval: AutoRefreshInterval, storage: AutoRefreshStorage) {
  if (interval === null) {
    storage.removeItem(storageKey);
  } else {
    storage.setItem(storageKey, interval.toString());
  }
}

export function readBrowserAutoRefreshInterval(storageKey: string): AutoRefreshInterval {
  if (typeof window === 'undefined') return null;

  try {
    return readAutoRefreshInterval(storageKey, window.localStorage);
  } catch {
    return null;
  }
}

export function useAutoRefreshInterval(storageKey: string) {
  const [interval, setIntervalState] = useState<AutoRefreshInterval>(() => readBrowserAutoRefreshInterval(storageKey));

  const setInterval = useCallback(
    (nextInterval: AutoRefreshInterval) => {
      setIntervalState(nextInterval);

      try {
        writeAutoRefreshInterval(storageKey, nextInterval, window.localStorage);
      } catch {
        // Keep the in-memory setting when localStorage is unavailable.
      }
    },
    [storageKey]
  );

  return [interval, setInterval] as const;
}
