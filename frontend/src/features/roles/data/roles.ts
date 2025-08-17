import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'
import i18n from '@/lib/i18n'
import {
  Role,
  RoleConnection,
  ScopeInfo,
  CreateRoleInput,
  UpdateRoleInput,
  roleConnectionSchema,
  roleSchema,
  scopeInfoSchema,
} from './schema'

// GraphQL queries and mutations
const ROLES_QUERY = `
  query GetRoles($first: Int, $after: Cursor, $where: RoleWhereInput) {
    roles(first: $first, after: $after, where: $where) {
      edges {
        node {
          id
          createdAt
          updatedAt
          code
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
`

const ALL_SCOPES_QUERY = `
  query GetAllScopes {
    allScopes {
      scope
      description
    }
  }
`

const CREATE_ROLE_MUTATION = `
  mutation CreateRole($input: CreateRoleInput!) {
    createRole(input: $input) {
      id
      code
      name
      scopes
      createdAt
      updatedAt
    }
  }
`

const UPDATE_ROLE_MUTATION = `
  mutation UpdateRole($id: ID!, $input: UpdateRoleInput!) {
    updateRole(id: $id, input: $input) {
      id
      code
      name
      scopes
      createdAt
      updatedAt
    }
  }
`

const DELETE_ROLE_MUTATION = `
  mutation DeleteRole($id: ID!) {
    deleteRole(id: $id)
  }
`

// Query hooks
export function useRoles(variables: {
  first?: number
  after?: string
  where?: any
} = {}) {
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['roles', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ roles: RoleConnection }>(
          ROLES_QUERY,
          variables
        )
        return roleConnectionSchema.parse(data?.roles)
      } catch (error) {
        handleError(error, '获取角色数据')
        throw error
      }
    }
  })
}

export function useRole(id: string) {
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['role', id],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ roles: RoleConnection }>(
          ROLES_QUERY,
          { where: { id } }
        )
        const role = data.roles.edges[0]?.node
        if (!role) {
          throw new Error('Role not found')
        }
        return roleSchema.parse(role)
      } catch (error) {
        handleError(error, '获取角色详情')
        throw error
      }
    },
    enabled: !!id,
  })
}

export function useAllScopes() {
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['allScopes'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ allScopes: ScopeInfo[] }>(
          ALL_SCOPES_QUERY
        )
        return data.allScopes.map(scope => scopeInfoSchema.parse(scope))
      } catch (error) {
        handleError(error, '获取权限列表')
        throw error
      }
    }
  })
}

// Mutation hooks
export function useCreateRole() {
  const queryClient = useQueryClient()
  const { handleError } = useErrorHandler()

  return useMutation({
    mutationFn: async (input: CreateRoleInput) => {
      try {
        const data = await graphqlRequest<{ createRole: Role }>(
          CREATE_ROLE_MUTATION,
          { input }
        )
        return roleSchema.parse(data.createRole)
      } catch (error) {
        handleError(error, '创建角色')
        throw error
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
      toast.success(i18n.t('common.success.roleCreated'))
    },
  })
}

export function useUpdateRole() {
  const queryClient = useQueryClient()
  const { handleError } = useErrorHandler()

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateRoleInput }) => {
      try {
        const data = await graphqlRequest<{ updateRole: Role }>(
          UPDATE_ROLE_MUTATION,
          { id, input }
        )
        return roleSchema.parse(data.updateRole)
      } catch (error) {
        handleError(error, '更新角色')
        throw error
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
      queryClient.invalidateQueries({ queryKey: ['role'] })
      toast.success(i18n.t('common.success.roleUpdated'))
    },
  })
}

export function useDeleteRole() {
  const queryClient = useQueryClient()
  const { handleError } = useErrorHandler()

  return useMutation({
    mutationFn: async (id: string) => {
      try {
        await graphqlRequest(DELETE_ROLE_MUTATION, { id })
      } catch (error) {
        handleError(error, '删除角色')
        throw error
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
      toast.success(i18n.t('common.success.roleDeleted'))
    },
  })
}