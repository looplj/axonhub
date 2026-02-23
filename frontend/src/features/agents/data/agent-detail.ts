import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';

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
