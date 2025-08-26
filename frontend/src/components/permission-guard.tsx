import React from 'react'
import { usePermissions } from '@/hooks/usePermissions'

export interface PermissionGuardProps {
  children?: React.ReactNode
  /** Required scope(s) - user must have at least one of these scopes */
  requiredScopes?: string[]
  /** All scopes required - user must have all of these scopes */
  requiredAllScopes?: string[]
  /** Single scope required - shorthand for requiredScopes with one scope */
  requiredScope?: string
  /** Whether to render children when permission is denied (default: false) */
  showWhenDenied?: boolean
  /** Custom component to render when permission is denied */
  fallback?: React.ReactNode
  /** Render function that receives permission state */
  render?: (hasPermission: boolean) => React.ReactNode
}

/**
 * PermissionGuard component wraps content and conditionally renders it based on user permissions.
 * 
 * @example
 * // Show button only if user has write_channels scope
 * <PermissionGuard requiredScope="write_channels">
 *   <Button>Bulk Import</Button>
 * </PermissionGuard>
 * 
 * @example
 * // Show button only if user has any of the specified scopes
 * <PermissionGuard requiredScopes={["write_channels", "admin"]}>
 *   <Button>Admin Action</Button>
 * </PermissionGuard>
 * 
 * @example
 * // Show button only if user has all specified scopes
 * <PermissionGuard requiredAllScopes={["read_users", "write_channels"]}>
 *   <Button>Complex Action</Button>
 * </PermissionGuard>
 * 
 * @example
 * // Use render prop for more complex logic
 * <PermissionGuard 
 *   requiredScope="write_channels"
 *   render={(hasPermission) => (
 *     <Button disabled={!hasPermission}>
 *       {hasPermission ? "Bulk Import" : "No Permission"}
 *     </Button>
 *   )}
 * />
 * 
 * @example
 * // Show fallback when denied
 * <PermissionGuard 
 *   requiredScope="write_channels"
 *   fallback={<span className="text-muted-foreground">Permission required</span>}
 * >
 *   <Button>Bulk Import</Button>
 * </PermissionGuard>
 */
export function PermissionGuard({
  children,
  requiredScopes = [],
  requiredAllScopes = [],
  requiredScope,
  showWhenDenied = false,
  fallback = null,
  render
}: PermissionGuardProps) {
  const { hasAnyScope, hasAllScopes } = usePermissions()

  // Determine the required scopes array
  let finalRequiredScopes: string[] = requiredScopes
  if (requiredScope) {
    finalRequiredScopes = [requiredScope]
  }

  // Check permissions
  let hasPermission = true

  if (requiredAllScopes.length > 0) {
    // User must have all scopes in requiredAllScopes
    hasPermission = hasAllScopes(requiredAllScopes)
  } else if (finalRequiredScopes.length > 0) {
    // User must have at least one scope in requiredScopes
    hasPermission = hasAnyScope(finalRequiredScopes)
  }

  // If using render prop, always call it with permission state
  if (render) {
    return <>{render(hasPermission)}</>
  }

  // If has permission, render children
  if (hasPermission) {
    return <>{children}</>
  }

  // If no permission and should show when denied, render children
  if (showWhenDenied) {
    return <>{children}</>
  }

  // If no permission and has fallback, render fallback
  if (fallback !== null) {
    return <>{fallback}</>
  }

  // Default: render nothing when permission denied
  return null
}

/**
 * Higher-order component version of PermissionGuard
 */
export function withPermissionGuard<P extends object>(
  Component: React.ComponentType<P>,
  guardProps: Omit<PermissionGuardProps, 'children'>
) {
  return function PermissionWrappedComponent(props: P) {
    return (
      <PermissionGuard {...guardProps}>
        <Component {...props} />
      </PermissionGuard>
    )
  }
}

/**
 * Hook version for conditional rendering based on permissions
 */
export function usePermissionCheck(
  requiredScopes?: string[],
  requiredAllScopes?: string[],
  requiredScope?: string
) {
  const { hasAnyScope, hasAllScopes } = usePermissions()

  let finalRequiredScopes: string[] = requiredScopes || []
  if (requiredScope) {
    finalRequiredScopes = [requiredScope]
  }

  let hasPermission = true

  if (requiredAllScopes && requiredAllScopes.length > 0) {
    hasPermission = hasAllScopes(requiredAllScopes)
  } else if (finalRequiredScopes.length > 0) {
    hasPermission = hasAnyScope(finalRequiredScopes)
  }

  return { hasPermission }
}