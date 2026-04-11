import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { useRequestPermissions } from '../../../hooks/useRequestPermissions';
import {
  Request,
  RequestConnection,
  RequestExecutionConnection,
  requestConnectionSchema,
  requestExecutionConnectionSchema,
  requestSchema,
} from './schema';

// Dynamic GraphQL query builder
function buildRequestsQuery(permissions: { canViewApiKeys: boolean; canViewChannels: boolean }) {
  const apiKeyFields = permissions.canViewApiKeys
    ? `
          apiKey {
            id
            name
          }`
    : '';

  const channelFields = permissions.canViewChannels
    ? `
                channel {
                  id
                  name
                }`
    : '';

  return `
    query GetRequests(
      $first: Int
      $after: Cursor
      $last: Int
      $before: Cursor
      $orderBy: RequestOrder
      $where: RequestWhereInput
    ) {
      requests(first: $first, after: $after, last: $last, before: $before, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt${apiKeyFields}${channelFields}
            source
            modelID
            stream
            status
            clientIP
            metricsLatencyMs
            metricsFirstTokenLatencyMs
            executions(first: 10, orderBy: { field: CREATED_AT, direction: DESC }) {
              edges {
                node {
                  modelID
                  status
                  channel {
                    id
                    name
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
            usageLogs(first: 1) {
              edges {
                node {
                  id
                  promptTokens
                  completionTokens
                  totalTokens
                  promptCachedTokens
                  promptWriteCachedTokens
                  totalCost
                }
              }
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

function buildRequestDetailQuery(permissions: { canViewApiKeys: boolean; canViewChannels: boolean }) {
  const apiKeyFields = permissions.canViewApiKeys
    ? `
          apiKey {
            id
            name
        }`
    : '';

  const requestChannelFields = permissions.canViewChannels
    ? `
          channel {
            id
            name
          }`
    : '';

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
          clientIP
          projectID
          dataStorageID
          contentSaved
          contentStorageKey
          requestHeaders
          requestBody
          responseBody
          responseChunks
          status
          format
          usageLogs(first: 1) {
            edges {
              node {
                  id
                  promptTokens
                  completionTokens
                  totalTokens
                  promptCachedTokens
                  promptWriteCachedTokens
                  totalCost
                }
            }
          }
        }
      }
    }
  `;
}

function buildRequestExecutionsQuery(permissions: { canViewChannels: boolean }) {
  const channelFields = permissions.canViewChannels
    ? `
              channel {
                  id
                  name
                  type
                  baseURL
              }`
    : '';

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
                dataStorageID
                requestHeaders
                requestBody
                responseBody
                responseChunks
                errorMessage
                responseStatusCode
                status
                format
                stream
                metricsFirstTokenLatencyMs
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
  `;
}

// Query hooks
export function useRequests(variables?: {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
  where?: {
    status?: string;
    source?: string;
    channelID?: string;
    channelIDIn?: string[];
    statusIn?: string[];
    sourceIn?: string[];
    projectID?: string;
    [key: string]: any;
  };
}, options?: { projectId?: string | null; scopeToSelectedProject?: boolean; enabled?: boolean }) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const permissions = useRequestPermissions();
  const selectedProjectId = useSelectedProjectId();
  const scopeToSelectedProject = options?.scopeToSelectedProject ?? true;
  const projectId = options?.projectId !== undefined ? options.projectId : selectedProjectId;
  const enabled = options?.enabled ?? true;

  return useQuery({
    queryKey: ['requests', variables, permissions, projectId, scopeToSelectedProject],
    queryFn: async () => {
      try {
        const query = buildRequestsQuery(permissions);
        const headers = projectId ? { 'X-Project-ID': projectId } : undefined;

        // Add project filter if project scoping is enabled
        const finalVariables = {
          ...variables,
          where: {
            ...variables?.where,
            ...(scopeToSelectedProject && projectId && { projectID: projectId }),
          },
        };

        const data = await graphqlRequest<{ requests: RequestConnection }>(query, finalVariables, headers);
        return requestConnectionSchema.parse(data?.requests);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled,
  });
}

export function useRequest(id: string, options?: { projectId?: string | null; enabled?: boolean }) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const permissions = useRequestPermissions();
  const selectedProjectId = useSelectedProjectId();
  const projectId = options?.projectId !== undefined ? options.projectId : selectedProjectId;
  const enabled = options?.enabled ?? true;

  return useQuery({
    queryKey: ['request', id, permissions, projectId],
    queryFn: async () => {
      try {
        const query = buildRequestDetailQuery(permissions);
        const headers = projectId ? { 'X-Project-ID': projectId } : undefined;
        const data = await graphqlRequest<{ node: Request }>(query, { id }, headers);
        if (!data.node) {
          throw new Error('Request not found');
        }
        return requestSchema.parse(data.node);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: enabled && !!id,
  });
}

/**
 * Imperative (non-hook) fetch of a page of requests for drawer navigation.
 * direction 'older' fetches the page after endCursor (older in DESC order).
 * direction 'newer' fetches the page before startCursor (newer in DESC order).
 */
export async function fetchAdjacentRequestPage(params: {
  cursor: string;
  direction: 'older' | 'newer';
  pageSize: number;
  where?: Record<string, any>;
  permissions: { canViewApiKeys: boolean; canViewChannels: boolean };
  projectId?: string | null;
}): Promise<{ requests: Request[]; pageInfo: RequestConnection['pageInfo'] }> {
  const query = buildRequestsQuery(params.permissions);
  const variables =
    params.direction === 'older'
      ? { first: params.pageSize, after: params.cursor }
      : { last: params.pageSize, before: params.cursor };

  const where: Record<string, any> = { ...params.where };
  if (params.projectId) where.projectID = params.projectId;

  const headers = params.projectId ? { 'X-Project-ID': params.projectId } : undefined;
  const data = await graphqlRequest<{ requests: RequestConnection }>(
    query,
    { ...variables, where: Object.keys(where).length > 0 ? where : undefined, orderBy: { field: 'CREATED_AT', direction: 'DESC' } },
    headers
  );
  const result = requestConnectionSchema.parse(data?.requests);
  return { requests: result.edges.map((e) => e.node), pageInfo: result.pageInfo };
}

export function useRequestExecutions(
  requestID: string,
  variables?: {
    first?: number;
    after?: string;
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: Record<string, any>;
  },
  options?: { projectId?: string | null }
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const permissions = useRequestPermissions();
  const selectedProjectId = useSelectedProjectId();
  const projectId = options?.projectId !== undefined ? options.projectId : selectedProjectId;

  return useQuery({
    queryKey: ['request-executions', requestID, variables, permissions, projectId],
    queryFn: async () => {
      try {
        const query = buildRequestExecutionsQuery(permissions);
        const headers = projectId ? { 'X-Project-ID': projectId } : undefined;
        const finalVariables = {
          requestID,
          ...variables,
        };
        const data = await graphqlRequest<{ node: { executions: RequestExecutionConnection } }>(query, finalVariables, headers);
        return requestExecutionConnectionSchema.parse(data?.node?.executions);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!requestID,
  });
}
