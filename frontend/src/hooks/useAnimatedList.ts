import { useState, useEffect, useRef } from 'react';
import useInterval from './useInterval';

const MAX_ITEMS = 50;
const parsedInterval = parseInt(import.meta.env.VITE_REQUESTS_ANIMATION_INTERVAL, 10);
const ANIMATION_INTERVAL = !isNaN(parsedInterval) && parsedInterval > 0 ? parsedInterval : 500;

export function useAnimatedList<T extends { id: string; createdAt: Date | string }>(data: T[], autoRefresh: boolean) {
  const [displayedData, setDisplayedData] = useState<T[]>(data);
  const queueRef = useRef<T[]>([]);
  const prevDataLengthRef = useRef<number>(data.length);

  const getTimestamp = (date: Date | string): number => {
    return date instanceof Date ? date.getTime() : new Date(date).getTime();
  };

  useEffect(() => {
    if (!autoRefresh) {
      setDisplayedData(data);
      queueRef.current = [];
      prevDataLengthRef.current = data.length;
      return;
    }

    setDisplayedData((currentDisplayed) => {
      const currentIds = new Set(currentDisplayed.map((r) => r.id));
      const newDataMap = new Map(data.map((r) => [r.id, r]));

      // Compute the minimum timestamp from incoming data to establish the time window
      const minTimestampOfNewData =
        data.length > 0 ? Math.min(...data.map((item) => getTimestamp(item.createdAt))) : 0;

      // Only flag removals when the removed item is still within the new data's time window
      // Items pushed off by pagination (older than minTimestampOfNewData) should not trigger a reset
      const hasRemovedItems = currentDisplayed.some((item) => {
        const isMissingFromNewData = !newDataMap.has(item.id);
        const itemTimestamp = getTimestamp(item.createdAt);
        return isMissingFromNewData && itemTimestamp >= minTimestampOfNewData;
      });

      const shouldResetToNewData = hasRemovedItems || data.length !== prevDataLengthRef.current;

      if (shouldResetToNewData) {
        prevDataLengthRef.current = data.length;
        queueRef.current = [];
        return data;
      }

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

      prevDataLengthRef.current = data.length;
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
