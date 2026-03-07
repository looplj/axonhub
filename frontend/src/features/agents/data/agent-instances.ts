import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';

export const agentInstanceSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  status: z.string(),
  platform: z.string().optional(),
  lastHeartbeatAt: z.coerce.date().optional(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
});

export type AgentInstance = z.infer<typeof agentInstanceSchema>;

export const agentInstanceEdgeSchema = z.object({
  node: agentInstanceSchema,
  cursor: z.string(),
});

export const agentInstanceConnectionSchema = z.object({
  edges: z.array(agentInstanceEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});

export type AgentInstanceConnection = z.infer<typeof agentInstanceConnectionSchema>;

const AGENT_INSTANCES_QUERY = `
  query GetAgentInstances($first: Int, $after: Cursor, $last: Int, $before: Cursor, $where: AgentInstanceWhereInput, $orderBy: AgentInstanceOrder) {
    agentInstances(first: $first, after: $after, last: $last, before: $before, where: $where, orderBy: $orderBy) {
      edges {
        node {
          id
          name
          description
          status
          platform
          lastHeartbeatAt
          createdAt
          updatedAt
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

interface QueryAgentInstancesArgs {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
  where?: Record<string, any>;
  orderBy?: {
    field: 'CREATED_AT' | 'UPDATED_AT' | 'STATUS';
    direction: 'ASC' | 'DESC';
  };
}

export function useQueryAgentInstances(args: QueryAgentInstancesArgs) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['agentInstances', args, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ agentInstances: AgentInstanceConnection }>(
        AGENT_INSTANCES_QUERY,
        args,
        headers
      );
      return agentInstanceConnectionSchema.parse(data.agentInstances);
    },
    enabled: !!selectedProjectId,
  });
}
