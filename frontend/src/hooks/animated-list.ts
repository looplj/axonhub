export const MAX_SEQUENTIAL_ANIMATED_ITEMS = 5;
export const MAX_SEQUENTIAL_ANIMATION_DURATION_MS = 1500;

export function getMaxSequentialAnimatedItems(animationIntervalMs: number): number {
  if (!Number.isFinite(animationIntervalMs) || animationIntervalMs <= 0) {
    return 0;
  }

  return Math.min(MAX_SEQUENTIAL_ANIMATED_ITEMS, Math.floor(MAX_SEQUENTIAL_ANIMATION_DURATION_MS / animationIntervalMs));
}

export interface AnimatedListUpdatePlan {
  shouldReplace: boolean;
  queuedIds: string[];
}

export function planAnimatedListUpdate(
  previousIds: string[],
  incomingIds: string[],
  queuedIds: string[],
  pageSize: number = incomingIds.length,
  maxSequentialAnimatedItems: number = MAX_SEQUENTIAL_ANIMATED_ITEMS
): AnimatedListUpdatePlan {
  const incomingIdSet = new Set(incomingIds);
  const nextQueue = queuedIds.filter((id, index) => incomingIdSet.has(id) && queuedIds.indexOf(id) === index);
  const snapshotsMatch = previousIds.length === incomingIds.length && previousIds.every((id, index) => id === incomingIds[index]);

  if (snapshotsMatch) {
    const shouldReplace = nextQueue.length > maxSequentialAnimatedItems || (nextQueue.length > 0 && incomingIds.length !== pageSize);
    return {
      shouldReplace,
      queuedIds: shouldReplace ? [] : nextQueue,
    };
  }

  if (previousIds.length === 0 || incomingIds.length === 0 || previousIds.length !== incomingIds.length) {
    return { shouldReplace: true, queuedIds: [] };
  }

  const previousIdSet = new Set(previousIds);
  const overlapIndex = incomingIds.findIndex((id) => previousIdSet.has(id));

  // Only a small prefix inserted ahead of the previous snapshot is a live update.
  // Pagination, deletion, reordering, and unrelated result sets replace the snapshot.
  if (overlapIndex <= 0 || incomingIds.length !== pageSize) {
    return { shouldReplace: true, queuedIds: [] };
  }

  const retainedIds = incomingIds.slice(overlapIndex);
  const retainsPreviousPrefix = retainedIds.every((id, index) => id === previousIds[index]);
  if (!retainsPreviousPrefix) {
    return { shouldReplace: true, queuedIds: [] };
  }

  const queuedIdSet = new Set(nextQueue);
  const newIdsOldestFirst = incomingIds.slice(0, overlapIndex).reverse();
  for (const id of newIdsOldestFirst) {
    if (!queuedIdSet.has(id)) {
      queuedIdSet.add(id);
      nextQueue.push(id);
    }
  }

  if (nextQueue.length > maxSequentialAnimatedItems) {
    return { shouldReplace: true, queuedIds: [] };
  }

  return { shouldReplace: false, queuedIds: nextQueue };
}
