import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { ThreadConnection, ThreadDetail, threadConnectionSchema, threadDetailSchema } from './schema';

type ThreadOrderField = 'CREATED_AT' | 'UPDATED_AT';

type OrderDirection = 'ASC' | 'DESC';

type ThreadOrder = {
  field: ThreadOrderField;
  direction: OrderDirection;
};

type ThreadWhereInput = {
  projectID?: string;
  threadID?: string;
  threadIDContains?: string;
  [key: string]: unknown;
};

function buildThreadsQuery() {
  return `
    query GetThreads(
      $first: Int
      $after: Cursor
      $orderBy: ThreadOrder
      $where: ThreadWhereInput
    ) {
      threads(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            threadID
            status
            createdAt
            updatedAt
            project {
              id
              name
            }
            tracesSummary: traces(first: 1, where: { statusNEQ: archived }) {
              totalCount
            }
            archivedTracesCount
            firstUserQuery
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

function buildThreadDetailQuery() {
  return `
    query GetThreadDetail(
      $id: ID!
      $tracesFirst: Int
      $tracesAfter: Cursor
      $traceOrderBy: TraceOrder
      $traceWhere: TraceWhereInput
    ) {
      node(id: $id) {
        ... on Thread {
          id
          threadID
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
          tracesSummary: traces(first: 1, where: { statusNEQ: archived }) {
            totalCount
          }
          archivedTracesCount
          tracesConnection: traces(first: $tracesFirst, after: $tracesAfter, orderBy: $traceOrderBy, where: $traceWhere) {
            edges {
              node {
                id
                traceID
                status
                createdAt
                updatedAt
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
                firstUserQuery
                firstText
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

export function useThreads(variables?: { first?: number; after?: string; orderBy?: ThreadOrder; where?: ThreadWhereInput }) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();
  const selectedProjectId = useSelectedProjectId();

  return useQuery<ThreadConnection>({
    queryKey: ['threads', variables, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildThreadsQuery();
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const finalVariables = {
          ...variables,
          where: {
            ...variables?.where,
            ...(selectedProjectId && { projectID: selectedProjectId }),
            // Default: exclude archived threads unless statusIn is explicitly set
            ...(variables?.where?.statusIn ? {} : { statusNEQ: variables?.where?.statusNEQ ?? 'archived' }),
          },
        };

        const data = await graphqlRequest<{ threads: ThreadConnection }>(query, finalVariables, headers);
        return threadConnectionSchema.parse(data?.threads);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: true,
  });
}

export function useThreadDetail({
  id,
  tracesFirst,
  tracesAfter,
  traceOrderBy,
  showArchivedTraces = false,
}: {
  id: string;
  tracesFirst?: number;
  tracesAfter?: string;
  traceOrderBy?: {
    field: 'CREATED_AT';
    direction: OrderDirection;
  };
  showArchivedTraces?: boolean;
}) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();
  const selectedProjectId = useSelectedProjectId();

  return useQuery<ThreadDetail>({
    queryKey: ['thread-detail', id, tracesFirst, tracesAfter, traceOrderBy, selectedProjectId, showArchivedTraces],
    queryFn: async () => {
      try {
        const query = buildThreadDetailQuery();
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;

        const variables = {
          id,
          tracesFirst,
          tracesAfter,
          traceOrderBy,
          traceWhere: showArchivedTraces ? {} : { statusNEQ: 'archived' },
        };

        const data = await graphqlRequest<{ node?: ThreadDetail | null }>(query, variables, headers);
        if (!data?.node) {
          throw new Error(t('threads.errors.notFound'));
        }

        return threadDetailSchema.parse(data.node);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!id,
  });
}

// Status mutation hooks
export function useArchiveThread() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ archiveThread: boolean }>(
        `mutation ArchiveThread($id: ID!) { archiveThread(id: $id) }`,
        { id }
      );
      return data.archiveThread;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['threads'] });
      queryClient.invalidateQueries({ queryKey: ['traces'] });
      toast.success(t('threads.messages.archiveSuccess', 'Thread archived'));
    },
  });
}

export function useUnarchiveThread() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ unarchiveThread: boolean }>(
        `mutation UnarchiveThread($id: ID!) { unarchiveThread(id: $id) }`,
        { id }
      );
      return data.unarchiveThread;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['threads'] });
      queryClient.invalidateQueries({ queryKey: ['traces'] });
      toast.success(t('threads.messages.unarchiveSuccess', 'Thread restored'));
    },
  });
}

export function useRetainThread() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ retainThread: boolean }>(
        `mutation RetainThread($id: ID!) { retainThread(id: $id) }`,
        { id }
      );
      return data.retainThread;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['threads'] });
      queryClient.invalidateQueries({ queryKey: ['traces'] });
      toast.success(t('threads.messages.retainSuccess', 'Thread retained'));
    },
  });
}

export function useUnretainThread() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ unretainThread: boolean }>(
        `mutation UnretainThread($id: ID!) { unretainThread(id: $id) }`,
        { id }
      );
      return data.unretainThread;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['threads'] });
      queryClient.invalidateQueries({ queryKey: ['traces'] });
      toast.success(t('threads.messages.unretainSuccess', 'Thread no longer retained'));
    },
  });
}
