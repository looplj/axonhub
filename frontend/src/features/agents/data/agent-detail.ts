import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import { usePermissions } from '@/hooks/usePermissions';

const AGENT_DETAIL_QUERY = `
  query GetAgentDetail($id: ID!, $instancesFirst: Int) {
    node(id: $id) {
      ... on Agent {
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
        apiKey {
          key
        }
        instances(first: $instancesFirst) {
          edges {
            node {
              id
              instanceID
              name
              platform
              version
              lastHeartbeatAt
              createdAt
              updatedAt
            }
            cursor
          }
          totalCount
          pageInfo {
            hasNextPage
            hasPreviousPage
            startCursor
            endCursor
          }
        }
      }
    }
  }
`;

const AGENT_THREADS_QUERY = `
  query GetAgentThreadSummaries($agentID: ID!, $first: Int) {
    agentThreadSummaries(agentID: $agentID, first: $first) {
      threadID
      createdAt
    }
  }
`;

type AgentInstanceNode = {
  id: string;
  instanceID: string;
  name: string;
  platform: string;
  version: string;
  lastHeartbeatAt: string | Date;
  createdAt: string | Date;
  updatedAt: string | Date;
};

type AgentDetail = {
  id: string;
  createdAt: string | Date;
  updatedAt: string | Date;
  projectID: string;
  createdByUserID: string;
  name: string;
  description: string;
  status: 'enabled' | 'disabled' | 'archived';
  model: string;
  agentBuiltinTools: any;
  skillsPolicy: any;
  prompt?: { id?: string; content?: string } | null;
  apiKey?: { key?: string } | null;
  instances?: {
    edges?: { node: AgentInstanceNode }[];
    totalCount?: number;
  } | null;
};

type ThreadNode = {
  threadID: string;
  createdAt: string | Date;
};

export function useAgentDetail(id: string) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['agentDetail', id, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ node: AgentDetail | null }>(AGENT_DETAIL_QUERY, { id, instancesFirst: 50 }, headers);
      if (!data.node) return null;
      return data.node;
    },
    enabled: Boolean(selectedProjectId && id),
  });
}

export function useAgentThreads(agentId: string) {
  const selectedProjectId = useSelectedProjectId();
  const { hasScope } = usePermissions();
  const canReadThreads = hasScope('read_agents');

  return useQuery({
    queryKey: ['agentThreads', agentId, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ agentThreadSummaries: ThreadNode[] }>(
        AGENT_THREADS_QUERY,
        { agentID: agentId, first: 50 },
        headers
      );
      return data.agentThreadSummaries ?? [];
    },
    enabled: Boolean(selectedProjectId && agentId && canReadThreads),
  });
}
