import { AuthUser } from '@/stores/authStore';

/**
 * 获取用户的所有权限范围（包括直接权限和角色权限）
 */
export function getUserScopes(user: AuthUser | null, projectId?: string | null): string[] {
  if (!user) return [];

  const scopes = new Set<string>();

  // 添加用户直接拥有的全局权限
  user.scopes.forEach((scope) => scopes.add(scope));

  // 如果指定了项目ID，添加项目级别的权限
  if (projectId) {
    const project = user.projects.find((p) => p.projectID === projectId);
    if (project) {
      project.scopes.forEach((scope) => scopes.add(scope));
    }
  }

  return Array.from(scopes);
}

/**
 * 检查用户是否拥有指定的权限
 */
export function hasScope(user: AuthUser | null, scope: string, projectId?: string): boolean {
  if (!user) return false;
  if (user.isOwner) return true; // Owner 拥有所有权限

  const userScopes = getUserScopes(user, projectId);
  return userScopes.includes(scope);
}

/**
 * 检查用户是否是项目 Owner
 */
export function isProjectOwner(user: AuthUser | null, projectId?: string | null): boolean {
  if (!user) return false;
  if (user.isOwner) return true; // 全局 Owner

  if (projectId) {
    const project = user.projects.find((p) => p.projectID === projectId);
    return project?.isOwner || false;
  }

  return false;
}

/**
 * 过滤用户可以授予的权限范围
 * 规则：用户只能授予不高于自己的权限
 */
export function filterGrantableScopes(currentUser: AuthUser | null, allScopes: string[], projectId?: string | null): string[] {
  if (!currentUser) return [];

  // Owner 可以授予所有权限
  if (isProjectOwner(currentUser, projectId)) {
    return allScopes;
  }

  const userScopes = getUserScopes(currentUser, projectId);

  // 用户只能授予自己拥有的权限
  return allScopes.filter((scope) => userScopes.includes(scope));
}

/**
 * 检查用户是否可以授予指定的权限范围
 */
export function canGrantScopes(currentUser: AuthUser | null, scopesToGrant: string[], projectId?: string): boolean {
  if (!currentUser) return false;

  // Owner 可以授予所有权限
  if (isProjectOwner(currentUser, projectId)) {
    return true;
  }

  const userScopes = getUserScopes(currentUser, projectId);

  // 检查是否所有要授予的权限都在用户权限范围内
  return scopesToGrant.every((scope) => userScopes.includes(scope));
}

/**
 * 过滤用户可以授予的角色
 * 规则：用户只能授予权限不高于自己的角色
 */
export function filterGrantableRoles<T extends { scopes?: string[] }>(
  currentUser: AuthUser | null,
  allRoles: T[],
  projectId?: string | null
): T[] {
  if (!currentUser) return [];

  // Owner 可以授予所有角色
  if (isProjectOwner(currentUser, projectId)) {
    return allRoles;
  }

  const userScopes = getUserScopes(currentUser, projectId);

  // 过滤出权限不高于当前用户的角色
  return allRoles.filter((role) => {
    if (!role.scopes || role.scopes.length === 0) return true;

    // 角色的所有权限都必须在用户权限范围内
    return role.scopes.every((scope) => userScopes.includes(scope));
  });
}

/**
 * 检查用户是否可以授予指定的角色
 */
export function canGrantRole(currentUser: AuthUser | null, roleScopes: string[], projectId?: string): boolean {
  if (!currentUser) return false;

  // Owner 可以授予所有角色
  if (isProjectOwner(currentUser, projectId)) {
    return true;
  }

  const userScopes = getUserScopes(currentUser, projectId);

  // 角色的所有权限都必须在用户权限范围内
  return roleScopes.every((scope) => userScopes.includes(scope));
}

/**
 * 验证用户是否可以编辑目标用户/角色
 * 规则：只能编辑权限不高于自己的用户/角色
 */
export function canEditUserPermissions(
  currentUser: AuthUser | null,
  targetUserScopes: string[],
  targetUserIsOwner: boolean,
  projectId?: string | null
): boolean {
  if (!currentUser) return false;

  // 不能编辑 Owner（除非自己是 Owner）
  if (targetUserIsOwner && !isProjectOwner(currentUser, projectId)) {
    return false;
  }

  // Owner 可以编辑所有用户
  if (isProjectOwner(currentUser, projectId)) {
    return true;
  }

  const userScopes = getUserScopes(currentUser, projectId);

  // 目标用户的所有权限都必须在当前用户权限范围内
  return targetUserScopes.every((scope) => userScopes.includes(scope));
}
