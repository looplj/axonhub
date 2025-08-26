import { useCallback, useMemo } from 'react'
import { useAuthStore } from '@/stores/authStore'
import { useMe } from '@/features/auth/data/auth'

/**
 * Hook for checking user permissions based on scopes
 * Provides utilities to check if user has specific permissions for actions
 */
export function usePermissions() {
  const { user: authUser } = useAuthStore((state) => state.auth)
  const { data: meData } = useMe()

  // Use data from me query if available, otherwise fall back to auth store
  const user = meData || authUser
  const isOwner = user?.isOwner || false

  // Check if user has a specific scope
  const hasScope = useCallback(
    (requiredScope: string): boolean => {
      // Owner has all permissions
      if (isOwner) {
        return true
      }

      // Check for wildcard permission
      if (user?.scopes?.includes('*')) {
        return true
      }

      // Check for specific scope
      return user?.scopes?.includes(requiredScope) || false
    },
    [user, isOwner]
  )

  // Check if user has any of the required scopes
  const hasAnyScope = useCallback(
    (requiredScopes: string[]): boolean => {
      if (requiredScopes.length === 0) {
        return true
      }

      return requiredScopes.some((scope) => hasScope(scope))
    },
    [hasScope]
  )

  // Check if user has all of the required scopes
  const hasAllScopes = useCallback(
    (requiredScopes: string[]): boolean => {
      if (requiredScopes.length === 0) {
        return true
      }

      return requiredScopes.every((scope) => hasScope(scope))
    },
    [hasScope]
  )

  // Common permission checks for channel operations
  const channelPermissions = useMemo(
    () => ({
      canRead: hasScope('read_channels'),
      canWrite: hasScope('write_channels'),
      canBulkImport: hasScope('write_channels'), // Bulk import requires write permission
      canCreate: hasScope('write_channels'),
      canEdit: hasScope('write_channels'),
      canDelete: hasScope('write_channels'),
      canTest: hasScope('read_channels'), // Test requires at least read permission
      canOrder: hasScope('write_channels'), // Ordering requires write permission
    }),
    [hasScope]
  )

  // Common permission checks for user operations
  const userPermissions = useMemo(
    () => ({
      canRead: hasScope('read_users'),
      canWrite: hasScope('write_users'),
      canCreate: hasScope('write_users'),
      canEdit: hasScope('write_users'),
      canDelete: hasScope('write_users'),
    }),
    [hasScope]
  )

  // Common permission checks for role operations
  const rolePermissions = useMemo(
    () => ({
      canRead: hasScope('read_roles'),
      canWrite: hasScope('write_roles'),
      canCreate: hasScope('write_roles'),
      canEdit: hasScope('write_roles'),
      canDelete: hasScope('write_roles'),
    }),
    [hasScope]
  )

  // Common permission checks for API key operations
  const apiKeyPermissions = useMemo(
    () => ({
      canRead: hasScope('read_api_keys'),
      canWrite: hasScope('write_api_keys'),
      canCreate: hasScope('write_api_keys'),
      canEdit: hasScope('write_api_keys'),
      canDelete: hasScope('write_api_keys'),
    }),
    [hasScope]
  )

  return {
    user,
    isOwner,
    hasScope,
    hasAnyScope,
    hasAllScopes,
    channelPermissions,
    userPermissions,
    rolePermissions,
    apiKeyPermissions,
  }
}
