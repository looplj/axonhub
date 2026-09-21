import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

/** Per-channel request outcome counts and derived success rate. */
export const channelSuccessRateSchema = z.object({
  channelId: z.string(),
  channelName: z.string(),
  channelType: z.string(),
  channelDisabled: z.boolean(),
  successCount: z.number(),
  failedCount: z.number(),
  totalCount: z.number(),
  successRate: z.number(),
});

export type ChannelSuccessRate = z.infer<typeof channelSuccessRateSchema>;

const CHANNEL_SUCCESS_RATES_QUERY = `
  query GetChannelSuccessRates($timeWindow: String, $limit: Int) {
    channelSuccessRates(timeWindow: $timeWindow, limit: $limit) {
      channelId
      channelName
      channelType
      channelDisabled
      successCount
      failedCount
      totalCount
      successRate
    }
  }
`;

/** Channel success rates for a coarse time window, refreshed every 5 minutes. */
export function useChannelSuccessRates(limit?: number, timeWindow?: string) {
  return useQuery({
    queryKey: ['channelSuccessRates', limit, timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ channelSuccessRates: ChannelSuccessRate[] }>(
        CHANNEL_SUCCESS_RATES_QUERY,
        { ...(timeWindow != null && { timeWindow }), ...(limit != null && { limit }) }
      );
      return data.channelSuccessRates.map((item) => channelSuccessRateSchema.parse(item));
    },
    refetchInterval: 300000,
    placeholderData: (previousData) => previousData,
  });
}
