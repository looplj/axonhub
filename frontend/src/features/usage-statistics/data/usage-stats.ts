import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import { usageStatsByUserSchema, type UsageStatsByUser } from '@/features/dashboard/data/dashboard';

const USAGE_STATS_BY_USER_QUERY = `
  query GetUsageStatsByUser($timeWindow: String) {
    usageStatsByUser(timeWindow: $timeWindow) {
      userId
      userName
      requestCount
      totalTokens
      totalCost
    }
  }
`;

export function useUsageStatsByUser(timeWindow?: string) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['usageStatsByUser', timeWindow, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ usageStatsByUser: UsageStatsByUser[] }>(
        USAGE_STATS_BY_USER_QUERY,
        { timeWindow },
        headers
      );
      return data.usageStatsByUser.map((item) => usageStatsByUserSchema.parse(item));
    },
    enabled: !!selectedProjectId,
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

