// 路由权限配置
export type ScopeLevel = 'system' | 'project' | 'any';

export interface RouteConfig {
  path: string;
  requiredScopes?: string[];
  scopeLevel?: ScopeLevel; // 权限级别：system 只检查系统级权限，project 只检查项目级权限，any 检查两者
  mode?: 'hidden' | 'disabled'; // 当没有权限时的处理方式
  children?: RouteConfig[];
}

export interface RouteGroup {
  title: string;
  scopeLevel?: ScopeLevel; // 路由组的默认权限级别
  routes: RouteConfig[];
}

// 定义所有路由的权限配置
export const routeConfigs: RouteGroup[] = [
  {
    title: 'Admin',
    scopeLevel: 'system', // Admin 路由组只能通过 system-level 权限访问
    routes: [
      {
        path: '/',
        requiredScopes: ['read_dashboard'],
        mode: 'hidden',
      },
      {
        path: '/projects',
        requiredScopes: ['read_projects'],
        mode: 'hidden',
      },
      {
        path: '/users',
        requiredScopes: ['read_users'],
        mode: 'hidden',
      },
      {
        path: '/roles',
        requiredScopes: ['read_roles'],
        mode: 'hidden',
      },
      {
        path: '/channels',
        requiredScopes: ['read_channels'],
        mode: 'hidden',
      },
      {
        path: '/system',
        requiredScopes: ['read_system'],
        mode: 'hidden',
      },
      {
        path: '/permission-demo',
        // 权限演示页面所有用户都可以访问
      },
    ],
  },
  {
    title: 'Project',
    scopeLevel: 'any', // Project 路由组可以通过 system-level 或 project-level 权限访问
    routes: [
      {
        path: '/project/requests',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
      },
      {
        path: '/project/usage-logs',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
      },
      {
        path: '/project/api-keys',
        requiredScopes: ['read_api_keys'],
        mode: 'hidden',
      },
      {
        path: '/project/users',
        requiredScopes: ['read_users'],
        mode: 'hidden',
      },
      {
        path: '/project/roles',
        requiredScopes: ['read_roles'],
        mode: 'hidden',
      },
      {
        path: '/project/playground',
        // Playground is accessible to all users
      },
    ],
  },
  {
    title: 'Settings',
    routes: [
      {
        path: '/settings',
        // Profile 设置所有用户都可以访问
      },
      {
        path: '/settings/profile',
        // Profile 设置所有用户都可以访问
      },
      {
        path: '/settings/appearance',
        // Appearance 设置所有用户都可以访问
      },
      {
        path: '/settings/notifications',
        // Notifications 设置所有用户都可以访问
      },
    ],
  },
];

// 获取路由配置的辅助函数
export function getRouteConfig(path: string): RouteConfig | undefined {
  for (const group of routeConfigs) {
    for (const route of group.routes) {
      if (route.path === path) {
        return route;
      }
      if (route.children) {
        const childConfig = route.children.find((child) => child.path === path);
        if (childConfig) return childConfig;
      }
    }
  }
  return undefined;
}

// 检查用户是否有访问路由的权限
export function hasRouteAccess(userScopes: string[], routeConfig: RouteConfig): boolean {
  if (!routeConfig.requiredScopes || routeConfig.requiredScopes.length === 0) {
    return true;
  }

  // 如果用户有通配符权限，则拥有所有权限
  if (userScopes.includes('*')) {
    return true;
  }

  // 检查用户是否拥有所需的任一权限
  return routeConfig.requiredScopes.some((scope) => userScopes.includes(scope));
}

// 检查用户是否有访问路由组的权限
export function hasGroupAccess(userScopes: string[], group: RouteGroup): boolean {
  return group.routes.some((route) => hasRouteAccess(userScopes, route));
}
