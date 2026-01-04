import { useMemo } from 'react';
import { routeConfigs, type RouteConfig, type RouteGroup, type ScopeLevel } from '@/config/route-permission';
import { useAuthStore } from '@/stores/authStore';
import { useSelectedProjectId } from '@/stores/projectStore';
import { type NavGroup, type NavItem } from '@/components/layout/types';
import { useMe } from '@/features/auth/data/auth';

export function useRoutePermissions() {
  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const selectedProjectId = useSelectedProjectId();

  // Use data from me query if available, otherwise fall back to auth store
  const user = meData || authUser;
  const systemScopes = user?.scopes || [];
  const isOwner = user?.isOwner || false;

  // Get project-level scopes for the selected project
  const projectScopes = useMemo(() => {
    if (!selectedProjectId || !user?.projects) {
      return [];
    }
    const project = user.projects.find((p) => p.projectID === selectedProjectId);
    return project?.scopes || [];
  }, [selectedProjectId, user?.projects]);

  // 检查路由权限（根据 scopeLevel 决定检查哪个级别的权限）
  const hasRouteAccess = (routeConfig: RouteConfig, groupScopeLevel?: ScopeLevel): boolean => {
    if (!routeConfig.requiredScopes || routeConfig.requiredScopes.length === 0) {
      return true;
    }

    // Owner 拥有所有权限
    if (isOwner) {
      return true;
    }

    // 确定要检查的权限级别（路由配置优先，否则使用组配置，默认为 'any'）
    const scopeLevel = routeConfig.scopeLevel || groupScopeLevel || 'any';

    // 根据 scopeLevel 决定检查哪些 scopes
    let scopesToCheck: string[] = [];

    if (scopeLevel === 'system') {
      // 只检查系统级权限
      scopesToCheck = systemScopes;
    } else if (scopeLevel === 'project') {
      // 只检查项目级权限
      scopesToCheck = projectScopes;
    } else {
      // 检查系统级和项目级权限
      scopesToCheck = [...systemScopes, ...projectScopes];
    }

    // 检查通配符权限
    if (scopesToCheck.includes('*')) {
      return true;
    }

    // 检查是否拥有所需的任一权限
    return routeConfig.requiredScopes.some((scope) => scopesToCheck.includes(scope));
  };

  // 检查路由组权限
  const hasGroupAccess = (group: RouteGroup): boolean => {
    return group.routes.some((route) => hasRouteAccess(route, group.scopeLevel));
  };

  // 检查单个路由权限
  const checkRouteAccess = useMemo(() => {
    return (path: string): { hasAccess: boolean; mode?: 'hidden' | 'disabled' } => {
      const { routeConfig, groupScopeLevel } = getRouteConfigByPathWithGroup(path);
      if (!routeConfig) {
        return { hasAccess: true };
      }

      const access = hasRouteAccess(routeConfig, groupScopeLevel);
      return {
        hasAccess: access,
        mode: routeConfig.mode,
      };
    };
  }, [systemScopes, projectScopes, isOwner]);

  // 检查路由组权限
  const checkGroupAccess = useMemo(() => {
    return (group: RouteGroup): boolean => {
      return hasGroupAccess(group);
    };
  }, [systemScopes, projectScopes, isOwner]);

  // 过滤导航项
  const filterNavItems = useMemo(() => {
    return (items: NavItem[]): NavItem[] => {
      return items
        .filter((item) => {
          if ('url' in item) {
            const access = checkRouteAccess(item.url as string);

            // 如果是隐藏模式且没有权限，则过滤掉
            if (!access.hasAccess && access.mode === 'hidden') {
              return false;
            }
          }

          return true;
        })
        .map((item) => {
          if ('url' in item) {
            const access = checkRouteAccess(item.url as string);

            return {
              ...item,
              isDisabled: !access.hasAccess && access.mode === 'disabled',
            };
          }

          return item;
        });
    };
  }, [checkRouteAccess]);

  // 过滤导航组
  const filterNavGroups = useMemo(() => {
    return (groups: NavGroup[]): NavGroup[] => {
      return groups
        .filter((group) => {
          // 找到对应的路由组配置
          const routeGroup = routeConfigs.find((rg) => rg.title === group.title);
          if (!routeGroup) {
            return true; // 如果没有配置，默认显示
          }

          // 检查组是否有可访问的路由
          return checkGroupAccess(routeGroup);
        })
        .map((group) => ({
          ...group,
          items: filterNavItems(group.items),
        }));
    };
  }, [checkGroupAccess, filterNavItems]);

  return {
    userScopes: [...systemScopes, ...projectScopes],
    isOwner,
    checkRouteAccess,
    checkGroupAccess,
    filterNavItems,
    filterNavGroups,
  };
}

// 辅助函数：根据路径查找路由配置及其所属组的 scopeLevel
function getRouteConfigByPathWithGroup(path: string): {
  routeConfig?: RouteConfig;
  groupScopeLevel?: ScopeLevel;
} {
  for (const group of routeConfigs) {
    for (const route of group.routes) {
      if (route.path === path) {
        return { routeConfig: route, groupScopeLevel: group.scopeLevel };
      }
      if (route.children) {
        const childConfig = route.children.find((child) => child.path === path);
        if (childConfig) {
          return { routeConfig: childConfig, groupScopeLevel: group.scopeLevel };
        }
      }
    }
  }
  return {};
}
