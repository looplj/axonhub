import {
  IconBarrierBlock,
  IconBrowserCheck,
  IconBug,
  IconChecklist,
  IconError404,
  IconHelp,
  IconLayoutDashboard,
  IconLock,
  IconLockAccess,
  IconMessages,
  IconNotification,
  IconPackages,
  IconPalette,
  IconServerOff,
  IconSettings,
  IconTool,
  IconUserCog,
  IconUserOff,
  IconUsers,
  IconRobot,
  IconShield,
} from '@tabler/icons-react'
import { AudioWaveform, Command, GalleryVerticalEnd } from 'lucide-react'
import { ClerkLogo } from '@/assets/clerk-logo'
import { type SidebarData } from './components/layout/types'

export const sidebarData: SidebarData = {
  user: {
    name: 'Admin',
    email: 'admin@axonhub.com',
    avatar: '/avatars/shadcn.jpg',
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
