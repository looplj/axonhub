import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { USERS_QUERY, CREATE_USER_MUTATION, UPDATE_USER_MUTATION, UPDATE_USER_STATUS_MUTATION } from '@/gql/users'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSelectedProjectId } from '@/stores/projectStore'
import { useErrorHandler } from '@/hooks/use-error-handler'
import {
  User,
  UserConnection,
  ProjectUser,
  CreateUserInput,
  UpdateUserInput,
  userSchema,
  projectUserSchema,
} from './schema'

// GraphQL query for project-level users
// This query fetches users with their project-specific information (owner status and scopes)
export const PROJECT_USERS_QUERY = `
  query ProjectUsers($projectId: ID!) {
    node(id: $projectId) {
      ... on Project {
        id
        name
        projectUsers {
          id
          userID
          projectID
          isOwner
          scopes
          users {
            id
            createdAt
            updatedAt
            email
            status
            firstName
            lastName
            preferLanguage
            roles(where: { projectID: $projectId }) {
              edges {
                node {
                  id
                  name
                  code
                  level
                }
              }
            }
          }
        }
      }
    }
  }
`

// Mutation to add a user to a project
export const ADD_USER_TO_PROJECT_MUTATION = `
  mutation AddUserToProject($input: AddUserToProjectInput!) {
    addUserToProject(input: $input) {
      id
      userID
      projectID
      isOwner
      scopes
    }
  }
`

// Mutation to remove a user from a project
export const REMOVE_USER_FROM_PROJECT_MUTATION = `
  mutation RemoveUserFromProject($input: RemoveUserFromProjectInput!) {
    removeUserFromProject(input: $input)
  }
`

// Mutation to update user's project membership
export const UPDATE_PROJECT_USER_MUTATION = `
  mutation UpdateProjectUser($input: UpdateProjectUserInput!) {
    updateProjectUser(input: $input) {
      id
      userID
      projectID
      isOwner
      scopes
    }
  }
`

// Query to get all users (for adding to project)
export const ALL_USERS_QUERY = `
  query AllUsers($first: Int, $after: Cursor, $where: UserWhereInput) {
    users(first: $first, after: $after, where: $where) {
      edges {
        node {
          id
          email
          firstName
          lastName
          status
        }
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        startCursor
        endCursor
      }
    }
  }
`

// Query hooks - for project-level users
export function useUsers(
  variables?: {
    first?: number
    after?: string
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
    where?: Record<string, any>
  },
  options?: {
    disableAutoFetch?: boolean
  }
) {
  const { t } = useTranslation()
  const { handleError } = useErrorHandler()
  const selectedProjectId = useSelectedProjectId()

  return useQuery({
    queryKey: ['project-users', selectedProjectId, variables],
    queryFn: async () => {
      try {
        if (!selectedProjectId) {
          throw new Error('No project selected')
        }

        const headers = { 'X-Project-ID': selectedProjectId }
        const data = await graphqlRequest<{
          node: {
            id: string
            name: string
            projectUsers: ProjectUser[]
          }
        }>(PROJECT_USERS_QUERY, { projectId: selectedProjectId }, headers)

        // Transform projectUsers to User format for display
        const projectUsers = data.node.projectUsers || []
        const transformedUsers = projectUsers.map((pu) => {
          const parsedPU = projectUserSchema.parse(pu)
          return {
            id: parsedPU.users.id,
            createdAt: parsedPU.users.createdAt,
            updatedAt: parsedPU.users.updatedAt,
            email: parsedPU.users.email,
            status: parsedPU.users.status,
            firstName: parsedPU.users.firstName,
            lastName: parsedPU.users.lastName,
            preferLanguage: parsedPU.users.preferLanguage,
            isOwner: parsedPU.isOwner,
            scopes: parsedPU.scopes,
            roles: parsedPU.users.roles,
            projectUserId: parsedPU.id, // Store the project_user ID for removal
          }
        })

        // Return in UserConnection format for compatibility
        return {
          edges: transformedUsers.map((user) => ({ node: user })),
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: null,
            endCursor: null,
          },
        }
      } catch (error) {
        handleError(error, t('users.messages.loadUsersError'))
        throw error
      }
    },
    enabled: !options?.disableAutoFetch && !!selectedProjectId,
  })
}

export function useUser(id: string) {
  const { t } = useTranslation()
  const { handleError } = useErrorHandler()
  const selectedProjectId = useSelectedProjectId()

  return useQuery({
    queryKey: ['user', id, selectedProjectId],
    queryFn: async () => {
      try {
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
        const data = await graphqlRequest<{ users: UserConnection }>(USERS_QUERY, { where: { id } }, headers)
        const user = data.users.edges[0]?.node
        if (!user) {
          throw new Error(t('users.messages.userNotFound'))
        }
        return userSchema.parse(user)
      } catch (error) {
        handleError(error, t('users.messages.loadUserError'))
        throw error
      }
    },
    enabled: !!id,
  })
}

// Mutation hooks
export function useCreateUser() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const selectedProjectId = useSelectedProjectId()

  return useMutation({
    mutationFn: async (input: CreateUserInput) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
      const data = await graphqlRequest<{ createUser: User }>(CREATE_USER_MUTATION, { input }, headers)
      return userSchema.parse(data.createUser)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success(t('users.messages.createSuccess'))
    },
    onError: (error: any) => {
      toast.error(t('users.messages.createError') + `: ${error.message}`)
    },
  })
}

export function useUpdateUser() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const selectedProjectId = useSelectedProjectId()

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateUserInput }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
      const data = await graphqlRequest<{ updateUser: User }>(UPDATE_USER_MUTATION, { id, input }, headers)
      return userSchema.parse(data.updateUser)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success(t('users.messages.updateSuccess'))
    },
    onError: (error: any) => {
      toast.error(t('users.messages.updateError') + `: ${error.message}`)
    },
  })
}

export function useUpdateUserStatus() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const selectedProjectId = useSelectedProjectId()

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: 'activated' | 'deactivated' }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
      const data = await graphqlRequest<{ updateUserStatus: boolean }>(
        UPDATE_USER_STATUS_MUTATION,
        { id, status },
        headers
      )
      return data.updateUserStatus
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      queryClient.invalidateQueries({ queryKey: ['user', variables.id] })
      const statusText = variables.status === 'activated' ? t('users.status.activated') : t('users.status.deactivated')
      toast.success(t('users.messages.statusUpdateSuccess', { status: statusText }))
    },
    onError: (error: any) => {
      toast.error(t('users.messages.statusUpdateError') + `: ${error.message}`)
    },
  })
}

export function useDeleteUser() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (_id: string) => {
      // This is now deprecated, use useRemoveUserFromProject instead
      throw new Error('Direct deletion is not supported. Use removeUserFromProject instead.')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success(t('users.messages.deleteSuccess'))
    },
    onError: (error: any) => {
      toast.error(t('users.messages.deleteError') + `: ${error.message}`)
    },
  })
}

// Add user to project
export function useAddUserToProject() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const selectedProjectId = useSelectedProjectId()

  return useMutation({
    mutationFn: async (input: { userId: string; isOwner?: boolean; scopes?: string[]; roleIDs?: string[] }) => {
      if (!selectedProjectId) {
        throw new Error('No project selected')
      }
      const headers = { 'X-Project-ID': selectedProjectId }
      const data = await graphqlRequest<{ addUserToProject: any }>(
        ADD_USER_TO_PROJECT_MUTATION,
        {
          input: {
            projectId: selectedProjectId,
            userId: input.userId,
            isOwner: input.isOwner,
            scopes: input.scopes,
            roleIDs: input.roleIDs,
          },
        },
        headers
      )
      return data.addUserToProject
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-users', selectedProjectId] })
      toast.success(t('users.messages.addToProjectSuccess'))
    },
    onError: (error: any) => {
      toast.error(t('users.messages.addToProjectError') + `: ${error.message}`)
    },
  })
}

// Remove user from project
export function useRemoveUserFromProject() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const selectedProjectId = useSelectedProjectId()

  return useMutation({
    mutationFn: async (userId: string) => {
      if (!selectedProjectId) {
        throw new Error('No project selected')
      }
      const headers = { 'X-Project-ID': selectedProjectId }
      const data = await graphqlRequest<{ removeUserFromProject: boolean }>(
        REMOVE_USER_FROM_PROJECT_MUTATION,
        {
          input: {
            projectId: selectedProjectId,
            userId: userId,
          },
        },
        headers
      )
      return data.removeUserFromProject
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-users', selectedProjectId] })
      toast.success(t('users.messages.removeFromProjectSuccess'))
    },
    onError: (error: any) => {
      toast.error(t('users.messages.removeFromProjectError') + `: ${error.message}`)
    },
  })
}

// Update project user (for editing roles/scopes)
export function useUpdateProjectUser() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const selectedProjectId = useSelectedProjectId()

  return useMutation({
    mutationFn: async (input: {
      userId: string
      isOwner?: boolean
      scopes?: string[]
      addRoleIDs?: string[]
      removeRoleIDs?: string[]
    }) => {
      if (!selectedProjectId) {
        throw new Error('No project selected')
      }
      const headers = { 'X-Project-ID': selectedProjectId }
      const data = await graphqlRequest<{ updateProjectUser: any }>(
        UPDATE_PROJECT_USER_MUTATION,
        {
          input: {
            projectId: selectedProjectId,
            userId: input.userId,
            isOwner: input.isOwner,
            scopes: input.scopes,
            addRoleIDs: input.addRoleIDs,
            removeRoleIDs: input.removeRoleIDs,
          },
        },
        headers
      )
      return data.updateProjectUser
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-users', selectedProjectId] })
      toast.success(t('users.messages.updateSuccess'))
    },
    onError: (error: any) => {
      toast.error(t('users.messages.updateError') + `: ${error.message}`)
    },
  })
}

// Get all users (for adding to project)
export function useAllUsers(variables?: { first?: number; after?: string; where?: Record<string, any> }) {
  const { t } = useTranslation()
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['all-users', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ users: UserConnection }>(ALL_USERS_QUERY, variables)
        return data.users
      } catch (error) {
        handleError(error, t('users.messages.loadUsersError'))
        throw error
      }
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
