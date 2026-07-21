import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { Trace, TraceConnection, TraceDetail, traceConnectionSchema, traceDetailSchema } from './schema';

// GraphQL query for traces
function buildTracesQuery() {
  return `
    query GetTraces(
      $first: Int
      $after: Cursor
      $orderBy: TraceOrder
      $where: TraceWhereInput
    ) {
      traces(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            traceID
            firstUserQuery
            status
            createdAt
            updatedAt
            thread {
              id
              threadID
            }
            requests(where: { status: completed }) {
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
  `;
}

// GraphQL query for trace detail
function buildTraceDetailQuery() {
  return `
    query GetTraceDetail($id: ID!) {
      node(id: $id) {
        ... on Trace {
          id
          traceID
          status
          createdAt
          updatedAt
          usageMetadata {
            totalInputTokens
            totalOutputTokens
            totalTokens
            totalCost
            totalCachedTokens
            totalCachedWriteTokens
          }
          project {
            id
            name
          }
          thread {
            id
            threadID
          }
          requests(where: { status: completed }) {
            totalCount
          }
        }
      }
    }
  `;
}

// GraphQL query for trace with request traces
function buildTraceWithRequestTracesQuery() {
  return `query GetTraceWithSegments($id: ID!) {
      node(id: $id) {
        ... on Trace {
          id
          traceID
          status
          createdAt
          updatedAt
          usageMetadata {
            totalInputTokens
            totalOutputTokens
            totalTokens
            totalCost
            totalCachedTokens
            totalCachedWriteTokens
          }
          project {
            id
            name
          }
          thread {
            id
            threadID
          }
          requests(where: { status: completed }) {
            totalCount
          }
          rawRootSegment
        }
      }
    }
  `;
}

// Query hooks
export function useTraces(variables?: {
  first?: number;
  after?: string;
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
  where?: {
    projectID?: string;
    threadID?: string;
    traceID?: string;
    [key: string]: any;
  };
}) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['traces', variables, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildTracesQuery();
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;

        // Add project filter if project is selected
        const finalVariables = {
          ...variables,
          where: {
            ...variables?.where,
            ...(selectedProjectId && { projectID: selectedProjectId }),
            // Default: exclude archived traces unless statusIn is explicitly set
            ...(variables?.where?.statusIn ? {} : { statusNEQ: variables?.where?.statusNEQ ?? 'archived' }),
          },
        };

        const data = await graphqlRequest<{ traces: TraceConnection }>(query, finalVariables, headers);
        return traceConnectionSchema.parse(data?.traces);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: true, // Traces can be queried without project selection for admin users
  });
}

export function useTrace(id: string) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['trace', id, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildTraceDetailQuery();
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const data = await graphqlRequest<{ node: Trace }>(query, { id }, headers);
        if (!data.node) {
          throw new Error('Trace not found');
        }
        return traceDetailSchema.parse(data.node);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!id,
  });
}

export function useTraceWithSegments(id: string) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['trace-with-segments', id, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildTraceWithRequestTracesQuery();
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const data = await graphqlRequest<{ node: TraceDetail }>(query, { id }, headers);
        if (!data.node) {
          throw new Error('Trace not found');
        }
        return traceDetailSchema.parse(data.node);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!id,
  });
}

// Backward compatibility alias
export const useTraceWithRequestTraces = useTraceWithSegments;

// Status mutation hooks
export function useArchiveTrace() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ archiveTrace: boolean }>(
        `mutation ArchiveTrace($id: ID!) { archiveTrace(id: $id) }`,
        { id }
      );
      return data.archiveTrace;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['traces'] });
      toast.success(t('traces.messages.archiveSuccess', 'Trace archived'));
    },
  });
}

export function useUnarchiveTrace() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ unarchiveTrace: boolean }>(
        `mutation UnarchiveTrace($id: ID!) { unarchiveTrace(id: $id) }`,
        { id }
      );
      return data.unarchiveTrace;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['traces'] });
      toast.success(t('traces.messages.unarchiveSuccess', 'Trace restored'));
    },
  });
}

export function useRetainTrace() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ retainTrace: boolean }>(
        `mutation RetainTrace($id: ID!) { retainTrace(id: $id) }`,
        { id }
      );
      return data.retainTrace;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['traces'] });
      toast.success(t('traces.messages.retainSuccess', 'Trace retained'));
    },
  });
}

export function useUnretainTrace() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ unretainTrace: boolean }>(
        `mutation UnretainTrace($id: ID!) { unretainTrace(id: $id) }`,
        { id }
      );
      return data.unretainTrace;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['traces'] });
      toast.success(t('traces.messages.unretainSuccess', 'Trace no longer retained'));
    },
  });
}
