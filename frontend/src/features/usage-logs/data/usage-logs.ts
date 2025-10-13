import { useQuery } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useErrorHandler } from '@/hooks/use-error-handler'
import { useUsageLogPermissions } from '../../../gql/useUsageLogPermissions'
import { useSelectedProjectId } from '@/stores/projectStore'
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
    projectID?: string
    [key: string]: any
  }
}) {
  const { handleError } = useErrorHandler()
  const permissions = useUsageLogPermissions()
  const selectedProjectId = useSelectedProjectId()
  
  // Automatically add projectID filter if a project is selected
  const variablesWithProject = {
    ...variables,
    where: {
      ...variables?.where,
      ...(selectedProjectId && { projectID: selectedProjectId }),
    },
  }
  
  return useQuery({
    queryKey: ['usageLogs', variablesWithProject, permissions],
    queryFn: async () => {
      try {
        const query = buildUsageLogsQuery(permissions)
        const data = await graphqlRequest<{ usageLogs: UsageLogConnection }>(
          query,
          variablesWithProject
        )
        return usageLogConnectionSchema.parse(data?.usageLogs)
      } catch (error) {
        handleError(error, '获取用量日志数据')
        throw error
      }
    },
    enabled: !!selectedProjectId, // Only query when a project is selected
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
        handleError(error, '获取用量日志详情')
        throw error
      }
    },
    enabled: !!id,
  })
}