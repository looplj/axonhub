import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

// Schema definitions for regular queries
export const fastestChannelSchema = z.object({
  channelId: z.string(),
  channelName: z.string(),
  channelType: z.string(),
  throughput: z.number().nullable().transform(v => v ?? 0).default(0),
  tokensCount: z.number().nullable().transform(v => v ?? 0).default(0),
  latencyMs: z.number().nullable().transform(v => v ?? 0).default(0),
  requestCount: z.number().nullable().transform(v => v ?? 0).default(0),
  confidenceLevel: z.enum(['high', 'medium', 'low']).nullable().transform(v => v ?? 'medium').default('medium'),
});

export const fastestModelSchema = z.object({
  modelId: z.string(),
  modelName: z.string(),
  throughput: z.number().nullable().transform(v => v ?? 0).default(0),
  tokensCount: z.number().nullable().transform(v => v ?? 0).default(0),
  latencyMs: z.number().nullable().transform(v => v ?? 0).default(0),
  requestCount: z.number().nullable().transform(v => v ?? 0).default(0),
  confidenceLevel: z.enum(['high', 'medium', 'low']).nullable().transform(v => v ?? 'medium').default('medium'),
});

export const fastestChannelsInputSchema = z.object({
  timeWindow: z.string(),
  limit: z.number().optional().default(5),
});

// Type exports
export type FastestChannel = z.infer<typeof fastestChannelSchema>;
export type FastestModel = z.infer<typeof fastestModelSchema>;
export type FastestChannelsInput = z.infer<typeof fastestChannelsInputSchema>;
export type ConfidenceLevel = 'high' | 'medium' | 'low';

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
      confidenceLevel
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
      confidenceLevel
    }
  }
`;

// Query hooks
export function useFastestChannels(timeWindow: string = 'day', limit: number = 5) {
  return useQuery({
    queryKey: ['fastestChannels', timeWindow, limit],
    queryFn: async () => {
      const data = await graphqlRequest<{ fastestChannels: FastestChannel[] }>(
        FASTEST_CHANNELS_QUERY,
        { input: { timeWindow, limit } }
      );
      return data.fastestChannels.map((item) => fastestChannelSchema.parse(item));
    },
    refetchInterval: 30000, // Refetch every 30 seconds
    placeholderData: (previousData) => previousData, // Keep previous data while fetching to prevent flash
  });
}

export function useFastestModels(timeWindow: string = 'day', limit: number = 5) {
  return useQuery({
    queryKey: ['fastestModels', timeWindow, limit],
    queryFn: async () => {
      const data = await graphqlRequest<{ fastestModels: FastestModel[] }>(
        FASTEST_MODELS_QUERY,
        { input: { timeWindow, limit } }
      );
      return data.fastestModels.map((item) => fastestModelSchema.parse(item));
    },
    refetchInterval: 30000, // Refetch every 30 seconds
    placeholderData: (previousData) => previousData, // Keep previous data while fetching to prevent flash
  });
}
