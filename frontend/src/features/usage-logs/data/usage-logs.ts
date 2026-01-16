import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { useUsageLogPermissions } from '../../../gql/useUsageLogPermissions';
import { UsageLog, UsageLogConnection, usageLogConnectionSchema, usageLogSchema } from './schema';

// Dynamic GraphQL query builder
function buildUsageLogsQuery(permissions: { canViewChannels: boolean }) {
  const channelFields = permissions.canViewChannels
    ? `
          channel {
            id
            name
            type
          }`
    : '';

  return `
    query GetUsageLogs($first: Int, $after: Cursor, $orderBy: UsageLogOrder, $where: UsageLogWhereInput) {
      usageLogs(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt
            requestID${channelFields}
            modelID
            promptTokens
            completionTokens
            totalTokens
            promptAudioTokens
            promptCachedTokens
            promptWriteCachedTokens
            completionAudioTokens
            completionReasoningTokens
            completionAcceptedPredictionTokens
            completionRejectedPredictionTokens
            source
            format
            totalCost
            costItems {
              itemCode
              quantity
              subtotal
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
  `;
}

function buildUsageLogDetailQuery(permissions: { canViewChannels: boolean }) {
  const channelFields = permissions.canViewChannels
    ? `
        channel {
          id
          name
          type
        }`
    : '';

  return `
    query GetUsageLog($id: ID!) {
      node(id: $id) {
        ... on UsageLog {
          id
          createdAt
          updatedAt
          requestID${channelFields}
          modelID
          promptTokens
          completionTokens
          totalTokens
          promptAudioTokens
          promptCachedTokens
          promptWriteCachedTokens
          completionAudioTokens
          completionReasoningTokens
          completionAcceptedPredictionTokens
          completionRejectedPredictionTokens
          source
          format
          totalCost
          costItems {
            itemCode
            quantity
            subtotal
          }
        }
      }
    }
  `;
}

// Query hooks
export function useUsageLogs(variables?: {
  first?: number;
  after?: string;
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
  where?: {
    source?: string;
    modelID?: string;
    channelID?: string;
    projectID?: string;
    [key: string]: any;
  };
}) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const permissions = useUsageLogPermissions();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['usageLogs', variables, permissions, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildUsageLogsQuery(permissions);
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const data = await graphqlRequest<{ usageLogs: UsageLogConnection }>(query, variables, headers);
        return usageLogConnectionSchema.parse(data?.usageLogs);
      } catch (error) {
        handleError(error, t('usageLogs.errors.loadUsageLogsFailed'));
        throw error;
      }
    },
    enabled: !!selectedProjectId, // Only query when a project is selected
  });
}

export function useUsageLog(id: string) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const permissions = useUsageLogPermissions();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['usageLog', id, permissions, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildUsageLogDetailQuery(permissions);
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const data = await graphqlRequest<{ node: UsageLog }>(query, { id }, headers);
        if (!data.node) {
          throw new Error('Usage log not found');
        }
        return usageLogSchema.parse(data.node);
      } catch (error) {
        handleError(error, t('usageLogs.errors.loadUsageLogDetailFailed'));
        throw error;
      }
    },
    enabled: !!id,
  });
}
