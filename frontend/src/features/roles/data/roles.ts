import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import i18n from '@/lib/i18n';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { Role, RoleConnection, CreateRoleInput, UpdateRoleInput, roleConnectionSchema, roleSchema } from './schema';

// GraphQL queries and mutations
const ROLES_QUERY = `
  query GetRoles($first: Int, $after: Cursor, $orderBy: RoleOrder, $where: RoleWhereInput) {
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

const BULK_DELETE_ROLES_MUTATION = `
  mutation BulkDeleteRoles($ids: [ID!]!) {
    bulkDeleteRoles(ids: $ids)
  }
`;

// Query hooks
export function useRoles(
  variables: {
    first?: number;
    after?: string;
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: Record<string, unknown>;
  } = {}
) {
  const { handleError } = useErrorHandler();

  // Always filter for system-level roles only (not project-specific)
  const queryVariables = {
    ...variables,
    where: {
      ...variables.where,
      projectID: 'gid://axonhub/Project/0', // Only system roles (projectID = 0)
    },
  };

  return useQuery({
    queryKey: ['roles', queryVariables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ roles: RoleConnection }>(ROLES_QUERY, queryVariables);
        return roleConnectionSchema.parse(data?.roles);
      } catch (error) {
        handleError(error, '获取角色数据');
        throw error;
      }
    },
  });
}

export function useRole(id: string) {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['role', id],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ roles: RoleConnection }>(ROLES_QUERY, { where: { id } });
        const role = data.roles.edges[0]?.node;
        if (!role) {
          throw new Error('Role not found');
        }
        return roleSchema.parse(role);
      } catch (error) {
        handleError(error, '获取角色详情');
        throw error;
      }
    },
    enabled: !!id,
  });
}

// Mutation hooks
export function useCreateRole() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (input: CreateRoleInput) => {
      try {
        const data = await graphqlRequest<{ createRole: Role }>(CREATE_ROLE_MUTATION, { input });
        return roleSchema.parse(data.createRole);
      } catch (error) {
        handleError(error, '创建角色');
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
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
        handleError(error, '更新角色');
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      queryClient.invalidateQueries({ queryKey: ['role'] });
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
        handleError(error, '删除角色');
        throw error;
      }
    },
    onMutate: async (id: string) => {
      await queryClient.cancelQueries({ queryKey: ['roles'] });

      const previousQueries = queryClient.getQueriesData<RoleConnection>({ queryKey: ['roles'] });

      previousQueries.forEach(([key, data]) => {
        if (!data) return;

        const filteredEdges = data.edges.filter((edge) => edge.node.id !== id);
        if (filteredEdges.length === data.edges.length) return;

        queryClient.setQueryData<RoleConnection>(key, {
          ...data,
          edges: filteredEdges,
          totalCount: Math.max(0, data.totalCount - 1),
        });
      });

      return { previousQueries };
    },
    onError: (_error, _id, context) => {
      context?.previousQueries.forEach(([key, data]) => {
        queryClient.setQueryData(key, data);
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      toast.success(i18n.t('common.success.roleDeleted'));
    },
  });
}

export function useBulkDeleteRoles() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      try {
        await graphqlRequest(BULK_DELETE_ROLES_MUTATION, { ids });
      } catch (error) {
        handleError(error, '批量删除角色');
        throw error;
      }
    },
    onSuccess: (_, ids) => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      toast.success(i18n.t('common.success.rolesDeleted', { count: ids.length }));
    },
  });
}
