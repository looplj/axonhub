import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

// Schema definitions
export const fastestChannelSchema = z.object({
  channelId: z.string(),
  channelName: z.string(),
  channelType: z.string(),
  throughput: z.number(),
  tokensCount: z.number(),
  latencyMs: z.number(),
  requestCount: z.number(),
});

export const fastestModelSchema = z.object({
  modelId: z.string(),
  modelName: z.string(),
  throughput: z.number(),
  tokensCount: z.number(),
  latencyMs: z.number(),
  requestCount: z.number(),
});

export const fastestChannelsInputSchema = z.object({
  timeWindow: z.string(),
});

// Type exports
export type FastestChannel = z.infer<typeof fastestChannelSchema>;
export type FastestModel = z.infer<typeof fastestModelSchema>;
export type FastestChannelsInput = z.infer<typeof fastestChannelsInputSchema>;

// GraphQL queries
const FASTEST_CHANNELS_QUERY = `
  query GetFastestChannels($input: FastestChannelsInput!) {
    fastestChannels(input: $input) {
      channelId
      channelName
      channelType
      throughput
      tokensCount
      latencyMs
      requestCount
    }
  }
`;

const FASTEST_MODELS_QUERY = `
  query GetFastestModels($input: FastestChannelsInput!) {
    fastestModels(input: $input) {
      modelId
      modelName
      throughput
      tokensCount
      latencyMs
      requestCount
    }
  }
`;

// Query hooks
export function useFastestChannels(timeWindow: string = '24h') {
  return useQuery({
    queryKey: ['fastestChannels', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ fastestChannels: FastestChannel[] }>(
        FASTEST_CHANNELS_QUERY,
        { input: { timeWindow } }
      );
      return data.fastestChannels.map((item) => fastestChannelSchema.parse(item));
    },
    refetchInterval: 30000, // Refetch every 30 seconds
  });
}

export function useFastestModels(timeWindow: string = '24h') {
  return useQuery({
    queryKey: ['fastestModels', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ fastestModels: FastestModel[] }>(
        FASTEST_MODELS_QUERY,
        { input: { timeWindow } }
      );
      return data.fastestModels.map((item) => fastestModelSchema.parse(item));
    },
    refetchInterval: 30000, // Refetch every 30 seconds
  });
}