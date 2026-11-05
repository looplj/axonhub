import { ReactNode } from 'react';
import { Link, useLocation } from '@tanstack/react-router';
import { ChevronRight } from 'lucide-react';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  useSidebar,
} from '@/components/ui/sidebar';
import { Badge } from '../ui/badge';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../ui/dropdown-menu';
import { NavCollapsible, NavItem, NavLink, type NavGroup } from './types';

export function NavGroup({ title, items }: NavGroup) {
  const { state, isMobile } = useSidebar();
  const href = useLocation({ select: (location) => location.href });

  const visibleItems = items.filter((item) => {
    if (!item.items) {
      if (item.isDisabled) return false;
      if ((item as NavLink).mobileOnly && !isMobile) return false;
      return true;
    }

    const hasVisibleSubItems = item.items.some((subItem) => !subItem.isDisabled);
    return hasVisibleSubItems && !item.isDisabled;
  });

  // 如果没有可见的菜单项，不渲染整个分组
  if (visibleItems.length === 0) {
    return null;
  }

  return (
    <SidebarGroup>
      <SidebarGroupLabel>{title}</SidebarGroupLabel>
      <SidebarMenu>
        {visibleItems.map((item) => {
          const key = `${item.title}-${navTarget(item)}`;

          if (!item.items) return <SidebarMenuLink key={key} item={item} href={href} />;

          if (state === 'collapsed' && !isMobile) return <SidebarMenuCollapsedDropdown key={key} item={item} href={href} />;

          return <SidebarMenuCollapsible key={key} item={item} href={href} />;
        })}
      </SidebarMenu>
    </SidebarGroup>
  );
}

const NavBadge = ({ children }: { children: ReactNode }) => <Badge className='rounded-full px-1 py-0 text-xs'>{children}</Badge>;

const SidebarMenuLink = ({ item, href }: { item: NavLink; href: string }) => {
  const { setOpenMobile } = useSidebar();

  return (
    <SidebarMenuItem>
      <SidebarMenuButton asChild isActive={checkIsActive(href, item)} tooltip={item.title} className='h-12 rounded-2xl transition-all'>
        <Link {...navLinkProps(item)} onClick={() => setOpenMobile(false)}>
          {item.icon && <item.icon className='text-xl' />}
          <span>{item.title}</span>
          {item.badge && <NavBadge>{item.badge}</NavBadge>}
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
};

const SidebarMenuCollapsible = ({ item, href }: { item: NavCollapsible; href: string }) => {
  const { setOpenMobile } = useSidebar();
  // 只渲染未被禁用的子项
  const visibleSubItems = item.items.filter((subItem) => !subItem.isDisabled);

  return (
    <Collapsible asChild defaultOpen={checkIsActive(href, item, true)} className='group/collapsible'>
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton tooltip={item.title} className='h-12 rounded-2xl transition-all'>
            {item.icon && <item.icon className='text-xl' />}
            <span>{item.title}</span>
            {item.badge && <NavBadge>{item.badge}</NavBadge>}
            <ChevronRight className='ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90' />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent className='CollapsibleContent'>
          <SidebarMenuSub>
            {visibleSubItems.map((subItem) => (
              <SidebarMenuSubItem key={subItem.title}>
                <SidebarMenuSubButton asChild isActive={checkIsActive(href, subItem)}>
                  <Link {...navLinkProps(subItem)} onClick={() => setOpenMobile(false)}>
                    {subItem.icon && <subItem.icon />}
                    <span>{subItem.title}</span>
                    {subItem.badge && <NavBadge>{subItem.badge}</NavBadge>}
                  </Link>
                </SidebarMenuSubButton>
              </SidebarMenuSubItem>
            ))}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
};

const SidebarMenuCollapsedDropdown = ({ item, href }: { item: NavCollapsible; href: string }) => {
  // 只渲染未被禁用的子项
  const visibleSubItems = item.items.filter((subItem) => !subItem.isDisabled);

  return (
    <SidebarMenuItem>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <SidebarMenuButton tooltip={item.title} isActive={checkIsActive(href, item)} className='h-12 rounded-2xl transition-all'>
            {item.icon && <item.icon className='text-xl' />}
            <span>{item.title}</span>
            {item.badge && <NavBadge>{item.badge}</NavBadge>}
            <ChevronRight className='ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90' />
          </SidebarMenuButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent side='right' align='start' sideOffset={4}>
          <DropdownMenuLabel>
            {item.title} {item.badge ? `(${item.badge})` : ''}
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          {visibleSubItems.map((sub) => (
            <DropdownMenuItem key={`${sub.title}-${sub.url}`} asChild>
              <Link {...navLinkProps(sub)} className={`${checkIsActive(href, sub) ? 'bg-secondary' : ''}`}>
                {sub.icon && <sub.icon />}
                <span className='max-w-52 text-wrap'>{sub.title}</span>
                {sub.badge && <span className='ml-auto text-xs'>{sub.badge}</span>}
              </Link>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </SidebarMenuItem>
  );
};

function navTarget(item: { url?: string; matchPath?: string }) {
  return item.matchPath || item.url || '';
}

function navLinkProps(item: { url: NavLink['url']; params?: Record<string, string> }): { to: NavLink['url'] } {
  return {
    to: item.url,
    ...(item.params ? { params: item.params } : {}),
  } as { to: NavLink['url'] };
}

function checkIsActive(href: string, item: NavItem, mainNav = false) {
  const path = href.split('?')[0];
  const target = navTarget(item);
  return (
    href === target ||
    path === target ||
    !!item?.items?.some((subItem) => navTarget(subItem) === href || navTarget(subItem) === path) ||
    (mainNav && path.split('/')[1] !== '' && path.split('/')[1] === String(item.url ?? '').split('/')[1])
  );
}
