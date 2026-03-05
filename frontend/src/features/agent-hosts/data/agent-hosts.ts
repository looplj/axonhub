import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { graphqlRequest } from '@/gql/graphql';
import { useErrorHandler } from '@/hooks/use-error-handler';
import {
  AgentHost,
  AgentHostConnection,
  agentHostConnectionSchema,
  agentHostSchema,
  CreateAgentHostInput,
  UpdateAgentHostInput,
} from './schema';

const QUERY_AGENT_HOSTS = `
  query AgentHosts($after: Cursor, $first: Int, $before: Cursor, $last: Int, $orderBy: AgentHostOrder, $where: AgentHostWhereInput) {
    agentHosts(after: $after, first: $first, before: $before, last: $last, orderBy: $orderBy, where: $where) {
      edges {
        node {
          id
          createdAt
          updatedAt
          name
          type
          status
          addr
          user
          password
          authMethod
          sshPrivateKey
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

const CREATE_AGENT_HOST = `
  mutation CreateAgentHost($input: CreateAgentHostInput!) {
    createAgentHost(input: $input) {
      id
      createdAt
      updatedAt
      name
      type
      status
      addr
      user
      password
      authMethod
      sshPrivateKey
    }
  }
`;

const UPDATE_AGENT_HOST = `
  mutation UpdateAgentHost($id: ID!, $input: UpdateAgentHostInput!) {
    updateAgentHost(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      name
      type
      status
      addr
      user
      password
      authMethod
      sshPrivateKey
    }
  }
`;

const UPDATE_AGENT_HOST_STATUS = `
  mutation UpdateAgentHostStatus($id: ID!, $status: AgentHostStatus!) {
    updateAgentHostStatus(id: $id, status: $status) {
      id
      status
    }
  }
`;

const DELETE_AGENT_HOST = `
  mutation DeleteAgentHost($id: ID!) {
    deleteAgentHost(id: $id)
  }
`;

const BULK_DELETE_AGENT_HOSTS = `
  mutation BulkDeleteAgentHosts($ids: [ID!]!) {
    bulkDeleteAgentHosts(ids: $ids)
  }
`;

const BULK_UPDATE_AGENT_HOST_STATUS = `
  mutation BulkUpdateAgentHostStatus($ids: [ID!]!, $status: AgentHostStatus!) {
    bulkUpdateAgentHostStatus(ids: $ids, status: $status)
  }
`;

export type AgentHostOrderField = 'CREATED_AT' | 'UPDATED_AT';

export function useQueryAgentHosts(
  variables?: {
    first?: number;
    after?: string;
    before?: string;
    last?: number;
    where?: Record<string, unknown>;
    orderBy?: {
      field: AgentHostOrderField;
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
      'agentHosts',
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

        const data = await graphqlRequest<{ agentHosts: AgentHostConnection }>(
          QUERY_AGENT_HOSTS,
          queryVariables
        );
        return agentHostConnectionSchema.parse(data?.agentHosts);
      } catch (error) {
        handleError(error, t('agentHosts.errors.fetchList'));
        throw error;
      }
    },
  });
}

export function useCreateAgentHost() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (input: CreateAgentHostInput) => {
      const data = await graphqlRequest<{ createAgentHost: AgentHost }>(
        CREATE_AGENT_HOST,
        { input }
      );
      return agentHostSchema.parse(data.createAgentHost);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agentHosts'] });
      toast.success(t('agentHosts.messages.createSuccess'));
    },
    onError: (error) => {
      toast.error(t('agentHosts.messages.createError', { error: error.message }));
    },
  });
}

export function useUpdateAgentHost() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateAgentHostInput }) => {
      const data = await graphqlRequest<{ updateAgentHost: AgentHost }>(
        UPDATE_AGENT_HOST,
        { id, input }
      );
      return agentHostSchema.parse(data.updateAgentHost);
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['agentHosts'] });
      queryClient.invalidateQueries({ queryKey: ['agentHost', data.id] });
      toast.success(t('agentHosts.messages.updateSuccess'));
    },
    onError: (error) => {
      toast.error(t('agentHosts.messages.updateError', { error: error.message }));
    },
  });
}

export function useUpdateAgentHostStatus() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: 'active' | 'inactive' | 'error' }) => {
      const data = await graphqlRequest<{ updateAgentHostStatus: { id: string; status: string } }>(
        UPDATE_AGENT_HOST_STATUS,
        { id, status }
      );
      return data.updateAgentHostStatus;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['agentHosts'] });
      const statusText =
        variables.status === 'active'
          ? t('agentHosts.status.active')
          : variables.status === 'inactive'
            ? t('agentHosts.status.inactive')
            : t('agentHosts.status.error');
      toast.success(t('agentHosts.messages.statusUpdateSuccess', { status: statusText }));
    },
    onError: (error) => {
      toast.error(t('agentHosts.messages.statusUpdateError', { error: error.message }));
    },
  });
}

export function useDeleteAgentHost() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ deleteAgentHost: boolean }>(
        DELETE_AGENT_HOST,
        { id }
      );
      return data.deleteAgentHost;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agentHosts'] });
      toast.success(t('agentHosts.messages.deleteSuccess'));
    },
    onError: (error) => {
      toast.error(t('agentHosts.messages.deleteError', { error: error.message }));
    },
  });
}

export function useBulkDeleteAgentHosts() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkDeleteAgentHosts: boolean }>(
        BULK_DELETE_AGENT_HOSTS,
        { ids }
      );
      return data.bulkDeleteAgentHosts;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['agentHosts'] });
      toast.success(t('agentHosts.messages.bulkDeleteSuccess', { count: variables.length }));
    },
    onError: (error) => {
      toast.error(t('agentHosts.messages.bulkDeleteError', { error: error.message }));
    },
  });
}

export function useBulkUpdateAgentHostStatus() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async ({ ids, status }: { ids: string[]; status: 'active' | 'inactive' }) => {
      const data = await graphqlRequest<{ bulkUpdateAgentHostStatus: boolean }>(
        BULK_UPDATE_AGENT_HOST_STATUS,
        { ids, status }
      );
      return data.bulkUpdateAgentHostStatus;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['agentHosts'] });
      const statusText =
        variables.status === 'active'
          ? t('agentHosts.status.active')
          : t('agentHosts.status.inactive');
      toast.success(
        t('agentHosts.messages.bulkStatusUpdateSuccess', {
          count: variables.ids.length,
          status: statusText,
        })
      );
    },
    onError: (error) => {
      toast.error(t('agentHosts.messages.bulkStatusUpdateError', { error: error.message }));
    },
  });
}
