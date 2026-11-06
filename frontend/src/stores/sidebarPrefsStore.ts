import { create } from 'zustand';
import type { NavModuleId } from '@/config/nav-items';

// Persisted per browser, like the other UI-level preferences (column
// visibility, table sorting, sidebar collapsed state). Items are identified by
// their route path so translations can change without losing the preference.
export const HIDDEN_NAV_ITEMS_STORAGE_KEY = 'axonhub_hidden_nav_items';

export function getHiddenNavItems(): string[] {
  try {
    const raw = localStorage.getItem(HIDDEN_NAV_ITEMS_STORAGE_KEY);
    if (!raw) {
      return [];
    }

    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }

    return parsed.filter((item): item is string => typeof item === 'string');
  } catch {
    // localStorage unavailable or corrupt — fall back to showing everything.
    return [];
  }
}

function persistHiddenNavItems(items: string[]): void {
  try {
    localStorage.setItem(HIDDEN_NAV_ITEMS_STORAGE_KEY, JSON.stringify(items));
  } catch {
    // localStorage unavailable or quota exceeded — skip persistence.
  }
}

// Remembers the module the user was last in and the last page visited in each
// module, so switching tabs returns to where the user left off.
export const LAST_NAV_MODULE_STORAGE_KEY = 'axonhub_nav_last_module';

interface LastNavModuleState {
  lastModuleId: NavModuleId | null;
  lastModulePaths: Partial<Record<NavModuleId, string>>;
}

export function getLastNavModuleState(): LastNavModuleState {
  try {
    const raw = localStorage.getItem(LAST_NAV_MODULE_STORAGE_KEY);
    if (!raw) {
      return { lastModuleId: null, lastModulePaths: {} };
    }

    const parsed = JSON.parse(raw) as Partial<LastNavModuleState> | null;
    const lastModulePaths: Partial<Record<NavModuleId, string>> = {};
    if (parsed?.lastModulePaths && typeof parsed.lastModulePaths === 'object') {
      for (const [id, path] of Object.entries(parsed.lastModulePaths)) {
        if (typeof path === 'string') {
          lastModulePaths[id as NavModuleId] = path;
        }
      }
    }

    return {
      lastModuleId: typeof parsed?.lastModuleId === 'string' ? (parsed.lastModuleId as NavModuleId) : null,
      lastModulePaths,
    };
  } catch {
    // localStorage unavailable or corrupt — start without history.
    return { lastModuleId: null, lastModulePaths: {} };
  }
}

function persistLastNavModuleState(state: LastNavModuleState): void {
  try {
    localStorage.setItem(LAST_NAV_MODULE_STORAGE_KEY, JSON.stringify(state));
  } catch {
    // localStorage unavailable or quota exceeded — skip persistence.
  }
}

interface SidebarPrefsState extends LastNavModuleState {
  hiddenItems: string[];
  isHidden: (url: string) => boolean;
  toggleItem: (url: string) => void;
  setHiddenItems: (urls: string[]) => void;
  reset: () => void;
  recordModuleVisit: (moduleId: NavModuleId, path: string) => void;
}

export const useSidebarPrefsStore = create<SidebarPrefsState>()((set, get) => ({
  hiddenItems: getHiddenNavItems(),
  ...getLastNavModuleState(),

  isHidden: (url) => get().hiddenItems.includes(url),

  toggleItem: (url) =>
    set((state) => {
      const hiddenItems = state.hiddenItems.includes(url) ? state.hiddenItems.filter((item) => item !== url) : [...state.hiddenItems, url];

      persistHiddenNavItems(hiddenItems);

      return { hiddenItems };
    }),

  setHiddenItems: (urls) => {
    persistHiddenNavItems(urls);
    set({ hiddenItems: urls });
  },

  reset: () => {
    persistHiddenNavItems([]);
    set({ hiddenItems: [] });
  },

  recordModuleVisit: (moduleId, path) => {
    const { lastModuleId, lastModulePaths } = get();
    if (lastModuleId === moduleId && lastModulePaths[moduleId] === path) {
      return;
    }

    const next = { lastModuleId: moduleId, lastModulePaths: { ...lastModulePaths, [moduleId]: path } };
    persistLastNavModuleState(next);
    set(next);
  },
}));
