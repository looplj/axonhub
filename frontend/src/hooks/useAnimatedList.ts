import { useState, useEffect, useRef } from 'react';
import useInterval from './useInterval';

const MAX_ITEMS = 50;
const ANIMATION_INTERVAL = 500;

export function useAnimatedList<T extends { id: string; createdAt: Date | string }>(data: T[], autoRefresh: boolean) {
  const [displayedData, setDisplayedData] = useState<T[]>(data);
  const queueRef = useRef<T[]>([]);

  const getTimestamp = (date: Date | string): number => {
    return date instanceof Date ? date.getTime() : new Date(date).getTime();
  };

  useEffect(() => {
    if (!autoRefresh) {
      setDisplayedData(data);
      queueRef.current = [];
      return;
    }

    setDisplayedData((currentDisplayed) => {
      const currentIds = new Set(currentDisplayed.map((r) => r.id));
      const newDataMap = new Map(data.map((r) => [r.id, r]));

      const updatedDisplayed = currentDisplayed.map((item) => {
        const newItem = newDataMap.get(item.id);
        return newItem ? newItem : item;
      });

      const newestCurrentTime = currentDisplayed.length > 0 ? getTimestamp(currentDisplayed[0].createdAt) : 0;

      const newItems = data.filter((item) => {
        const isNew = !currentIds.has(item.id);
        const isNewer = getTimestamp(item.createdAt) > newestCurrentTime;
        return isNew && isNewer;
      });

      const sortedNewItems = newItems.sort((a, b) => getTimestamp(a.createdAt) - getTimestamp(b.createdAt));

      sortedNewItems.forEach((item) => {
        if (!queueRef.current.some((q) => q.id === item.id)) {
          queueRef.current.push(item);
        }
      });

      return updatedDisplayed;
    });
  }, [data, autoRefresh]);

  useInterval(
    () => {
      if (queueRef.current.length > 0) {
        const nextItem = queueRef.current.shift();
        if (nextItem) {
          setDisplayedData((prev) => {
            const newData = [nextItem, ...prev];
            if (newData.length > MAX_ITEMS) {
              return newData.slice(0, MAX_ITEMS);
            }
            return newData;
          });
        }
      }
    },
    autoRefresh ? ANIMATION_INTERVAL : null
  );

  return displayedData;
}
