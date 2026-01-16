import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
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
            createdAt
            updatedAt
            project {
              id
              name
            }
            tracesSummary: traces(first: 1) {
              totalCount
            }
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
    ) {
      node(id: $id) {
        ... on Thread {
          id
          threadID
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
          tracesSummary: traces(first: 1) {
            totalCount
          }
          tracesConnection: traces(first: $tracesFirst, after: $tracesAfter, orderBy: $traceOrderBy) {
            edges {
              node {
                id
                traceID
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
          },
        };

        const data = await graphqlRequest<{ threads: ThreadConnection }>(query, finalVariables, headers);
        return threadConnectionSchema.parse(data?.threads);
      } catch (error) {
        handleError(error, t('threads.errors.fetchList'));
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
}: {
  id: string;
  tracesFirst?: number;
  tracesAfter?: string;
  traceOrderBy?: {
    field: 'CREATED_AT';
    direction: OrderDirection;
  };
}) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();
  const selectedProjectId = useSelectedProjectId();

  return useQuery<ThreadDetail>({
    queryKey: ['thread-detail', id, tracesFirst, tracesAfter, traceOrderBy, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildThreadDetailQuery();
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;

        const variables = {
          id,
          tracesFirst,
          tracesAfter,
          traceOrderBy,
        };

        const data = await graphqlRequest<{ node?: ThreadDetail | null }>(query, variables, headers);
        if (!data?.node) {
          throw new Error(t('threads.errors.notFound'));
        }

        return threadDetailSchema.parse(data.node);
      } catch (error) {
        handleError(error, t('threads.errors.fetchDetail'));
        throw error;
      }
    },
    enabled: !!id,
  });
}
