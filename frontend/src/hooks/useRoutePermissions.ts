import { useMemo } from 'react'
import { useAuthStore } from '@/stores/authStore'
import { useMe } from '@/features/auth/data/auth'
import { routeConfigs, hasRouteAccess, hasGroupAccess, type RouteConfig, type RouteGroup } from '@/config/route-permission'
import { type NavGroup, type NavItem } from '@/components/layout/types'

export function useRoutePermissions() {
  const { user: authUser } = useAuthStore((state) => state.auth)
  const { data: meData } = useMe()
  
  // Use data from me query if available, otherwise fall back to auth store
  const user = meData || authUser
  const userScopes = user?.scopes || []
  const isOwner = user?.isOwner || false

  // Owner用户拥有所有权限
  const effectiveScopes = useMemo(() => {
    if (isOwner) {
      // Owner拥有通配符权限，可以访问所有路由
      return ['*']
    }
    return userScopes
  }, [userScopes, isOwner])

  // 检查单个路由权限
  const checkRouteAccess = useMemo(() => {
    return (path: string): { hasAccess: boolean; mode?: 'hidden' | 'disabled' } => {
      const routeConfig = getRouteConfigByPath(path)
      if (!routeConfig) {
        return { hasAccess: true }
      }
      
      const hasAccess = hasRouteAccess(effectiveScopes, routeConfig)
      return {
        hasAccess,
        mode: routeConfig.mode
      }
    }
  }, [effectiveScopes])

  // 检查路由组权限
  const checkGroupAccess = useMemo(() => {
    return (group: RouteGroup): boolean => {
      return hasGroupAccess(effectiveScopes, group)
    }
  }, [effectiveScopes])

  // 过滤导航项
  const filterNavItems = useMemo(() => {
    return (items: NavItem[]): NavItem[] => {
      return items.filter(item => {
        if ('url' in item) {
          const access = checkRouteAccess(item.url as string)
          
          // 如果是隐藏模式且没有权限，则过滤掉
          if (!access.hasAccess && access.mode === 'hidden') {
            return false
          }
        }
        
        return true
      }).map(item => {
        if ('url' in item) {
          const access = checkRouteAccess(item.url as string)
          
          return {
            ...item,
            isDisabled: !access.hasAccess && access.mode === 'disabled'
          }
        }
        
        return item
      })
    }
  }, [checkRouteAccess])

  // 过滤导航组
  const filterNavGroups = useMemo(() => {
    return (groups: NavGroup[]): NavGroup[] => {
      return groups.filter(group => {
        // 找到对应的路由组配置
        const routeGroup = routeConfigs.find(rg => rg.title === group.title)
        if (!routeGroup) {
          return true // 如果没有配置，默认显示
        }
        
        // 检查组是否有可访问的路由
        return checkGroupAccess(routeGroup)
      }).map(group => ({
        ...group,
        items: filterNavItems(group.items)
      }))
    }
  }, [checkGroupAccess, filterNavItems])

  return {
    userScopes: effectiveScopes,
    isOwner,
    checkRouteAccess,
    checkGroupAccess,
    filterNavItems,
    filterNavGroups
  }
}

// 辅助函数：根据路径查找路由配置
function getRouteConfigByPath(path: string): RouteConfig | undefined {
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