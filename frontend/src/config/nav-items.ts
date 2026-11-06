import {
  IconAB2,
  IconActivity,
  IconAi,
  IconBaselineDensityMedium,
  IconChartBar,
  IconDatabase,
  IconDatabaseCog,
  IconFlask,
  IconFolder,
  IconKey,
  IconLayoutDashboard,
  IconNote,
  IconPackages,
  IconPlugConnected,
  IconPuzzle,
  IconRefresh,
  IconRobot,
  IconSettings,
  IconShield,
  IconUserShield,
  IconUsers,
} from '@tabler/icons-react';

// Structural definition of the navigation. It intentionally contains no
// translations or auth logic so it can be shared by:
// - `sidebar.ts` (rendering, after permission + visibility filtering)
// - the "customize menu" dialog (listing the items a user may hide)
// - the sign-in landing fallback (picking a visible default route)
//
// Navigation is two-level: top-level modules are shown as tabs in the header,
// and the sidebar only renders the groups of the active module.
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
  // Omit for a group rendered without a heading.
  titleKey?: string;
  items: NavItemDef[];
}

export type NavModuleId = 'admin' | 'project' | 'plugins';

export interface NavModuleDef {
  id: NavModuleId;
  titleKey: string;
  icon: React.ElementType;
  groups: NavGroupDef[];
}

export const PLUGINS_NAV_GROUP_ID = 'plugins';

export const NAV_MODULE_DEFS: NavModuleDef[] = [
  {
    id: 'admin',
    titleKey: 'sidebar.modules.admin',
    icon: IconLayoutDashboard,
    groups: [
      {
        // The landing pages sit at the top without a group title.
        id: 'overview',
        items: [
          { titleKey: 'sidebar.items.dashboard', url: '/', icon: IconLayoutDashboard },
          { titleKey: 'sidebar.items.projects', url: '/projects', icon: IconPackages },
        ],
      },
      {
        id: 'gateway',
        titleKey: 'sidebar.groups.gateway',
        items: [
          { titleKey: 'sidebar.items.channels', url: '/channels', icon: IconAi },
          { titleKey: 'sidebar.items.models', url: '/models', icon: IconRobot },
          { titleKey: 'sidebar.items.promptProtectionRules', url: '/prompt-protection-rules', icon: IconShield },
          { titleKey: 'sidebar.items.dataStorages', url: '/data-storages', icon: IconDatabase },
        ],
      },
      {
        id: 'access-control',
        titleKey: 'sidebar.groups.accessControl',
        items: [
          { titleKey: 'sidebar.items.users', url: '/users', icon: IconUsers },
          { titleKey: 'sidebar.items.roles', url: '/roles', icon: IconUserShield },
        ],
      },
      {
        id: 'settings',
        titleKey: 'sidebar.groups.settings',
        items: [
          { titleKey: 'sidebar.items.system', url: '/system', icon: IconSettings },
          { titleKey: 'system.pages.requests.title', url: '/system/requests', icon: IconRefresh },
          { titleKey: 'system.pages.integrations.title', url: '/system/integrations', icon: IconPlugConnected },
          { titleKey: 'system.pages.operations.title', url: '/system/operations', icon: IconDatabaseCog },
        ],
      },
    ],
  },
  {
    id: 'project',
    titleKey: 'sidebar.modules.project',
    icon: IconFolder,
    groups: [
      {
        id: 'develop',
        titleKey: 'sidebar.groups.develop',
        items: [
          { titleKey: 'sidebar.items.apiKeys', url: '/project/api-keys', icon: IconKey },
          { titleKey: 'sidebar.items.prompts', url: '/project/prompts', icon: IconNote },
          { titleKey: 'sidebar.items.playground', url: '/project/playground', icon: IconFlask },
        ],
      },
      {
        id: 'observability',
        titleKey: 'sidebar.groups.observability',
        items: [
          { titleKey: 'sidebar.items.requests', url: '/project/requests', icon: IconActivity },
          { titleKey: 'sidebar.items.traces', url: '/project/traces', icon: IconAB2 },
          { titleKey: 'sidebar.items.threads', url: '/project/threads', icon: IconBaselineDensityMedium },
          { titleKey: 'sidebar.items.usageStats', url: '/project/usage-stats', icon: IconChartBar },
        ],
      },
      {
        id: 'members',
        titleKey: 'sidebar.groups.members',
        items: [
          { titleKey: 'sidebar.items.users', url: '/project/users', icon: IconUsers },
          { titleKey: 'sidebar.items.roles', url: '/project/roles', icon: IconUserShield },
        ],
      },
    ],
  },
  {
    id: 'plugins',
    titleKey: 'sidebar.modules.plugins',
    icon: IconPuzzle,
    groups: [
      {
        // Plugin pages are appended to this group at runtime by `useSidebarData`.
        id: PLUGINS_NAV_GROUP_ID,
        titleKey: 'plugins.nav.group',
        items: [{ titleKey: 'plugins.nav.manage', url: '/plugins', icon: IconPuzzle }],
      },
    ],
  },
];

export const NAV_GROUP_DEFS: NavGroupDef[] = NAV_MODULE_DEFS.flatMap((module) => module.groups);

// All navigable URLs in display order.
export const NAV_ITEM_URLS: string[] = NAV_GROUP_DEFS.flatMap((group) => group.items.map((item) => item.url));

function moduleItemUrls(id: NavModuleId): string[] {
  return (NAV_MODULE_DEFS.find((module) => module.id === id)?.groups ?? []).flatMap((group) => group.items.map((item) => item.url));
}

// Project-scoped URLs, used as the landing fallback for non-owner users.
export const PROJECT_NAV_ITEM_URLS: string[] = moduleItemUrls('project');

// Admin pages that are reachable but not listed in the sidebar.
const EXTRA_ADMIN_PATHS = ['/dashboard', '/api-keys'];

const ADMIN_PATHS: string[] = [...moduleItemUrls('admin'), ...EXTRA_ADMIN_PATHS];

function matchesPath(path: string, prefix: string): boolean {
  if (prefix === '/') {
    return path === '/';
  }
  return path === prefix || path.startsWith(`${prefix}/`);
}

// Maps a location to the module whose tab should be active. Returns null for
// pages that belong to no module (e.g. personal settings), so callers can keep
// showing the module the user came from.
export function resolveNavModule(pathname: string): NavModuleId | null {
  const path = pathname.split(/[?#]/)[0].replace(/\/+$/, '') || '/';

  if (matchesPath(path, '/project')) {
    return 'project';
  }
  if (matchesPath(path, '/plugins')) {
    return 'plugins';
  }
  if (ADMIN_PATHS.some((prefix) => matchesPath(path, prefix))) {
    return 'admin';
  }
  return null;
}

// Pick a landing route that is not hidden. Falls back to `preferred` when the
// user has hidden every candidate.
export function pickFallbackNavUrl(preferred: string, hiddenItems: string[], isOwner: boolean): string {
  if (!hiddenItems.includes(preferred)) {
    return preferred;
  }

  const candidates = isOwner ? NAV_ITEM_URLS : PROJECT_NAV_ITEM_URLS;

  return candidates.find((url) => !hiddenItems.includes(url)) ?? preferred;
}
