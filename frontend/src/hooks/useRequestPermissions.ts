import { useMemo } from 'react';
import { useAuthStore } from '@/stores/authStore';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useMe } from '@/features/auth/data/auth';

export interface RequestPermissions {
  canViewUsers: boolean;
  canViewApiKeys: boolean;
  canViewChannels: boolean;
  canViewRoles: boolean;
}

export function useRequestPermissions(): RequestPermissions {
  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const selectedProjectId = useSelectedProjectId();

  // Use data from me query if available, otherwise fall back to auth store
  const user = meData || authUser;
  const systemScopes = user?.scopes || [];
  const isOwner = user?.isOwner || false;

  // Get project-level scopes for the selected project
  const projectScopes = useMemo(() => {
    if (!selectedProjectId || !user?.projects) {
      return [];
    }
    const project = user.projects.find((p) => p.projectID === selectedProjectId);
    return project?.scopes || [];
  }, [selectedProjectId, user?.projects]);

  const permissions = useMemo(() => {
    // 合并系统级和项目级权限
    const userScopes = [...systemScopes, ...projectScopes];

    // Owner用户拥有所有权限
    if (isOwner || userScopes.includes('*')) {
      return {
        canViewUsers: true,
        canViewApiKeys: true,
        canViewChannels: true,
        canViewRoles: true,
      };
    }

    return {
      canViewUsers: userScopes.includes('read_users'),
      canViewApiKeys: userScopes.includes('read_api_keys'),
      canViewChannels: userScopes.includes('read_channels'),
      canViewRoles: userScopes.includes('read_roles'),
    };
  }, [systemScopes, projectScopes, isOwner]);

  return permissions;
}
