import {
  IconAB2,
  IconActivity,
  IconAi,
  IconBaselineDensityMedium,
  IconChartBar,
  IconDatabase,
  IconKey,
  IconLayoutDashboard,
  IconNote,
  IconPackages,
  IconRobot,
  IconSettings,
  IconShield,
  IconUsers,
} from '@tabler/icons-react';

// Structural definition of the sidebar navigation. It intentionally contains no
// translations or auth logic so it can be shared by:
// - `sidebar.ts` (rendering, after permission + visibility filtering)
// - the "customize menu" dialog (listing the items a user may hide)
// - the sign-in landing fallback (picking a visible default route)
export interface NavItemDef {
  // i18n key, e.g. "sidebar.items.dashboard".
  titleKey: string;
  // Stable route path, also used as the persisted identifier when hiding items.
  url: string;
  icon: React.ElementType;
  mobileOnly?: boolean;
}

export interface NavGroupDef {
  id: string;
  titleKey: string;
  items: NavItemDef[];
}

export const NAV_GROUP_DEFS: NavGroupDef[] = [
  {
    id: 'admin',
    titleKey: 'sidebar.groups.admin',
    items: [
      { titleKey: 'sidebar.items.dashboard', url: '/', icon: IconLayoutDashboard },
      { titleKey: 'analytics.title', url: '/analytics', icon: IconChartBar },
      { titleKey: 'sidebar.items.projects', url: '/projects', icon: IconPackages },
      { titleKey: 'sidebar.items.channels', url: '/channels', icon: IconAi },
      { titleKey: 'sidebar.items.models', url: '/models', icon: IconRobot },
      { titleKey: 'sidebar.items.promptProtectionRules', url: '/prompt-protection-rules', icon: IconShield },
      { titleKey: 'sidebar.items.dataStorages', url: '/data-storages', icon: IconDatabase },
      { titleKey: 'sidebar.items.users', url: '/users', icon: IconUsers },
      { titleKey: 'sidebar.items.roles', url: '/roles', icon: IconShield },
    ],
  },
  {
    id: 'project',
    titleKey: 'sidebar.groups.project',
    items: [
      { titleKey: 'sidebar.items.apiKeys', url: '/project/api-keys', icon: IconKey },
      { titleKey: 'sidebar.items.prompts', url: '/project/prompts', icon: IconNote },
      { titleKey: 'sidebar.items.requests', url: '/project/requests', icon: IconActivity },
      { titleKey: 'sidebar.items.usageStats', url: '/project/usage-stats', icon: IconChartBar },
      { titleKey: 'sidebar.items.traces', url: '/project/traces', icon: IconAB2 },
      { titleKey: 'sidebar.items.threads', url: '/project/threads', icon: IconBaselineDensityMedium },
      { titleKey: 'sidebar.items.users', url: '/project/users', icon: IconUsers },
      { titleKey: 'sidebar.items.roles', url: '/project/roles', icon: IconShield },
      { titleKey: 'sidebar.items.playground', url: '/project/playground', icon: IconRobot },
    ],
  },
  {
    id: 'settings',
    titleKey: 'sidebar.groups.settings',
    items: [{ titleKey: 'sidebar.items.system', url: '/system', icon: IconSettings, mobileOnly: true }],
  },
];

// All navigable URLs in display order.
export const NAV_ITEM_URLS: string[] = NAV_GROUP_DEFS.flatMap((group) => group.items.map((item) => item.url));

// Project-scoped URLs, used as the landing fallback for non-owner users.
export const PROJECT_NAV_ITEM_URLS: string[] = (NAV_GROUP_DEFS.find((group) => group.id === 'project')?.items ?? []).map(
  (item) => item.url
);

// Pick a landing route that is not hidden. Falls back to `preferred` when the
// user has hidden every candidate.
export function pickFallbackNavUrl(preferred: string, hiddenItems: string[], isOwner: boolean): string {
  if (!hiddenItems.includes(preferred)) {
    return preferred;
  }

  const candidates = isOwner ? NAV_ITEM_URLS : PROJECT_NAV_ITEM_URLS;

  return candidates.find((url) => !hiddenItems.includes(url)) ?? preferred;
}
