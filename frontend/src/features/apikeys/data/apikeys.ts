import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { graphqlRequest } from '@/gql/graphql'
import { useErrorHandler } from '@/hooks/use-error-handler'
import { useRequestPermissions } from '../../../hooks/useRequestPermissions'
import type { ApiKey, ApiKeyConnection, CreateApiKeyInput, UpdateApiKeyInput } from './schema'
import { apiKeyConnectionSchema, apiKeySchema } from './schema'
import { toast } from 'sonner'

// Dynamic GraphQL query builders
function buildApiKeysQuery(permissions: { canViewUsers: boolean }) {
  const userFields = permissions.canViewUsers ? `
          user {
            id
            firstName
            lastName
          }` : ''

  return `
    query GetApiKeys($first: Int, $after: Cursor, $orderBy: APIKeyOrder, $where: APIKeyWhereInput) {
      apiKeys(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt${userFields}
            key
            name
            status
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
}

function buildApiKeyQuery(permissions: { canViewUsers: boolean }) {
  const userFields = permissions.canViewUsers ? `
      user {
        id
        firstName
        lastName
      }` : ''

  return `
    query GetApiKey($id: ID!) {
      apiKey(id: $id) {
        id
        createdAt
        updatedAt${userFields}
        key
        name
        status
      }
    }
  `
}

function buildCreateApiKeyMutation(permissions: { canViewUsers: boolean }) {
  const userFields = permissions.canViewUsers ? `
      user {
        id
        firstName
        lastName
      }` : ''

  return `
    mutation CreateAPIKey($input: CreateAPIKeyInput!) {
      createAPIKey(input: $input) {
        id
        createdAt
        updatedAt${userFields}
        key
        name
        status
      }
    }
  `
}

function buildUpdateApiKeyMutation(permissions: { canViewUsers: boolean }) {
  const userFields = permissions.canViewUsers ? `
      user {
        id
        firstName
        lastName
      }` : ''

  return `
    mutation UpdateAPIKey($id: ID!, $input: UpdateAPIKeyInput!) {
      updateAPIKey(id: $id, input: $input) {
        id
        createdAt
        updatedAt${userFields}
        key
        name
        status
      }
    }
  `
}

const UPDATE_APIKEY_STATUS_MUTATION = `
  mutation UpdateAPIKeyStatus($id: ID!, $status: APIKeyStatus!) {
    updateAPIKeyStatus(id: $id, status: $status) {
      id
      status
    }
  }
`

// React Query hooks
export function useApiKeys(variables?: {
  first?: number
  after?: string
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
  where?: {
    nameContainsFold?: string
    status?: string
    userID?: string
    [key: string]: any
  }
}) {
  const { t } = useTranslation()
  const { handleError } = useErrorHandler()
  const permissions = useRequestPermissions()
  
  return useQuery({
    queryKey: ['apiKeys', variables, permissions],
    queryFn: async () => {
      try {
        const query = buildApiKeysQuery(permissions)
        const data = await graphqlRequest<{ apiKeys: ApiKeyConnection }>(
          query,
          variables
        )
        return apiKeyConnectionSchema.parse(data?.apiKeys)
      } catch (error) {
        handleError(error, t('apikeys.errors.fetchData'))
        throw error
      }
    },
  })
}

export function useApiKey(id: string) {
  const { t } = useTranslation()
  const { handleError } = useErrorHandler()
  const permissions = useRequestPermissions()
  
  return useQuery({
    queryKey: ['apiKey', id, permissions],
    queryFn: async () => {
      try {
        const query = buildApiKeyQuery(permissions)
        const data = await graphqlRequest<{ apiKey: ApiKey }>(query, { id })
        return apiKeySchema.parse(data.apiKey)
      } catch (error) {
        handleError(error, t('apikeys.errors.fetchDetails'))
        throw error
      }
    },
    enabled: !!id,
  })
}

export function useCreateApiKey() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const permissions = useRequestPermissions()
  
  return useMutation({
    mutationFn: (input: CreateApiKeyInput) => {
      const mutation = buildCreateApiKeyMutation(permissions)
      return graphqlRequest<{ createAPIKey: ApiKey }>(mutation, { input })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
      toast.success(t('apikeys.messages.createSuccess'))
    },
    onError: (error) => {
      toast.error(t('apikeys.messages.createError'))
      console.error('Create API Key error:', error)
    },
  })
}

export function useUpdateApiKey() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const permissions = useRequestPermissions()
  
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateApiKeyInput }) => {
      const mutation = buildUpdateApiKeyMutation(permissions)
      return graphqlRequest<{ updateAPIKey: ApiKey }>(mutation, { id, input })
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] })
      toast.success(t('apikeys.messages.updateSuccess'))
    },
    onError: (error) => {
      toast.error(t('apikeys.messages.updateError'))
      console.error('Update API Key error:', error)
    },
  })
}

export function useUpdateApiKeyStatus() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: 'enabled' | 'disabled' }) => 
      graphqlRequest<{ updateAPIKeyStatus: ApiKey }>(UPDATE_APIKEY_STATUS_MUTATION, { id, status }),
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] })
      const statusText = data.updateAPIKeyStatus.status === 'enabled' 
        ? t('apikeys.status.enabled') 
        : t('apikeys.status.disabled')
      toast.success(t('apikeys.messages.statusUpdateSuccess', { status: statusText }))
    },
    onError: (error) => {
      toast.error(t('apikeys.messages.statusUpdateError'))
      console.error('Update API Key status error:', error)
    },
  })
}