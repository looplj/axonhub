import { useQuery } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useErrorHandler } from '@/hooks/use-error-handler'
import {
  Request,
  RequestConnection,
  RequestExecution,
  RequestExecutionConnection,
  requestConnectionSchema,
  requestExecutionConnectionSchema,
  requestSchema,
} from './schema'

// GraphQL queries
const REQUESTS_QUERY = `
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
          updatedAt
          user {
            id
            firstName
            lastName
          }
          apiKey {
            id
            name
          }
          requestBody
          responseBody
          status
          executions(first: 10, orderBy: { field: CREATED_AT, direction: DESC }) {
            edges {
              node {
                id
                createdAt
                updatedAt
                channel {
                  id
                  name
                }
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

const REQUEST_DETAIL_QUERY = `
  query GetRequestDetail($id: ID!) {
    node(id: $id) {
      ... on Request {
        id
        createdAt
        updatedAt
        user {
            id
            firstName
            lastName
          }
          apiKey {
            id
            name
          }
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
              requestID
              channel {
                id
                name
              }
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

const REQUEST_EXECUTIONS_QUERY = `
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
              requestID
              channel {
                  id
                  name
              }
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

// Query hooks
export function useRequests(variables?: {
  first?: number
  after?: string
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
  where?: Record<string, any>
}) {
  const { handleError } = useErrorHandler()
  
  return useQuery({
    queryKey: ['requests', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ requests: RequestConnection }>(
          REQUESTS_QUERY,
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
  
  return useQuery({
    queryKey: ['request', id],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ node: Request }>(
          REQUEST_DETAIL_QUERY,
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
  return useQuery({
    queryKey: ['request-executions', requestID, variables],
    queryFn: async () => {
      const data = await graphqlRequest<{ node: { executions: RequestExecutionConnection } }>(
        REQUEST_EXECUTIONS_QUERY,
        { requestID, ...variables }
      )
      return requestExecutionConnectionSchema.parse(data?.node?.executions)
    },
    enabled: !!requestID,
  })
}