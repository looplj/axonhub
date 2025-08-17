import {
  IconBrowserCheck,
  IconChecklist,
  IconLayoutDashboard,
  IconPackages,
  IconUserCog,
  IconUsers,
  IconRobot,
  IconShield,
  IconSettings,
} from '@tabler/icons-react'
import { Command } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/authStore'
import { useRoutePermissions } from '@/hooks/useRoutePermissions'
import { useMe } from '@/features/auth/data/auth'
import {
  type SidebarData,
  type NavGroup,
  type NavLink,
} from './components/layout/types'

export function useSidebarData(): SidebarData {
  const { t } = useTranslation()
  const { user: authUser } = useAuthStore((state) => state.auth)
  const { data: meData } = useMe()
  const { filterNavGroups } = useRoutePermissions()

  // Use data from me query if available, otherwise fall back to auth store
  const user = meData || authUser

  // Generate user initials for avatar
  const getInitials = (
    firstName?: string,
    lastName?: string,
    email?: string
  ) => {
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
  const getDisplayName = (
    firstName?: string,
    lastName?: string,
    email?: string
  ) => {
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
      title: t('sidebar.groups.admin'),
      items: [
        {
          title: t('sidebar.items.dashboard'),
          url: '/',
          icon: IconLayoutDashboard,
        } as NavLink,
        {
          title: t('sidebar.items.channels'),
          url: '/channels',
          icon: IconChecklist,
        } as NavLink,
        {
          title: t('sidebar.items.users'),
          url: '/users',
          icon: IconUsers,
        } as NavLink,
        {
          title: t('sidebar.items.roles'),
          url: '/roles',
          icon: IconShield,
        } as NavLink,

        {
          title: t('sidebar.items.system'),
          url: '/system',
          icon: IconSettings,
        } as NavLink,
        // {
        //   title: 'Permission Demo',
        //   url: '/permission-demo',
        //   icon: IconSettings,
        // } as NavLink,
      ],
    },
    {
      title: t('sidebar.groups.api'),
      items: [
        {
          title: t('sidebar.items.requests'),
          url: '/requests',
          icon: IconBrowserCheck,
        } as NavLink,
        {
          title: t('sidebar.items.apiKeys'),
          url: '/api-keys',
          icon: IconPackages,
        } as NavLink,
        {
          title: t('sidebar.items.playground'),
          url: '/playground',
          icon: IconRobot,
        } as NavLink,
      ],
    },
    {
      title: t('sidebar.groups.settings'),
      items: [
        {
          title: t('sidebar.items.profile'),
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
      avatar: user?.avatar || getInitials(user?.firstName, user?.lastName, user?.email),
    },
    teams: [
      {
        name: t('sidebar.team.name'),
        logo: Command,
        plan: '',
        // DO NOT USE THIS
        // plan: t('sidebar.team.plan'),
      },
    ],
    navGroups: filteredNavGroups,
  }
}
