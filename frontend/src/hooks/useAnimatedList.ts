import { useState, useEffect, useRef } from 'react';
import { getMaxSequentialAnimatedItems, planAnimatedListUpdate } from './animated-list';
import useInterval from './useInterval';

const MAX_ITEMS = 50;
const parsedInterval = parseInt(import.meta.env.VITE_REQUESTS_ANIMATION_INTERVAL, 10);
const ANIMATION_INTERVAL = !isNaN(parsedInterval) && parsedInterval > 0 ? parsedInterval : 500;
const MAX_ANIMATED_ITEMS = getMaxSequentialAnimatedItems(ANIMATION_INTERVAL);

export function useAnimatedList<T extends { id: string; createdAt: Date | string }>(
  data: T[],
  animateUpdates: boolean,
  pageSize: number = MAX_ITEMS,
  resetKey?: string
) {
  const getTimestamp = (date: Date | string): number => {
    return date instanceof Date ? date.getTime() : new Date(date).getTime();
  };
  const dataSignature = data.map((item) => `${item.id}:${getTimestamp(item.createdAt)}`).join('|');

  const [displayedData, setDisplayedData] = useState<T[]>(data);
  const queueRef = useRef<T[]>([]);
  const prevResetKeyRef = useRef(resetKey);
  const prevDataSignatureRef = useRef(dataSignature);
  const prevDataIdsRef = useRef(data.map((item) => item.id));
  const pendingResetDataSignatureRef = useRef<string | null>(null);

  useEffect(() => {
    const resetKeyChanged = resetKey !== prevResetKeyRef.current;
    if (resetKeyChanged) {
      prevResetKeyRef.current = resetKey;
      pendingResetDataSignatureRef.current = prevDataSignatureRef.current;
    }

    if (pendingResetDataSignatureRef.current !== null) {
      // The first result after a query change is a new result set, not a poll update.
      if (dataSignature !== pendingResetDataSignatureRef.current) {
        pendingResetDataSignatureRef.current = null;
      }
      setDisplayedData(data);
      queueRef.current = [];
      prevDataSignatureRef.current = dataSignature;
      prevDataIdsRef.current = data.map((item) => item.id);
      return;
    }

    if (!animateUpdates) {
      setDisplayedData(data);
      queueRef.current = [];
      prevDataSignatureRef.current = dataSignature;
      prevDataIdsRef.current = data.map((item) => item.id);
      return;
    }

    const incomingIds = data.map((item) => item.id);
    const newDataMap = new Map(data.map((item) => [item.id, item]));
    const updatePlan = planAnimatedListUpdate(
      prevDataIdsRef.current,
      incomingIds,
      queueRef.current.map((item) => item.id),
      pageSize,
      MAX_ANIMATED_ITEMS
    );

    prevDataIdsRef.current = incomingIds;
    if (updatePlan.shouldReplace) {
      queueRef.current = [];
      setDisplayedData(data);
    } else {
      queueRef.current = updatePlan.queuedIds.flatMap((id) => {
        const item = newDataMap.get(id);
        return item ? [item] : [];
      });
      setDisplayedData((currentDisplayed) => {
        const updatedDisplayed = currentDisplayed.map((item) => {
          const newItem = newDataMap.get(item.id);
          return newItem ? newItem : item;
        });
        return updatedDisplayed;
      });
    }
    prevDataSignatureRef.current = dataSignature;
  }, [data, animateUpdates, pageSize, resetKey]);

  useInterval(
    () => {
      if (queueRef.current.length > 0) {
        const nextItem = queueRef.current.shift();
        if (nextItem) {
          setDisplayedData((prev) => {
            const newData = [nextItem, ...prev];
            return newData.slice(0, pageSize);
          });
        }
      }
    },
    animateUpdates ? ANIMATION_INTERVAL : null
  );

  return displayedData;
}
