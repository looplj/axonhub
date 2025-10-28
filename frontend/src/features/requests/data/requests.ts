import { useQuery } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useErrorHandler } from '@/hooks/use-error-handler'
import { useRequestPermissions } from '../../../hooks/useRequestPermissions'
import { useSelectedProjectId } from '@/stores/projectStore'
import {
  Request,
  RequestConnection,
  RequestExecutionConnection,
  requestConnectionSchema,
  requestExecutionConnectionSchema,
  requestSchema,
} from './schema'

// Dynamic GraphQL query builder
function buildRequestsQuery(permissions: { canViewApiKeys: boolean; canViewChannels: boolean }) {
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
            updatedAt${apiKeyFields}${channelFields}
            source
            modelID
            stream
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

function buildRequestDetailQuery(permissions: { canViewApiKeys: boolean; canViewChannels: boolean }) {
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
          updatedAt${apiKeyFields}${requestChannelFields}
          source
          modelID
          stream
          projectID
          dataStorage {
            id
          }
          requestBody
          responseBody
          status
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
                requestID${channelFields}
                modelID
                projectID
                dataStorage {
                  id
                }
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
    status?: string
    source?: string
    channelID?: string
    channelIDIn?: string[]
    statusIn?: string[]
    sourceIn?: string[]
    projectID?: string
    [key: string]: any
  }
}) {
  const { handleError } = useErrorHandler()
  const permissions = useRequestPermissions()
  const selectedProjectId = useSelectedProjectId()
  
  return useQuery({
    queryKey: ['requests', variables, permissions, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildRequestsQuery(permissions)
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
        const data = await graphqlRequest<{ requests: RequestConnection }>(
          query,
          variables,
          headers
        )
        return requestConnectionSchema.parse(data?.requests)
      } catch (error) {
        handleError(error, '获取请求数据')
        throw error
      }
    },
    enabled: !!selectedProjectId, // Only query when a project is selected
  })
}

export function useRequest(id: string) {
  const { handleError } = useErrorHandler()
  const permissions = useRequestPermissions()
  const selectedProjectId = useSelectedProjectId()
  
  return useQuery({
    queryKey: ['request', id, permissions, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildRequestDetailQuery(permissions)
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
        const data = await graphqlRequest<{ node: Request }>(
          query,
          { id },
          headers
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
  const selectedProjectId = useSelectedProjectId()
  
  return useQuery({
    queryKey: ['request-executions', requestID, variables, permissions, selectedProjectId],
    queryFn: async () => {
      const query = buildRequestExecutionsQuery(permissions)
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
      const data = await graphqlRequest<{ node: { executions: RequestExecutionConnection } }>(
        query,
        { requestID, ...variables },
        headers
      )
      return requestExecutionConnectionSchema.parse(data?.node?.executions)
    },
    enabled: !!requestID,
  })
}