import { useCallback, useMemo } from 'react'
import { useNavigate, useRouterState } from '@tanstack/react-router'

interface UsePaginationSearchOptions {
  defaultPageSize?: number
  startCursorKey?: string
  endCursorKey?: string
  pageSizeKey?: string
  directionKey?: string
  cursorHistoryKey?: string
  defaultDirection?: CursorDirection
  replace?: boolean
}

interface PaginationState {
  startCursor: string | undefined
  endCursor: string | undefined
  pageSize: number
  pageSizeFromSearch: number | undefined
  cursorDirection: CursorDirection
  cursorHistory: string[]
}

interface PaginationVariables {
  first?: number
  after?: string
  last?: number
  before?: string
}

type CursorDirection = 'after' | 'before'

export interface UsePaginationSearchResult {
  startCursor: string | undefined
  endCursor: string | undefined
  pageSize: number
  pageSizeFromSearch: number | undefined
  paginationArgs: PaginationVariables
  cursorHistory: string[]
  setCursors: (startCursor: string | undefined, endCursor: string | undefined, direction?: CursorDirection) => void
  setPageSize: (pageSize: number) => void
  resetCursor: () => void
  getSearchParams: () => Record<string, unknown>
  navigateWithSearch: (options: { to: string; params?: Record<string, any> }) => void
}

const DEFAULT_PAGE_SIZE = 20
const DEFAULT_START_CURSOR_KEY = 'startCursor'
const DEFAULT_END_CURSOR_KEY = 'endCursor'
const DEFAULT_PAGE_SIZE_KEY = 'pageSize'
const DEFAULT_DIRECTION_KEY = 'cursorDirection'
const DEFAULT_CURSOR_HISTORY_KEY = 'cursorHistory'
const DEFAULT_DIRECTION: CursorDirection = 'after'

export function usePaginationSearch(
  options: UsePaginationSearchOptions = {}
): UsePaginationSearchResult {
  const {
    defaultPageSize = DEFAULT_PAGE_SIZE,
    startCursorKey = DEFAULT_START_CURSOR_KEY,
    endCursorKey = DEFAULT_END_CURSOR_KEY,
    pageSizeKey = DEFAULT_PAGE_SIZE_KEY,
    directionKey = DEFAULT_DIRECTION_KEY,
    cursorHistoryKey = DEFAULT_CURSOR_HISTORY_KEY,
    defaultDirection = DEFAULT_DIRECTION,
    replace = true,
  } = options

  const navigate = useNavigate()
  const { search } = useRouterState({
    select: (state) => ({
      search: (state.location.search ?? {}) as Record<string, unknown>,
    }),
  })

  const rawStartCursor = search[startCursorKey]
  const rawEndCursor = search[endCursorKey]
  const rawPageSize = search[pageSizeKey]
  const rawDirection = search[directionKey]
  const rawCursorHistory = search[cursorHistoryKey]

  const { startCursor, endCursor, pageSize, pageSizeFromSearch, cursorDirection, cursorHistory } = useMemo<PaginationState>(() => {
    const parsedStartCursor = typeof rawStartCursor === 'string' && rawStartCursor.length > 0 ? rawStartCursor : undefined
    const parsedEndCursor = typeof rawEndCursor === 'string' && rawEndCursor.length > 0 ? rawEndCursor : undefined

    const parsedPageSize = (() => {
      const candidate =
        typeof rawPageSize === 'number'
          ? rawPageSize
          : typeof rawPageSize === 'string'
            ? Number.parseInt(rawPageSize, 10)
            : undefined

      if (typeof candidate !== 'number' || !Number.isFinite(candidate) || candidate <= 0) {
        return undefined
      }

      return Math.floor(candidate)
    })()

    const parsedDirection: CursorDirection = rawDirection === 'before' ? 'before' : defaultDirection

    const parsedCursorHistory = (() => {
      try {
        if (typeof rawCursorHistory === 'string' && rawCursorHistory.length > 0) {
          const parsed = JSON.parse(rawCursorHistory)
          if (Array.isArray(parsed)) {
            return parsed.filter((item): item is string => typeof item === 'string' && item.length > 0)
          }
        }
      } catch {
        // Invalid JSON, return empty array
      }
      return []
    })()

    return {
      startCursor: parsedStartCursor,
      endCursor: parsedEndCursor,
      pageSize: parsedPageSize ?? defaultPageSize,
      pageSizeFromSearch: parsedPageSize,
      cursorDirection: parsedDirection,
      cursorHistory: parsedCursorHistory,
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rawStartCursor, rawEndCursor, rawPageSize, rawDirection, rawCursorHistory, defaultPageSize, defaultDirection])

  const updateSearch = useCallback(
    (updater: (draft: Record<string, unknown>) => Record<string, unknown>) => {
      navigate({
        search: ((prev: Record<string, unknown> | undefined) => {
          const draft = { ...((prev ?? {}) as Record<string, unknown>) }
          return updater(draft)
        }) as any,
        replace,
      } as any)
    },
    [navigate, replace]
  )

  const paginationArgs = useMemo<PaginationVariables>(() => {
    if (cursorDirection === 'before') {
      // When going backward, use the cursor from history with 'after' direction
      // to get the exact previous page
      const previousCursor = cursorHistory.length > 0 ? cursorHistory[cursorHistory.length - 1] : undefined
      return {
        first: pageSize,
        after: previousCursor,
      }
    }

    return {
      first: pageSize,
      after: endCursor,
    }
  }, [endCursor, cursorDirection, cursorHistory, pageSize])

  const setCursors = useCallback(
    (nextStartCursor: string | undefined, nextEndCursor: string | undefined, direction: CursorDirection = DEFAULT_DIRECTION) => {
      if (startCursor === nextStartCursor && endCursor === nextEndCursor && cursorDirection === direction) return

      updateSearch((draft) => {
        // Update cursor history based on direction
        let newHistory = [...cursorHistory]
        
        if (direction === 'after' && nextEndCursor) {
          // Moving forward: push current endCursor to history
          if (endCursor && !newHistory.includes(endCursor)) {
            newHistory.push(endCursor)
          }
        } else if (direction === 'before') {
          // Moving backward: pop from history
          if (newHistory.length > 0) {
            newHistory.pop()
          }
        }

        if (nextStartCursor && nextStartCursor.length > 0) {
          draft[startCursorKey] = nextStartCursor
        } else {
          delete draft[startCursorKey]
        }
        
        if (nextEndCursor && nextEndCursor.length > 0) {
          draft[endCursorKey] = nextEndCursor
        } else {
          delete draft[endCursorKey]
        }
        
        draft[directionKey] = direction
        
        // Store cursor history as JSON string
        if (newHistory.length > 0) {
          draft[cursorHistoryKey] = JSON.stringify(newHistory)
        } else {
          delete draft[cursorHistoryKey]
        }
        
        return draft
      })
    },
    [startCursor, endCursor, cursorDirection, cursorHistory, startCursorKey, endCursorKey, directionKey, cursorHistoryKey, updateSearch]
  )

  const setPageSize = useCallback(
    (nextPageSize: number) => {
      if (!Number.isFinite(nextPageSize) || nextPageSize <= 0) return

      const normalized = Math.floor(nextPageSize)
      if (normalized === pageSizeFromSearch && normalized === pageSize) return

      updateSearch((draft) => {
        draft[pageSizeKey] = normalized
        delete draft[startCursorKey]
        delete draft[endCursorKey]
        delete draft[directionKey]
        delete draft[cursorHistoryKey]
        return draft
      })
    },
    [startCursorKey, endCursorKey, directionKey, cursorHistoryKey, pageSize, pageSizeFromSearch, pageSizeKey, updateSearch]
  )

  const resetCursor = useCallback(() => {
    if (!startCursor && !endCursor && cursorHistory.length === 0) return

    updateSearch((draft) => {
      delete draft[startCursorKey]
      delete draft[endCursorKey]
      delete draft[directionKey]
      delete draft[cursorHistoryKey]
      return draft
    })
  }, [startCursor, endCursor, cursorHistory, startCursorKey, endCursorKey, directionKey, cursorHistoryKey, updateSearch])

  const getSearchParams = useCallback(() => {
    const params: Record<string, unknown> = {}
    
    if (startCursor) {
      params[startCursorKey] = startCursor
    }
    
    if (endCursor) {
      params[endCursorKey] = endCursor
    }
    
    if (pageSizeFromSearch !== undefined) {
      params[pageSizeKey] = pageSizeFromSearch
    }
    
    if (cursorDirection !== defaultDirection) {
      params[directionKey] = cursorDirection
    }
    
    if (cursorHistory.length > 0) {
      params[cursorHistoryKey] = JSON.stringify(cursorHistory)
    }
    
    return params
  }, [startCursor, endCursor, pageSizeFromSearch, cursorDirection, cursorHistory, startCursorKey, endCursorKey, pageSizeKey, directionKey, cursorHistoryKey, defaultDirection])

  const navigateWithSearch = useCallback((options: { to: string; params?: Record<string, any> }) => {
    navigate({
      to: options.to as any,
      params: options.params,
      search: getSearchParams(),
    } as any)
  }, [navigate, getSearchParams])

  return {
    startCursor,
    endCursor,
    pageSize,
    pageSizeFromSearch,
    paginationArgs,
    cursorHistory,
    setCursors,
    setPageSize,
    resetCursor,
    getSearchParams,
    navigateWithSearch,
  }
}
