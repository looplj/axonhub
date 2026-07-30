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
  query GetProjectRoles($first: Int, $after: Cursor, $orderBy: RoleOrder, $where: RoleWhereInput) {
    roles(first: $first, after: $after, orderBy: $orderBy, where: $where) {
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
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: any;
  } = {}
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();

  const queryVariables = {
    first: 20,
    ...variables,
    orderBy: variables.orderBy || { field: 'CREATED_AT', direction: 'DESC' },
  };

  return useQuery({
    queryKey: ['project-roles', queryVariables, selectedProjectId],
    queryFn: async () => {
      try {
        if (!selectedProjectId) {
          throw new Error('Project ID is required');
        }
        const data = await graphqlRequest<{ roles: RoleConnection }>(
          PROJECT_ROLES_QUERY,
          {
            projectId: selectedProjectId,
            ...queryVariables,
            where: { and: [{ projectID: selectedProjectId }, ...(variables.where ? [variables.where] : [])] },
          },
          { 'X-Project-ID': selectedProjectId }
        );
        return roleConnectionSchema.parse(data.roles);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
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
        const data = await graphqlRequest<{ roles: RoleConnection }>(
          PROJECT_ROLES_QUERY,
          { projectId: selectedProjectId, where: { and: [{ projectID: selectedProjectId }, { id }] } },
          { 'X-Project-ID': selectedProjectId }
        );
        const role = data.roles?.edges[0]?.node;
        if (!role) {
          throw new Error('Role not found');
        }
        return roleSchema.parse(role);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
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
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (input: CreateRoleInput) => {
      try {
        // Ensure projectID is set from the selected project
        const inputWithProjectId = {
          ...input,
          level: 'project',
          projectID: input.projectID || selectedProjectId,
        };
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const data = await graphqlRequest<{ createRole: Role }>(CREATE_ROLE_MUTATION, { input: inputWithProjectId }, headers);
        return roleSchema.parse(data.createRole);
      } catch (error) {
        handleError(error, { context: t('roles.dialogs.create.title') });
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
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateRoleInput }) => {
      try {
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const data = await graphqlRequest<{ updateRole: Role }>(UPDATE_ROLE_MUTATION, { id, input }, headers);
        return roleSchema.parse(data.updateRole);
      } catch (error) {
        handleError(error, { context: t('roles.dialogs.edit.title') });
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
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (id: string) => {
      try {
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        await graphqlRequest(DELETE_ROLE_MUTATION, { id }, headers);
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-roles'] });
      toast.success(i18n.t('common.success.roleDeleted'));
    },
  });
}
