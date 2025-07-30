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
} from '@tabler/icons-react'
import { Command } from 'lucide-react'
import { type SidebarData } from './components/layout/types'
import { useAuthStore } from '@/stores/authStore'
import { useMe } from '@/features/auth/data/auth'

export function useSidebarData(): SidebarData {
  const { user: authUser } = useAuthStore((state) => state.auth)
  const { data: meData } = useMe()
  
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
    navGroups: [
      {
        title: 'Admin',
        items: [
          {
            title: 'Dashboard',
            url: '/',
            icon: IconLayoutDashboard,
          },
          {
            title: 'Users',
            url: '/users',
            icon: IconUsers,
          },
          {
            title: 'Roles',
            url: '/roles',
            icon: IconShield,
          },
          {
            title: 'Channels',
            url: '/channels',
            icon: IconChecklist,
          },
        ],
      },
      {
        title: 'General',
        items: [
          {
            title: 'Requests',
            url: '/requests',
            icon: IconBrowserCheck,
          },
          {
            title: 'API Keys',
            url: '/api-keys',
            icon: IconPackages,
          },
          {
            title: 'Playground',
            url: '/playground',
            icon: IconRobot,
          },
          // {
          //   title: 'Chats',
          //   url: '/chats',
          //   badge: '3',
          //   icon: IconMessages,
          // },
          // {
          //   title: 'Secured by Clerk',
          //   icon: ClerkLogo,
          //   items: [
          //     {
          //       title: 'Sign In',
          //       url: '/clerk/sign-in',
          //     },
          //     {
          //       title: 'Sign Up',
          //       url: '/clerk/sign-up',
          //     },
          //     {
          //       title: 'User Management',
          //       url: '/clerk/user-management',
          //     },
          //   ],
          // },
        ],
      },

      {
        title: 'Settings',
        items: [
          {
            title: 'Profile',
            url: '/settings',
            icon: IconUserCog,
          },
          {
            title: 'Account',
            url: '/settings/account',
            icon: IconTool,
          },
          {
            title: 'Appearance',
            url: '/settings/appearance',
            icon: IconPalette,
          },
          {
            title: 'Notifications',
            url: '/settings/notifications',
            icon: IconNotification,
          },
          // {
          //   title: 'Display',
          //   url: '/settings/display',
          //   icon: IconBrowserCheck,
          // },
        ],
      },

      // {
      //   title: 'Help Center',
      //   url: '/help-center',
      //   icon: IconHelp,
      // },
    ],
  }
}
