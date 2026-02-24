import { z } from 'zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import {
  AgentRuntime,
  AgentRuntimeConnection,
  CreateAgentRuntimeInput,
  UpdateAgentRuntimeInput,
  agentRuntimeConnectionSchema,
  agentRuntimeSchema,
} from './schema';

const QUERY_AGENT_RUNTIMES_QUERY = `
  query AgentRuntimes($after: Cursor, $first: Int, $before: Cursor, $last: Int, $orderBy: AgentRuntimeOrder, $where: AgentRuntimeWhereInput) {
    agentRuntimes(after: $after, first: $first, before: $before, last: $last, orderBy: $orderBy, where: $where) {
      edges {
        node {
          id
          createdAt
          updatedAt
          name
          type
          status
          host
          user
          password
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

const CREATE_AGENT_RUNTIME_MUTATION = `
  mutation CreateAgentRuntime($input: CreateAgentRuntimeInput!) {
    createAgentRuntime(input: $input) {
      id
      createdAt
      updatedAt
      name
      type
      status
      host
      user
      password
    }
  }
`;

const UPDATE_AGENT_RUNTIME_MUTATION = `
  mutation UpdateAgentRuntime($id: ID!, $input: UpdateAgentRuntimeInput!) {
    updateAgentRuntime(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      name
      type
      status
      host
      user
      password
    }
  }
`;

const UPDATE_AGENT_RUNTIME_STATUS_MUTATION = `
  mutation UpdateAgentRuntimeStatus($id: ID!, $status: AgentRuntimeStatus!) {
    updateAgentRuntimeStatus(id: $id, status: $status) {
      id
      status
    }
  }
`;

const DELETE_AGENT_RUNTIME_MUTATION = `
  mutation DeleteAgentRuntime($id: ID!) {
    deleteAgentRuntime(id: $id)
  }
`;

const BULK_DELETE_AGENT_RUNTIMES_MUTATION = `
  mutation BulkDeleteAgentRuntimes($ids: [ID!]!) {
    bulkDeleteAgentRuntimes(ids: $ids)
  }
`;

const BULK_UPDATE_AGENT_RUNTIME_STATUS_MUTATION = `
  mutation BulkUpdateAgentRuntimeStatus($ids: [ID!]!, $status: AgentRuntimeStatus!) {
    bulkUpdateAgentRuntimeStatus(ids: $ids, status: $status)
  }
`;

export type AgentRuntimeOrderField = 'CREATED_AT' | 'UPDATED_AT';

export function useQueryAgentRuntimes(
  variables?: {
    first?: number;
    after?: string;
    before?: string;
    last?: number;
    where?: Record<string, unknown>;
    orderBy?: {
      field: AgentRuntimeOrderField;
      direction: 'ASC' | 'DESC';
    };
  },
  options?: {
    disableAutoFetch?: boolean;
  }
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    enabled: !options?.disableAutoFetch,
    queryKey: [
      'agentRuntimes',
      variables?.where,
      variables?.orderBy?.field,
      variables?.orderBy?.direction,
      variables?.first,
      variables?.last,
      variables?.after,
      variables?.before,
    ],
    queryFn: async () => {
      try {
        const queryVariables: Record<string, unknown> = {
          first: variables?.first,
          after: variables?.after,
          before: variables?.before,
          last: variables?.last,
          where: variables?.where,
        };

        if (variables?.orderBy) {
          queryVariables.orderBy = {
            direction: variables.orderBy.direction,
            field: variables.orderBy.field,
          };
        }

        const data = await graphqlRequest<{ agentRuntimes: AgentRuntimeConnection }>(
          QUERY_AGENT_RUNTIMES_QUERY,
          queryVariables
        );
        return agentRuntimeConnectionSchema.parse(data?.agentRuntimes);
      } catch (error) {
        handleError(error, t('agentRuntimes.errors.fetchList'));
        throw error;
      }
    },
  });
}

export function useCreateAgentRuntime() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (input: CreateAgentRuntimeInput) => {
      const data = await graphqlRequest<{ createAgentRuntime: AgentRuntime }>(
        CREATE_AGENT_RUNTIME_MUTATION,
        { input }
      );
      return agentRuntimeSchema.parse(data.createAgentRuntime);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agentRuntimes'] });
      toast.success(t('agentRuntimes.messages.createSuccess'));
    },
    onError: (error) => {
      toast.error(t('agentRuntimes.messages.createError', { error: error.message }));
    },
  });
}

export function useUpdateAgentRuntime() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateAgentRuntimeInput }) => {
      const data = await graphqlRequest<{ updateAgentRuntime: AgentRuntime }>(
        UPDATE_AGENT_RUNTIME_MUTATION,
        { id, input }
      );
      return agentRuntimeSchema.parse(data.updateAgentRuntime);
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['agentRuntimes'] });
      queryClient.invalidateQueries({ queryKey: ['agentRuntime', data.id] });
      toast.success(t('agentRuntimes.messages.updateSuccess'));
    },
    onError: (error) => {
      toast.error(t('agentRuntimes.messages.updateError', { error: error.message }));
    },
  });
}

export function useUpdateAgentRuntimeStatus() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: 'active' | 'inactive' | 'error' }) => {
      const data = await graphqlRequest<{ updateAgentRuntimeStatus: { id: string; status: string } }>(
        UPDATE_AGENT_RUNTIME_STATUS_MUTATION,
        { id, status }
      );
      return data.updateAgentRuntimeStatus;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['agentRuntimes'] });
      const statusText =
        variables.status === 'active'
          ? t('agentRuntimes.status.active')
          : variables.status === 'inactive'
            ? t('agentRuntimes.status.inactive')
            : t('agentRuntimes.status.error');
      toast.success(t('agentRuntimes.messages.statusUpdateSuccess', { status: statusText }));
    },
    onError: (error) => {
      toast.error(t('agentRuntimes.messages.statusUpdateError', { error: error.message }));
    },
  });
}

export function useDeleteAgentRuntime() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ deleteAgentRuntime: boolean }>(
        DELETE_AGENT_RUNTIME_MUTATION,
        { id }
      );
      return data.deleteAgentRuntime;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agentRuntimes'] });
      toast.success(t('agentRuntimes.messages.deleteSuccess'));
    },
    onError: (error) => {
      toast.error(t('agentRuntimes.messages.deleteError', { error: error.message }));
    },
  });
}

export function useBulkDeleteAgentRuntimes() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkDeleteAgentRuntimes: boolean }>(
        BULK_DELETE_AGENT_RUNTIMES_MUTATION,
        { ids }
      );
      return data.bulkDeleteAgentRuntimes;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['agentRuntimes'] });
      toast.success(t('agentRuntimes.messages.bulkDeleteSuccess', { count: variables.length }));
    },
    onError: (error) => {
      toast.error(t('agentRuntimes.messages.bulkDeleteError', { error: error.message }));
    },
  });
}

export function useBulkUpdateAgentRuntimeStatus() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async ({ ids, status }: { ids: string[]; status: 'active' | 'inactive' }) => {
      const data = await graphqlRequest<{ bulkUpdateAgentRuntimeStatus: boolean }>(
        BULK_UPDATE_AGENT_RUNTIME_STATUS_MUTATION,
        { ids, status }
      );
      return data.bulkUpdateAgentRuntimeStatus;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['agentRuntimes'] });
      const statusText = variables.status === 'active' ? t('agentRuntimes.status.active') : t('agentRuntimes.status.inactive');
      toast.success(t('agentRuntimes.messages.bulkStatusUpdateSuccess', { count: variables.ids.length, status: statusText }));
    },
    onError: (error) => {
      toast.error(t('agentRuntimes.messages.bulkStatusUpdateError', { error: error.message }));
    },
  });
}
