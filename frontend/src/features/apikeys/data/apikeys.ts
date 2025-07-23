import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
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
            name
          }
          key
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

const APIKEY_QUERY = `
  query GetApiKey($id: ID!) {
    apiKey(id: $id) {
      id
      createdAt
      updatedAt
      user {
        id
        name
      }
      key
      name
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
        name
      }
      key
      name
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
        name
      }
      key
      name
    }
  }
`

const DELETE_APIKEY_MUTATION = `
  mutation DeleteApiKey($id: ID!) {
    deleteApiKey(id: $id) {
      id
    }
  }
`

// React Query hooks
export function useApiKeys(first = 10, after?: string) {
  return useQuery({
    queryKey: ['apiKeys', first, after],
    queryFn: () => graphqlRequest<{ apiKeys: ApiKeyConnection }>(APIKEYS_QUERY, { first, after }),
    select: (data) => data.apiKeys,
  })
}

export function useApiKey(id: string) {
  return useQuery({
    queryKey: ['apiKey', id],
    queryFn: () => graphqlRequest<{ apiKey: ApiKey }>(APIKEY_QUERY, { id }),
    select: (data) => data.apiKey,
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

export function useDeleteApiKey() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: (id: string) => 
      graphqlRequest<{ deleteApiKey: { id: string } }>(DELETE_APIKEY_MUTATION, { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
      toast.success('API Key deleted successfully')
    },
    onError: (error) => {
      toast.error('Failed to delete API Key')
      console.error('Delete API Key error:', error)
    },
  })
}