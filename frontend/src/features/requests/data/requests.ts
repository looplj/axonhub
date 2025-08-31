import { useQuery } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useErrorHandler } from '@/hooks/use-error-handler'
import { useRequestPermissions } from '../../../gql/useRequestPermissions'
import {
  Request,
  RequestConnection,
  RequestExecutionConnection,
  requestConnectionSchema,
  requestExecutionConnectionSchema,
  requestSchema,
} from './schema'

// Dynamic GraphQL query builder
function buildRequestsQuery(permissions: { canViewUsers: boolean; canViewApiKeys: boolean; canViewChannels: boolean }) {
  const userFields = permissions.canViewUsers ? `
          user {
            id
            firstName
            lastName
          }` : ''
  
  const apiKeyFields = permissions.canViewApiKeys ? `
          apiKey {
            id
            name
          }` : ''
  
  const channelFields = permissions.canViewChannels ? `
                channel {
                  id
                  name
                }` : ''

  return `
    query GetRequests(
      $first: Int
      $after: Cursor
      $orderBy: RequestOrder
      $where: RequestWhereInput
    ) {
      requests(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt${userFields}${apiKeyFields}
            source
            modelID
            requestBody
            responseBody
            status
            executions(first: 10, orderBy: { field: CREATED_AT, direction: DESC }) {
              edges {
                node {
                  id
                  createdAt
                  updatedAt${channelFields}
                  modelID
                  requestBody
                  responseBody
                  responseChunks
                  errorMessage
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

function buildRequestDetailQuery(permissions: { canViewUsers: boolean; canViewApiKeys: boolean; canViewChannels: boolean }) {
  const userFields = permissions.canViewUsers ? `
        user {
            id
            firstName
            lastName
          }` : ''
  
  const apiKeyFields = permissions.canViewApiKeys ? `
          apiKey {
            id
            name
        }` : ''
  
  const requestChannelFields = permissions.canViewChannels ? `
          channel {
            id
            name
          }` : ''
  
  const executionChannelFields = permissions.canViewChannels ? `
              channel {
                id
                name
              }` : ''

  return `
    query GetRequestDetail($id: ID!) {
      node(id: $id) {
        ... on Request {
          id
          createdAt
          updatedAt${userFields}${apiKeyFields}${requestChannelFields}
          source
          modelID
          requestBody
          responseBody
          status
          executions(first: 100, orderBy: { field: CREATED_AT, direction: DESC }) {
            edges {
              node {
                id
                createdAt
                updatedAt
                userID
                requestID${executionChannelFields}
                modelID
                requestBody
                responseBody
                responseChunks
                errorMessage
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
      }
    }
  `
}

function buildRequestExecutionsQuery(permissions: { canViewChannels: boolean }) {
  const channelFields = permissions.canViewChannels ? `
              channel {
                  id
                  name
              }` : ''

  return `
    query GetRequestExecutions(
      $requestID: ID!
      $first: Int
      $after: Cursor
      $orderBy: RequestExecutionOrder
      $where: RequestExecutionWhereInput
    ) {
      node(id: $requestID) {
        ... on Request {
          executions(first: $first, after: $after, orderBy: $orderBy, where: $where) {
            edges {
              node {
                id
                createdAt
                updatedAt
                userID
                requestID${channelFields}
                modelID
                requestBody
                responseBody
                responseChunks
                errorMessage
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
      }
    }
  `
}

// Query hooks
export function useRequests(variables?: {
  first?: number
  after?: string
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
  where?: {
    userID?: string
    status?: string
    source?: string
    channelID?: string
    channelIDIn?: string[]
    statusIn?: string[]
    sourceIn?: string[]
    [key: string]: any
  }
}) {
  const { handleError } = useErrorHandler()
  const permissions = useRequestPermissions()
  
  return useQuery({
    queryKey: ['requests', variables, permissions],
    queryFn: async () => {
      try {
        const query = buildRequestsQuery(permissions)
        const data = await graphqlRequest<{ requests: RequestConnection }>(
          query,
          variables
        )
        return requestConnectionSchema.parse(data?.requests)
      } catch (error) {
        handleError(error, '获取请求数据')
        throw error
      }
    },
  })
}

export function useRequest(id: string) {
  const { handleError } = useErrorHandler()
  const permissions = useRequestPermissions()
  
  return useQuery({
    queryKey: ['request', id, permissions],
    queryFn: async () => {
      try {
        const query = buildRequestDetailQuery(permissions)
        const data = await graphqlRequest<{ node: Request }>(
          query,
          { id }
        )
        if (!data.node) {
          throw new Error('Request not found')
        }
        return requestSchema.parse(data.node)
      } catch (error) {
        handleError(error, '获取请求详情')
        throw error
      }
    },
    enabled: !!id,
  })
}

export function useRequestExecutions(requestID: string, variables?: {
  first?: number
  after?: string
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
  where?: Record<string, any>
}) {
  const permissions = useRequestPermissions()
  
  return useQuery({
    queryKey: ['request-executions', requestID, variables, permissions],
    queryFn: async () => {
      const query = buildRequestExecutionsQuery(permissions)
      const data = await graphqlRequest<{ node: { executions: RequestExecutionConnection } }>(
        query,
        { requestID, ...variables }
      )
      return requestExecutionConnectionSchema.parse(data?.node?.executions)
    },
    enabled: !!requestID,
  })
}