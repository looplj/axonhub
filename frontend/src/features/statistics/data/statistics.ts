import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import type { ChannelStatistics, ModelStatistics } from './schema';

const endpoint = '/admin/graphql';

export type StatisticsTimeWindow = 'day' | 'week' | 'month';

// GraphQL Documents
export const CHANNEL_STATISTICS_QUERY = `
  query ChannelStatistics($timeWindow: StatisticsTimeWindow!) {
    channelStatistics(timeWindow: $timeWindow) {
      channelId
      channelName
      channelType
      requestCount
      promptTokens
      completionTokens
      cachedTokens
      avgTtftMs
      avgLatencyMs
      avgTps
      totalCost
    }
  }
`;

export const MODEL_STATISTICS_QUERY = `
  query ModelStatistics($channelId: ID, $timeWindow: StatisticsTimeWindow!) {
    modelStatistics(channelId: $channelId, timeWindow: $timeWindow) {
      modelId
      channelId
      channelName
      requestCount
      promptTokens
      completionTokens
      cachedTokens
      avgTtftMs
      avgLatencyMs
      avgTps
      totalCost
    }
  }
`;

// Hooks
export function useChannelStatistics(timeWindow: StatisticsTimeWindow) {
  return useQuery({
    queryKey: ['channelStatistics', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ channelStatistics: ChannelStatistics[] }>(
        CHANNEL_STATISTICS_QUERY,
        { timeWindow }
      );
      return data.channelStatistics;
    },
    refetchInterval: 60000,
  });
}

export function useModelStatistics(channelId?: string, timeWindow: StatisticsTimeWindow = 'day') {
  return useQuery({
    queryKey: ['modelStatistics', channelId, timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ modelStatistics: ModelStatistics[] }>(
        MODEL_STATISTICS_QUERY,
        { channelId, timeWindow }
      );
      return data.modelStatistics;
    },
    enabled: true,
    refetchInterval: 60000,
  });
}
