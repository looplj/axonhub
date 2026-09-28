export type CursorDirection = 'after' | 'before';

export interface PaginationVariables {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
}

/**
 * The cursor history stores one `after` boundary per page visited before the
 * current one: the `endCursor` of page N fetches page N + 1 when used as `after`.
 *
 * - Moving forward keeps the end cursor of the page being left, which is the
 *   only boundary that can re-enter it later.
 * - Moving backward pops that boundary, so the new top of the history is the
 *   boundary that fetches the previous page.
 */
export function updateCursorHistory(
  cursorHistory: string[],
  direction: CursorDirection,
  nextEndCursor: string | undefined
): string[] {
  const newHistory = [...cursorHistory];

  if (direction === 'after' && nextEndCursor && !newHistory.includes(nextEndCursor)) {
    newHistory.push(nextEndCursor);
  } else if (direction === 'before' && newHistory.length > 0) {
    newHistory.pop();
  }

  return newHistory;
}

/**
 * Build the connection arguments for the next fetch. Both directions scroll
 * forward with `first`/`after`: forward continues past the current page, while
 * backward re-enters the previous page from its front boundary in the history.
 */
export function buildPaginationArgs(
  cursorDirection: CursorDirection,
  cursorHistory: string[],
  endCursor: string | undefined,
  pageSize: number
): PaginationVariables {
  if (cursorDirection === 'before') {
    return {
      first: pageSize,
      after: cursorHistory[cursorHistory.length - 1],
    };
  }

  return {
    first: pageSize,
    after: endCursor,
  };
}
