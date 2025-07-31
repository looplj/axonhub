// 路由权限配置
export interface RouteConfig {
  path: string
  requiredScopes?: string[]
  mode?: 'hidden' | 'disabled' // 当没有权限时的处理方式
  children?: RouteConfig[]
}

export interface RouteGroup {
  title: string
  routes: RouteConfig[]
}

// 定义所有路由的权限配置
export const routeConfigs: RouteGroup[] = [
  {
    title: 'Admin',
    routes: [
      {
        path: '/',
        // Dashboard 不需要特殊权限，所有认证用户都可以访问
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
        path: '/permission-demo',
        // 权限演示页面所有用户都可以访问
      },
    ],
  },
  {
    title: 'General',
    routes: [
      {
        path: '/requests',
        requiredScopes: ['read_requests'],
        mode: 'hidden'
      },
      {
        path: '/api-keys',
        requiredScopes: ['read_api_keys'],
        mode: 'hidden'
      },
      {
        path: '/playground',
        requiredScopes: ['read_dashboard'], // 假设playground需要dashboard权限
        mode: 'hidden'
      }
    ]
  },
  {
    title: 'Settings',
    routes: [
      {
        path: '/settings',
        // Profile 设置所有用户都可以访问
      },
      {
        path: '/settings/account',
        // Account 设置所有用户都可以访问
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
  }
]

// 获取路由配置的辅助函数
export function getRouteConfig(path: string): RouteConfig | undefined {
  for (const group of routeConfigs) {
    for (const route of group.routes) {
      if (route.path === path) {
        return route
      }
      if (route.children) {
        const childConfig = route.children.find(child => child.path === path)
        if (childConfig) return childConfig
      }
    }
  }
  return undefined
}

// 检查用户是否有访问路由的权限
export function hasRouteAccess(userScopes: string[], routeConfig: RouteConfig): boolean {
  if (!routeConfig.requiredScopes || routeConfig.requiredScopes.length === 0) {
    return true
  }
  
  // 检查用户是否拥有所需的任一权限
  return routeConfig.requiredScopes.some(scope => userScopes.includes(scope))
}

// 检查用户是否有访问路由组的权限
export function hasGroupAccess(userScopes: string[], group: RouteGroup): boolean {
  return group.routes.some(route => hasRouteAccess(userScopes, route))
}