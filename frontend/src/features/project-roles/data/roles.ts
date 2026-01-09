import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import i18n from '@/lib/i18n';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { Role, RoleConnection, CreateRoleInput, UpdateRoleInput, roleConnectionSchema, roleSchema } from './schema';

// GraphQL queries and mutations
const PROJECT_ROLES_QUERY = `
  query GetProjectRoles($projectId: ID!, $first: Int, $after: Cursor, $where: RoleWhereInput) {
    node(id: $projectId) {
      ... on Project {
        roles(first: $first, after: $after, where: $where) {
          edges {
            node {
              id
              createdAt
              updatedAt
              name
              scopes
            }
            cursor
          }
          pageInfo {
            hasNextPage
            hasPreviousPage
            startCursor
            endCursor
          }
          totalCount
        }
      }
    }
  }
`;

const CREATE_ROLE_MUTATION = `
  mutation CreateRole($input: CreateRoleInput!) {
    createRole(input: $input) {
      id
      name
      scopes
      createdAt
      updatedAt
    }
  }
`;

const UPDATE_ROLE_MUTATION = `
  mutation UpdateRole($id: ID!, $input: UpdateRoleInput!) {
    updateRole(id: $id, input: $input) {
      id
      name
      scopes
      createdAt
      updatedAt
    }
  }
`;

const DELETE_ROLE_MUTATION = `
  mutation DeleteRole($id: ID!) {
    deleteRole(id: $id)
  }
`;

// Query hooks
export function useRoles(
  variables: {
    first?: number;
    after?: string;
    where?: any;
  } = {}
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['project-roles', variables, selectedProjectId],
    queryFn: async () => {
      try {
        if (!selectedProjectId) {
          throw new Error('Project ID is required');
        }
        const data = await graphqlRequest<{ node: { roles: RoleConnection } }>(PROJECT_ROLES_QUERY, {
          projectId: selectedProjectId,
          ...variables,
        });
        return roleConnectionSchema.parse(data?.node?.roles);
      } catch (error) {
        handleError(error, t('projectRoles.errors.loadProjectRolesFailed'));
        throw error;
      }
    },
    enabled: !!selectedProjectId,
  });
}

export function useRole(id: string) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['project-role', id, selectedProjectId],
    queryFn: async () => {
      try {
        if (!selectedProjectId) {
          throw new Error('Project ID is required');
        }
        const data = await graphqlRequest<{ node: { roles: RoleConnection } }>(PROJECT_ROLES_QUERY, {
          projectId: selectedProjectId,
          where: { id },
        });
        const role = data.node?.roles?.edges[0]?.node;
        if (!role) {
          throw new Error('Role not found');
        }
        return roleSchema.parse(role);
      } catch (error) {
        handleError(error, t('projectRoles.errors.loadProjectRoleDetailFailed'));
        throw error;
      }
    },
    enabled: !!id && !!selectedProjectId,
  });
}

// Mutation hooks
export function useCreateRole() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (input: CreateRoleInput) => {
      try {
        // Ensure projectID is set from the selected project
        const inputWithProjectId = {
          ...input,
          level: 'project',
          projectID: input.projectID || selectedProjectId,
        };
        const data = await graphqlRequest<{ createRole: Role }>(CREATE_ROLE_MUTATION, { input: inputWithProjectId });
        return roleSchema.parse(data.createRole);
      } catch (error) {
        handleError(error, i18n.t('projectRoles.errors.createProjectRoleFailed'));
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-roles'] });
      toast.success(i18n.t('common.success.roleCreated'));
    },
  });
}

export function useUpdateRole() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateRoleInput }) => {
      try {
        const data = await graphqlRequest<{ updateRole: Role }>(UPDATE_ROLE_MUTATION, { id, input });
        return roleSchema.parse(data.updateRole);
      } catch (error) {
        handleError(error, i18n.t('projectRoles.errors.updateProjectRoleFailed'));
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-roles'] });
      queryClient.invalidateQueries({ queryKey: ['project-role'] });
      toast.success(i18n.t('common.success.roleUpdated'));
    },
  });
}

export function useDeleteRole() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      try {
        await graphqlRequest(DELETE_ROLE_MUTATION, { id });
      } catch (error) {
        handleError(error, i18n.t('projectRoles.errors.deleteProjectRoleFailed'));
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-roles'] });
      toast.success(i18n.t('common.success.roleDeleted'));
    },
  });
}
