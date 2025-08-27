import { useQuery } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useErrorHandler } from '@/hooks/use-error-handler'
import { useRequestPermissions } from '../../../gql/useRequestPermissions'
import { useUsageLogPermissions } from '../../../gql/useUsageLogPermissions'
import {
  UsageLog,
  UsageLogConnection,
  usageLogConnectionSchema,
  usageLogSchema,
} from './schema'

// Dynamic GraphQL query builder
function buildUsageLogsQuery(permissions: { canViewUsers: boolean; canViewChannels: boolean }) {
  const userFields = permissions.canViewUsers ? `
          user {
            id
            firstName
            lastName
            email
          }` : ''
  
  const channelFields = permissions.canViewChannels ? `
          channel {
            id
            name
            type
          }` : ''

  return `
    query GetUsageLogs($first: Int, $after: Cursor, $orderBy: UsageLogOrder, $where: UsageLogWhereInput) {
      usageLogs(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt${userFields}
            requestID${channelFields}
            modelID
            promptTokens
            completionTokens
            totalTokens
            promptAudioTokens
            promptCachedTokens
            completionAudioTokens
            completionReasoningTokens
            completionAcceptedPredictionTokens
            completionRejectedPredictionTokens
            source
            format
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

function buildUsageLogDetailQuery(permissions: { canViewUsers: boolean; canViewChannels: boolean }) {
  const userFields = permissions.canViewUsers ? `
        user {
          id
          firstName
          lastName
          email
        }` : ''
  
  const channelFields = permissions.canViewChannels ? `
        channel {
          id
          name
          type
        }` : ''

  return `
    query GetUsageLog($id: ID!) {
      node(id: $id) {
        ... on UsageLog {
          id
          createdAt
          updatedAt${userFields}
          requestID${channelFields}
          modelID
          promptTokens
          completionTokens
          totalTokens
          promptAudioTokens
          promptCachedTokens
          completionAudioTokens
          completionReasoningTokens
          completionAcceptedPredictionTokens
          completionRejectedPredictionTokens
          source
          format
        }
      }
    }
  `
}

// Query hooks
export function useUsageLogs(variables?: {
  first?: number
  after?: string
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
  where?: {
    userID?: string
    source?: string
    modelID?: string
    channelID?: string
    [key: string]: any
  }
}) {
  const { handleError } = useErrorHandler()
  const permissions = useUsageLogPermissions()
  
  return useQuery({
    queryKey: ['usageLogs', variables, permissions],
    queryFn: async () => {
      try {
        const query = buildUsageLogsQuery(permissions)
        const data = await graphqlRequest<{ usageLogs: UsageLogConnection }>(
          query,
          variables
        )
        return usageLogConnectionSchema.parse(data?.usageLogs)
      } catch (error) {
        handleError(error, '获取使用日志数据')
        throw error
      }
    },
  })
}

export function useUsageLog(id: string) {
  const { handleError } = useErrorHandler()
  const permissions = useUsageLogPermissions()
  
  return useQuery({
    queryKey: ['usageLog', id, permissions],
    queryFn: async () => {
      try {
        const query = buildUsageLogDetailQuery(permissions)
        const data = await graphqlRequest<{ node: UsageLog }>(
          query,
          { id }
        )
        if (!data.node) {
          throw new Error('Usage log not found')
        }
        return usageLogSchema.parse(data.node)
      } catch (error) {
        handleError(error, '获取使用日志详情')
        throw error
      }
    },
    enabled: !!id,
  })
}