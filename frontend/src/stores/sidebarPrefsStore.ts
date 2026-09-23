import { create } from 'zustand';

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

interface SidebarPrefsState {
  hiddenItems: string[];
  isHidden: (url: string) => boolean;
  toggleItem: (url: string) => void;
  setHiddenItems: (urls: string[]) => void;
  reset: () => void;
}

export const useSidebarPrefsStore = create<SidebarPrefsState>()((set, get) => ({
  hiddenItems: getHiddenNavItems(),

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
}));
