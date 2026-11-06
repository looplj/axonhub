import { LinkProps } from '@tanstack/react-router';
import type { NavModuleId } from '@/config/nav-items';

interface User {
  name: string;
  email: string;
  avatar: string;
}

interface Team {
  name: string;
  logo: React.ElementType;
  description: string;
}

interface BaseNavItem {
  title: string;
  badge?: string;
  icon?: React.ElementType;
  isDisabled?: boolean;
  params?: Record<string, string>;
  matchPath?: string;
}

type NavLink = BaseNavItem & {
  url: LinkProps['to'];
  items?: never;
  mobileOnly?: boolean;
};

type NavCollapsible = BaseNavItem & {
  items: (BaseNavItem & { url: LinkProps['to'] })[];
  url?: never;
};

type NavItem = NavCollapsible | NavLink;

interface NavGroup {
  id?: string;
  title: string;
  items: NavItem[];
}

interface NavModule {
  id: NavModuleId;
  title: string;
  icon: React.ElementType;
  groups: NavGroup[];
}

interface SidebarData {
  user: User;
  teams: Team[];
  // Every module the user can see, used by the header tabs and the command menu.
  modules: NavModule[];
  activeModuleId: NavModuleId | null;
  // Groups of the active module, rendered by the sidebar.
  navGroups: NavGroup[];
}

export type { SidebarData, NavModule, NavGroup, NavItem, NavCollapsible, NavLink };
