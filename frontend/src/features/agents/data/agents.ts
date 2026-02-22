import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import type { Agent, AgentConnection, CreateAgentInput, UpdateAgentInput } from './schema';
import { agentConnectionSchema, agentSchema } from './schema';

const AGENTS_QUERY = `
  query GetAgents($first: Int, $after: Cursor, $last: Int, $before: Cursor, $where: AgentWhereInput, $orderBy: AgentOrder) {
    agents(first: $first, after: $after, last: $last, before: $before, where: $where, orderBy: $orderBy) {
      edges {
        node {
          id
          createdAt
          updatedAt
          projectID
          createdByUserID
          name
          description
          status
          model
          agentBuiltinTools {
            name
            enabled
            order
            config
          }
          skillsPolicy {
            add
          }
          prompt {
            id
            content
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

const CREATE_AGENT_MUTATION = `
  mutation CreateAgent($input: CreateAgentInput!) {
    createAgent(input: $input) {
      id
      createdAt
      updatedAt
      projectID
      createdByUserID
      name
      description
      status
      model
      agentBuiltinTools {
        name
        enabled
        order
        config
      }
      skillsPolicy {
        add
      }
      prompt {
        id
        content
      }
    }
  }
`;

const UPDATE_AGENT_MUTATION = `
  mutation UpdateAgent($id: ID!, $input: UpdateAgentInput!) {
    updateAgent(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      projectID
      createdByUserID
      name
      description
      status
      model
      agentBuiltinTools {
        name
        enabled
        order
        config
      }
      skillsPolicy {
        add
      }
      prompt {
        id
        content
      }
    }
  }
`;

const DELETE_AGENT_MUTATION = `
  mutation DeleteAgent($id: ID!) {
    deleteAgent(id: $id)
  }
`;

interface QueryAgentsArgs {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
  where?: Record<string, any>;
  orderBy?: {
    field: 'CREATED_AT' | 'UPDATED_AT';
    direction: 'ASC' | 'DESC';
  };
}

export function useQueryAgents(args: QueryAgentsArgs) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['agents', args, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ agents: AgentConnection }>(AGENTS_QUERY, args, headers);
      return agentConnectionSchema.parse(data.agents);
    },
    enabled: !!selectedProjectId,
  });
}

export function useCreateAgent() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (input: CreateAgentInput) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ createAgent: Agent }>(CREATE_AGENT_MUTATION, { input }, headers);
      return agentSchema.parse(data.createAgent);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] });
      toast.success(t('agents.messages.createSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('agents.messages.createError', { error: error.message }));
    },
  });
}

export function useUpdateAgent() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateAgentInput }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ updateAgent: Agent }>(UPDATE_AGENT_MUTATION, { id, input }, headers);
      return agentSchema.parse(data.updateAgent);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] });
      toast.success(t('agents.messages.updateSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('agents.messages.updateError', { error: error.message }));
    },
  });
}

export function useDeleteAgent() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (id: string) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      await graphqlRequest(DELETE_AGENT_MUTATION, { id }, headers);
      return true;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] });
      toast.success(t('agents.messages.deleteSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('agents.messages.deleteError', { error: error.message }));
    },
  });
}

