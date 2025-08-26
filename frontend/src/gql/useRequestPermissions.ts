import { useMemo } from 'react'
import { useAuthStore } from '@/stores/authStore'
import { useMe } from '@/features/auth/data/auth'

export interface RequestPermissions {
  canViewUsers: boolean
  canViewApiKeys: boolean
  canViewChannels: boolean
}

export function useRequestPermissions(): RequestPermissions {
  const { user: authUser } = useAuthStore((state) => state.auth)
  const { data: meData } = useMe()

  // Use data from me query if available, otherwise fall back to auth store
  const user = meData || authUser
  const isOwner = user?.isOwner || false

  const permissions = useMemo(() => {
    const userScopes = user?.scopes || []
    // Owner用户拥有所有权限
    if (isOwner || userScopes.includes('*')) {
      return {
        canViewUsers: true,
        canViewApiKeys: true,
        canViewChannels: true,
      }
    }

    return {
      canViewUsers: userScopes.includes('read_users'),
      canViewApiKeys: userScopes.includes('read_api_keys'),
      canViewChannels: userScopes.includes('read_channels'),
    }
  }, [user, isOwner])

  return permissions
}