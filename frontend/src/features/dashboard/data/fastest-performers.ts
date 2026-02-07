import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

// Schema definitions for regular queries
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

// Schema definitions for expanded queries
export const fastestModelInChannelSchema = z.object({
  modelId: z.string(),
  modelName: z.string(),
  throughput: z.number(),
  tokensCount: z.number(),
  latencyMs: z.number(),
  requestCount: z.number(),
});

export const fastestChannelExpandedSchema = z.object({
  channelId: z.string(),
  channelName: z.string(),
  channelType: z.string(),
  throughput: z.number(),
  tokensCount: z.number(),
  latencyMs: z.number(),
  requestCount: z.number(),
  models: z.array(fastestModelInChannelSchema),
});

export const fastestChannelForModelSchema = z.object({
  channelId: z.string(),
  channelName: z.string(),
  channelType: z.string(),
  throughput: z.number(),
  tokensCount: z.number(),
  latencyMs: z.number(),
  requestCount: z.number(),
});

export const fastestModelExpandedSchema = z.object({
  modelId: z.string(),
  modelName: z.string(),
  throughput: z.number(),
  tokensCount: z.number(),
  latencyMs: z.number(),
  requestCount: z.number(),
  channels: z.array(fastestChannelForModelSchema),
});

export const fastestChannelsInputSchema = z.object({
  timeWindow: z.string(),
});

// Type exports
export type FastestChannel = z.infer<typeof fastestChannelSchema>;
export type FastestModel = z.infer<typeof fastestModelSchema>;
export type FastestModelInChannel = z.infer<typeof fastestModelInChannelSchema>;
export type FastestChannelExpanded = z.infer<typeof fastestChannelExpandedSchema>;
export type FastestChannelForModel = z.infer<typeof fastestChannelForModelSchema>;
export type FastestModelExpanded = z.infer<typeof fastestModelExpandedSchema>;
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

const FASTEST_CHANNELS_EXPANDED_QUERY = `
  query GetFastestChannelsExpanded($input: FastestChannelsInput!) {
    fastestChannelsExpanded(input: $input) {
      channelId
      channelName
      channelType
      throughput
      tokensCount
      latencyMs
      requestCount
      models {
        modelId
        modelName
        throughput
        tokensCount
        latencyMs
        requestCount
      }
    }
  }
`;

const FASTEST_MODELS_EXPANDED_QUERY = `
  query GetFastestModelsExpanded($input: FastestChannelsInput!) {
    fastestModelsExpanded(input: $input) {
      modelId
      modelName
      throughput
      tokensCount
      latencyMs
      requestCount
      channels {
        channelId
        channelName
        channelType
        throughput
        tokensCount
        latencyMs
        requestCount
      }
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

// Expanded query hooks (only run when enabled)
export function useFastestChannelsExpanded(timeWindow: string = '24h', enabled: boolean = false) {
  return useQuery({
    queryKey: ['fastestChannelsExpanded', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ fastestChannelsExpanded: FastestChannelExpanded[] }>(
        FASTEST_CHANNELS_EXPANDED_QUERY,
        { input: { timeWindow } }
      );
      return data.fastestChannelsExpanded.map((item) => fastestChannelExpandedSchema.parse(item));
    },
    enabled,
    refetchInterval: 30000,
  });
}

export function useFastestModelsExpanded(timeWindow: string = '24h', enabled: boolean = false) {
  return useQuery({
    queryKey: ['fastestModelsExpanded', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ fastestModelsExpanded: FastestModelExpanded[] }>(
        FASTEST_MODELS_EXPANDED_QUERY,
        { input: { timeWindow } }
      );
      return data.fastestModelsExpanded.map((item) => fastestModelExpandedSchema.parse(item));
    },
    enabled,
    refetchInterval: 30000,
  });
}
