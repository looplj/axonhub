import {
  IconBrowserCheck,
  IconChecklist,
  IconLayoutDashboard,
  IconNotification,
  IconPackages,
  IconPalette,
  IconTool,
  IconUserCog,
  IconUsers,
  IconRobot,
  IconShield,
  IconSettings,
} from '@tabler/icons-react'
import { Command } from 'lucide-react'
import { type SidebarData, type NavGroup, type NavLink } from './components/layout/types'
import { useAuthStore } from '@/stores/authStore'
import { useMe } from '@/features/auth/data/auth'
import { useRoutePermissions } from '@/hooks/useRoutePermissions'

export function useSidebarData(): SidebarData {
  const { user: authUser } = useAuthStore((state) => state.auth)
  const { data: meData } = useMe()
  const { filterNavGroups } = useRoutePermissions()
  
  // Use data from me query if available, otherwise fall back to auth store
  const user = meData || authUser
  
  // Generate user initials for avatar
  const getInitials = (firstName?: string, lastName?: string, email?: string) => {
    if (firstName && lastName) {
      return `${firstName.charAt(0)}${lastName.charAt(0)}`.toUpperCase()
    }
    if (firstName) {
      return firstName.slice(0, 2).toUpperCase()
    }
    if (email) {
      return email.split('@')[0].slice(0, 2).toUpperCase()
    }
    return 'U'
  }
  
  // Generate user display name
  const getDisplayName = (firstName?: string, lastName?: string, email?: string) => {
    if (firstName && lastName) {
      return `${firstName} ${lastName}`
    }
    if (firstName) {
      return firstName
    }
    if (email) {
      const username = email.split('@')[0]
      return username.charAt(0).toUpperCase() + username.slice(1)
    }
    return 'User'
  }

  // 原始导航组配置
  const rawNavGroups: NavGroup[] = [
    {
      title: 'Admin',
      items: [
        {
          title: 'Dashboard',
          url: '/',
          icon: IconLayoutDashboard,
        } as NavLink,
        {
          title: 'Users',
          url: '/users',
          icon: IconUsers,
        } as NavLink,
        {
          title: 'Roles',
          url: '/roles',
          icon: IconShield,
        } as NavLink,
        {
          title: 'System',
          url: '/system',
          icon: IconSettings,
        } as NavLink,
        {
          title: 'Channels',
          url: '/channels',
          icon: IconChecklist,
        } as NavLink,
        // {
        //   title: 'Permission Demo',
        //   url: '/permission-demo',
        //   icon: IconSettings,
        // } as NavLink,
      ],
    },
    {
      title: 'General',
      items: [
        {
          title: 'Requests',
          url: '/requests',
          icon: IconBrowserCheck,
        } as NavLink,
        {
          title: 'API Keys',
          url: '/api-keys',
          icon: IconPackages,
        } as NavLink,
        {
          title: 'Playground',
          url: '/playground',
          icon: IconRobot,
        } as NavLink,
      ],
    },
    {
      title: 'Settings',
      items: [
        {
          title: 'Profile',
          url: '/settings',
          icon: IconUserCog,
        } as NavLink,
        // {
        //   title: 'Account',
        //   url: '/settings/account',
        //   icon: IconTool,
        // } as NavLink,
        // {
        //   title: 'Appearance',
        //   url: '/settings/appearance',
        //   icon: IconPalette,
        // } as NavLink,
        // {
        //   title: 'Notifications',
        //   url: '/settings/notifications',
        //   icon: IconNotification,
        // } as NavLink,
      ],
    },
  ]

  // 使用权限过滤导航组
  const filteredNavGroups = filterNavGroups(rawNavGroups)

  return {
    user: {
      name: getDisplayName(user?.firstName, user?.lastName, user?.email),
      email: user?.email || 'user@example.com',
      avatar: getInitials(user?.firstName, user?.lastName, user?.email),
    },
    teams: [
      {
        name: 'AxonHub Admin',
        logo: Command,
        plan: 'AI + Unified',
      },
    ],
    navGroups: filteredNavGroups,
  }
}
