import React from 'react';
import { usePermissions } from '@/hooks/usePermissions';

export interface PermissionGuardProps {
  children?: React.ReactNode;
  /** Required scope(s) - user must have at least one of these scopes (any level) */
  requiredScopes?: string[];
  /** All scopes required - user must have all of these scopes (any level) */
  requiredAllScopes?: string[];
  /** Single scope required - shorthand for requiredScopes with one scope (any level) */
  requiredScope?: string;
  /** Required system-level scope(s) - user must have at least one of these scopes at system level */
  requiredSystemScopes?: string[];
  /** All system-level scopes required - user must have all of these scopes at system level */
  requiredAllSystemScopes?: string[];
  /** Single system-level scope required */
  requiredSystemScope?: string;
  /** Required project-level scope(s) - user must have at least one of these scopes at project level */
  requiredProjectScopes?: string[];
  /** All project-level scopes required - user must have all of these scopes at project level */
  requiredAllProjectScopes?: string[];
  /** Single project-level scope required */
  requiredProjectScope?: string;
  /** Whether to render children when permission is denied (default: false) */
  showWhenDenied?: boolean;
  /** Custom component to render when permission is denied */
  fallback?: React.ReactNode;
  /** Render function that receives permission state */
  render?: (hasPermission: boolean) => React.ReactNode;
}

/**
 * PermissionGuard component wraps content and conditionally renders it based on user permissions.
 *
 * @example
 * // Show button only if user has write_channels scope (any level)
 * <PermissionGuard requiredScope="write_channels">
 *   <Button>Bulk Import</Button>
 * </PermissionGuard>
 *
 * @example
 * // Show button only if user has system-level read_users scope
 * <PermissionGuard requiredSystemScope="read_users">
 *   <Button>View All Users</Button>
 * </PermissionGuard>
 *
 * @example
 * // Show button only if user has project-level write_users scope
 * <PermissionGuard requiredProjectScope="write_users">
 *   <Button>Add User to Project</Button>
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
  requiredSystemScopes = [],
  requiredAllSystemScopes = [],
  requiredSystemScope,
  requiredProjectScopes = [],
  requiredAllProjectScopes = [],
  requiredProjectScope,
  showWhenDenied = false,
  fallback = null,
  render,
}: PermissionGuardProps) {
  const { hasAnyScope, hasAllScopes, hasSystemScope, hasProjectScope } = usePermissions();

  // Determine the required scopes arrays
  let finalRequiredScopes: string[] = requiredScopes;
  if (requiredScope) {
    finalRequiredScopes = [requiredScope];
  }

  let finalRequiredSystemScopes: string[] = requiredSystemScopes;
  if (requiredSystemScope) {
    finalRequiredSystemScopes = [requiredSystemScope];
  }

  let finalRequiredProjectScopes: string[] = requiredProjectScopes;
  if (requiredProjectScope) {
    finalRequiredProjectScopes = [requiredProjectScope];
  }

  // Check permissions
  let hasPermission = true;

  // Check system-level scopes
  if (requiredAllSystemScopes.length > 0) {
    hasPermission = hasPermission && requiredAllSystemScopes.every((scope) => hasSystemScope(scope));
  }
  if (finalRequiredSystemScopes.length > 0) {
    hasPermission = hasPermission && finalRequiredSystemScopes.some((scope) => hasSystemScope(scope));
  }

  // Check project-level scopes
  if (requiredAllProjectScopes.length > 0) {
    hasPermission = hasPermission && requiredAllProjectScopes.every((scope) => hasProjectScope(scope));
  }
  if (finalRequiredProjectScopes.length > 0) {
    hasPermission = hasPermission && finalRequiredProjectScopes.some((scope) => hasProjectScope(scope));
  }

  // Check any-level scopes (original behavior)
  if (requiredAllScopes.length > 0) {
    hasPermission = hasPermission && hasAllScopes(requiredAllScopes);
  }
  if (finalRequiredScopes.length > 0) {
    hasPermission = hasPermission && hasAnyScope(finalRequiredScopes);
  }

  // If using render prop, always call it with permission state
  if (render) {
    return <>{render(hasPermission)}</>;
  }

  // If has permission, render children
  if (hasPermission) {
    return <>{children}</>;
  }

  // If no permission and should show when denied, render children
  if (showWhenDenied) {
    return <>{children}</>;
  }

  // If no permission and has fallback, render fallback
  if (fallback !== null) {
    return <>{fallback}</>;
  }

  // Default: render nothing when permission denied
  return null;
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
    );
  };
}

/**
 * Hook version for conditional rendering based on permissions
 */
export function usePermissionCheck(
  requiredScopes?: string[],
  requiredAllScopes?: string[],
  requiredScope?: string,
  requiredSystemScopes?: string[],
  requiredAllSystemScopes?: string[],
  requiredSystemScope?: string,
  requiredProjectScopes?: string[],
  requiredAllProjectScopes?: string[],
  requiredProjectScope?: string
) {
  const { hasAnyScope, hasAllScopes, hasSystemScope, hasProjectScope } = usePermissions();

  let finalRequiredScopes: string[] = requiredScopes || [];
  if (requiredScope) {
    finalRequiredScopes = [requiredScope];
  }

  let finalRequiredSystemScopes: string[] = requiredSystemScopes || [];
  if (requiredSystemScope) {
    finalRequiredSystemScopes = [requiredSystemScope];
  }

  let finalRequiredProjectScopes: string[] = requiredProjectScopes || [];
  if (requiredProjectScope) {
    finalRequiredProjectScopes = [requiredProjectScope];
  }

  let hasPermission = true;

  // Check system-level scopes
  if (requiredAllSystemScopes && requiredAllSystemScopes.length > 0) {
    hasPermission = hasPermission && requiredAllSystemScopes.every((scope) => hasSystemScope(scope));
  }
  if (finalRequiredSystemScopes.length > 0) {
    hasPermission = hasPermission && finalRequiredSystemScopes.some((scope) => hasSystemScope(scope));
  }

  // Check project-level scopes
  if (requiredAllProjectScopes && requiredAllProjectScopes.length > 0) {
    hasPermission = hasPermission && requiredAllProjectScopes.every((scope) => hasProjectScope(scope));
  }
  if (finalRequiredProjectScopes.length > 0) {
    hasPermission = hasPermission && finalRequiredProjectScopes.some((scope) => hasProjectScope(scope));
  }

  // Check any-level scopes
  if (requiredAllScopes && requiredAllScopes.length > 0) {
    hasPermission = hasPermission && hasAllScopes(requiredAllScopes);
  }
  if (finalRequiredScopes.length > 0) {
    hasPermission = hasPermission && hasAnyScope(finalRequiredScopes);
  }

  return { hasPermission };
}
