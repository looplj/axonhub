import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useErrorHandler } from '@/hooks/use-error-handler'
import type { ApiKey, ApiKeyConnection, CreateApiKeyInput, UpdateApiKeyInput } from './schema'
import { apiKeyConnectionSchema, apiKeySchema } from './schema'
import { toast } from 'sonner'

// GraphQL queries and mutations
const APIKEYS_QUERY = `
  query GetApiKeys($first: Int, $after: Cursor, $orderBy: APIKeyOrder) {
    apiKeys(first: $first, after: $after, orderBy: $orderBy) {
      edges {
        node {
          id
          createdAt
          updatedAt
          user {
            id
            firstName
            lastName
          }
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

const APIKEY_QUERY = `
  query GetApiKey($id: ID!) {
    apiKey(id: $id) {
      id
      createdAt
      updatedAt
      user {
        id
        firstName
        lastName
      }
      key
      name
      status
    }
  }
`

const CREATE_APIKEY_MUTATION = `
  mutation CreateApiKey($input: CreateApiKeyInput!) {
    createApiKey(input: $input) {
      id
      createdAt
      updatedAt
      user {
        id
        firstName
        lastName
      }
      key
      name
      status
    }
  }
`

const UPDATE_APIKEY_MUTATION = `
  mutation UpdateApiKey($id: ID!, $input: UpdateApiKeyInput!) {
    updateApiKey(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      user {
        id
        firstName
        lastName
      }
      key
      name
      status
    }
  }
`

const UPDATE_APIKEY_STATUS_MUTATION = `
  mutation UpdateApiKeyStatus($id: ID!, $status: APIKeyStatus!) {
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
  where?: Record<string, any>
}) {
  const { handleError } = useErrorHandler()
  
  return useQuery({
    queryKey: ['apiKeys', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ apiKeys: ApiKeyConnection }>(
          APIKEYS_QUERY,
          variables
        )
        return apiKeyConnectionSchema.parse(data?.apiKeys)
      } catch (error) {
        handleError(error, '获取API密钥数据')
        throw error
      }
    },
  })
}

export function useApiKey(id: string) {
  const { handleError } = useErrorHandler()
  
  return useQuery({
    queryKey: ['apiKey', id],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ apiKey: ApiKey }>(APIKEY_QUERY, { id })
        return apiKeySchema.parse(data.apiKey)
      } catch (error) {
        handleError(error, '获取API密钥详情')
        throw error
      }
    },
    enabled: !!id,
  })
}

export function useCreateApiKey() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: (input: CreateApiKeyInput) => 
      graphqlRequest<{ createApiKey: ApiKey }>(CREATE_APIKEY_MUTATION, { input }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
      toast.success('API Key created successfully')
    },
    onError: (error) => {
      toast.error('Failed to create API Key')
      console.error('Create API Key error:', error)
    },
  })
}

export function useUpdateApiKey() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateApiKeyInput }) => 
      graphqlRequest<{ updateApiKey: ApiKey }>(UPDATE_APIKEY_MUTATION, { id, input }),
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] })
      toast.success('API Key updated successfully')
    },
    onError: (error) => {
      toast.error('Failed to update API Key')
      console.error('Update API Key error:', error)
    },
  })
}

export function useUpdateApiKeyStatus() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: 'enabled' | 'disabled' }) => 
      graphqlRequest<{ updateAPIKeyStatus: ApiKey }>(UPDATE_APIKEY_STATUS_MUTATION, { id, status }),
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] })
      toast.success(`API Key 状态已更新为 ${data.updateAPIKeyStatus.status === 'enabled' ? '启用' : '禁用'}`)
    },
    onError: (error) => {
      toast.error('状态更新失败')
      console.error('Update API Key status error:', error)
    },
  })
}