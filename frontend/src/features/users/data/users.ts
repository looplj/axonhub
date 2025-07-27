import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'
import {
  User,
  UserConnection,
  CreateUserInput,
  UpdateUserInput,
  userConnectionSchema,
  userSchema,
} from './schema'

// GraphQL queries and mutations
const USERS_QUERY = `
  query GetUsers(
    $first: Int
    $after: Cursor
    $orderBy: UserOrder
    $where: UserWhereInput
  ) {
    users(first: $first, after: $after, orderBy: $orderBy, where: $where) {
      edges {
        node {
          id
          createdAt
          updatedAt
          email
          name
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

const CREATE_USER_MUTATION = `
  mutation CreateUser($input: CreateUserInput!) {
    createUser(input: $input) {
      id
      createdAt
      updatedAt
      email
      name
    }
  }
`

const UPDATE_USER_MUTATION = `
  mutation UpdateUser($id: ID!, $input: UpdateUserInput!) {
    updateUser(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      email
      name
    }
  }
`

const DELETE_USER_MUTATION = `
  mutation DeleteUser($id: ID!) {
    deleteUser(id: $id)
  }
`

// Query hooks
export function useUsers(variables?: {
  first?: number
  after?: string
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
  where?: Record<string, any>
}) {
  const { handleError } = useErrorHandler()
  
  return useQuery({
    queryKey: ['users', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ users: UserConnection }>(
          USERS_QUERY,
          variables
        )
        return userConnectionSchema.parse(data?.users)
      } catch (error) {
        handleError(error, '获取用户数据')
        throw error
      }
    },
  })
}

export function useUser(id: string) {
  const { handleError } = useErrorHandler()
  
  return useQuery({
    queryKey: ['user', id],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ users: UserConnection }>(
          USERS_QUERY,
          { where: { id } }
        )
        const user = data.users.edges[0]?.node
        if (!user) {
          throw new Error('User not found')
        }
        return userSchema.parse(user)
      } catch (error) {
        handleError(error, '获取用户详情')
        throw error
      }
    },
    enabled: !!id,
  })
}

// Mutation hooks
export function useCreateUser() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: CreateUserInput) => {
      const data = await graphqlRequest<{ createUser: User }>(
        CREATE_USER_MUTATION,
        { input }
      )
      return userSchema.parse(data.createUser)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('用户创建成功')
    },
    onError: (error: any) => {
      toast.error(`创建用户失败: ${error.message}`)
    },
  })
}

export function useUpdateUser() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateUserInput }) => {
      const data = await graphqlRequest<{ updateUser: User }>(
        UPDATE_USER_MUTATION,
        { id, input }
      )
      return userSchema.parse(data.updateUser)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('用户更新成功')
    },
    onError: (error: any) => {
      toast.error(`更新用户失败: ${error.message}`)
    },
  })
}

export function useDeleteUser() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      await graphqlRequest(DELETE_USER_MUTATION, { id })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('用户删除成功')
    },
    onError: (error: any) => {
      toast.error(`删除用户失败: ${error.message}`)
    },
  })
}

// Export users for compatibility
export const users = {
  useUsers,
  useUser,
  useCreateUser,
  useUpdateUser,
  useDeleteUser,
}
