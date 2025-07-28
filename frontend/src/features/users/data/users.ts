import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'
import {
  USERS_QUERY,
  USER_QUERY,
  CREATE_USER_MUTATION,
  UPDATE_USER_MUTATION,
  DELETE_USER_MUTATION
} from '@/gql/users'
import {
  User,
  UserConnection,
  CreateUserInput,
  UpdateUserInput,
  userConnectionSchema,
  userSchema,
} from './schema'

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
