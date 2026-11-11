import { useMemo } from 'react';
import { useLocation } from '@tanstack/react-router';
import { NAV_MODULE_DEFS, PLUGINS_NAV_GROUP_ID, resolveNavModule, type NavModuleId } from '@/config/nav-items';
import { IconAppWindow } from '@tabler/icons-react';
import { Command } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { useSidebarPrefsStore } from '@/stores/sidebarPrefsStore';
import { formatUserName, isCJKName } from '@/lib/utils';
import { useIsMobile } from '@/hooks/use-mobile';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';
import { useMe } from '@/features/auth/data/auth';
import { usePluginUIEntries } from '@/features/plugins/data/plugin-ui';
import { resolvePluginTitle } from '@/features/plugins/plugin-ui';
import { type SidebarData, type NavGroup, type NavLink, type NavModule } from './components/layout/types';

// Translates the structural navigation definition into renderable modules.
// Shared with the "customize menu" dialog so it can list the same items.
export function useRawNavModules(): NavModule[] {
  const { t } = useTranslation();

  return useMemo(
    () =>
      NAV_MODULE_DEFS.map((module) => ({
        id: module.id,
        title: t(module.titleKey),
        icon: module.icon,
        groups: module.groups.map((group) => ({
          id: group.id,
          title: group.titleKey ? t(group.titleKey) : '',
          items: group.items.map(
            (item) =>
              ({
                title: t(item.titleKey),
                url: item.url,
                icon: item.icon,
                mobileOnly: item.mobileOnly,
              }) as NavLink
          ),
        })),
      })),
    [t]
  );
}

// Removes hidden items and drops groups that become empty. Pure so it can be
// unit tested without React.
export function applyHiddenNavItems(groups: NavGroup[], hiddenItems: string[]): NavGroup[] {
  if (hiddenItems.length === 0) {
    return groups;
  }

  return groups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => !('url' in item) || !hiddenItems.includes(item.url as string)),
    }))
    .filter((group) => group.items.length > 0);
}

// Appends plugin pages to the plugins group. The static group disappears when
// the user cannot or chose not to see "Manage plugins", so it is recreated.
export function mergePluginPages(groups: NavGroup[], pageItems: NavLink[], title: string): NavGroup[] {
  const index = groups.findIndex((group) => group.id === PLUGINS_NAV_GROUP_ID);
  if (index >= 0) {
    return groups.map((group, i) => (i === index ? { ...group, items: [...group.items, ...pageItems] } : group));
  }
  return [...groups, { id: PLUGINS_NAV_GROUP_ID, title, items: pageItems }];
}

// The tab to highlight: the module owning the current route, otherwise the
// module the user was last in, otherwise the first visible one.
export function pickActiveModuleId(
  visibleIds: NavModuleId[],
  routeModuleId: NavModuleId | null,
  lastModuleId: NavModuleId | null
): NavModuleId | null {
  if (routeModuleId && visibleIds.includes(routeModuleId)) {
    return routeModuleId;
  }
  if (lastModuleId && visibleIds.includes(lastModuleId)) {
    return lastModuleId;
  }
  return visibleIds[0] ?? null;
}

// First enabled link of a module, used as the landing page of its tab.
// Mobile-only entries are a last resort.
export function firstNavLink(module: NavModule): NavLink | undefined {
  const links = module.groups
    .flatMap((group) => group.items)
    .filter((item): item is NavLink => !item.isDisabled && 'url' in item && !!item.url);
  return links.find((link) => !link.mobileOnly) ?? links[0];
}

export function useSidebarData(): SidebarData {
  const { t, i18n } = useTranslation();
  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const { filterNavGroups } = useRoutePermissions();
  const hiddenItems = useSidebarPrefsStore((state) => state.hiddenItems);
  const lastModuleId = useSidebarPrefsStore((state) => state.lastModuleId);
  const pathname = useLocation({ select: (location) => location.pathname });
  const isMobile = useIsMobile();

  // Use data from me query if available, otherwise fall back to auth store
  const user = meData || authUser;

  // Generate user initials for avatar
  const getInitials = (firstName?: string, lastName?: string, email?: string) => {
    if (firstName && lastName) {
      const [first, second] = isCJKName(firstName, lastName) ? [lastName, firstName] : [firstName, lastName];
      return `${first.charAt(0)}${second.charAt(0)}`.toUpperCase();
    }
    if (firstName) {
      return firstName.slice(0, 2).toUpperCase();
    }
    if (email) {
      return email.split('@')[0].slice(0, 2).toUpperCase();
    }
    return 'U';
  };

  // Generate user display name
  const getDisplayName = (firstName?: string, lastName?: string, email?: string) => {
    if (firstName && lastName) {
      return formatUserName(firstName, lastName);
    }
    if (firstName) {
      return firstName;
    }
    if (email) {
      const username = email.split('@')[0];
      return username.charAt(0).toUpperCase() + username.slice(1);
    }
    return 'User';
  };

  const rawNavModules = useRawNavModules();
  const { data: pluginEntries } = usePluginUIEntries();

  // Filter by permission first, then by the user's own visibility preferences.
  // Plugin pages are added to the plugins module after that filter. They are
  // not static nav items and they are not gated by route-permission.ts.
  // Modules left without items are dropped so they get no header tab.
  const modules = useMemo(() => {
    const pages = pluginEntries?.pluginUIEntries.pages ?? [];
    const pageItems = pages.map(
      (page) =>
        ({
          title: resolvePluginTitle(page.title, i18n.language, page.key),
          url: '/plugins/$pluginId/pages/$page',
          params: { pluginId: page.pluginID, page: page.key },
          matchPath: `/plugins/${page.pluginID}/pages/${page.key}`,
          icon: IconAppWindow,
        }) as NavLink
    );

    return rawNavModules
      .map((module) => {
        let groups = applyHiddenNavItems(filterNavGroups(module.groups), hiddenItems).filter((group) =>
          group.items.some((item) => !item.isDisabled)
        );
        if (module.id === 'plugins' && pageItems.length > 0) {
          groups = mergePluginPages(groups, pageItems, t('plugins.nav.group'));
        }
        return { ...module, groups };
      })
      // Mobile-only items stay in the data (the command menu lists them) but do
      // not earn a module its tab on desktop.
      .filter((module) =>
        module.groups.some((group) => group.items.some((item) => !item.isDisabled && (isMobile || !(item as NavLink).mobileOnly)))
      );
  }, [filterNavGroups, rawNavModules, hiddenItems, isMobile, pluginEntries, i18n.language, t]);

  const activeModuleId = pickActiveModuleId(
    modules.map((module) => module.id),
    resolveNavModule(pathname),
    lastModuleId
  );
  const navGroups = modules.find((module) => module.id === activeModuleId)?.groups ?? [];

  return {
    user: {
      name: getDisplayName(user?.firstName, user?.lastName, user?.email),
      email: user?.email || 'user@example.com',
      avatar: user?.avatar || getInitials(user?.firstName, user?.lastName, user?.email),
    },
    teams: [
      {
        name: t('sidebar.team.name'),
        logo: Command,
        description: '',
        // DO NOT USE THIS
        // plan: t('sidebar.team.plan'),
      },
    ],
    modules,
    activeModuleId,
    navGroups,
  };
}
