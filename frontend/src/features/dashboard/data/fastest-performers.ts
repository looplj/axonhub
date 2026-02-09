import { z } from 'zod';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

// Refetch interval constant (30 seconds)
const REFETCH_INTERVAL_MS = 30000;

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

export const fastestChannelsInputSchema = z.object({
  timeWindow: z.string(),
  limit: z.number().optional().default(5),
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
export function useFastestChannels(timeWindow: string = 'day', limit: number = 5) {
  const { t } = useTranslation('dashboard');

  return useQuery({
    queryKey: ['fastestChannels', timeWindow, limit],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ fastestChannels: FastestChannel[] }>(
          FASTEST_CHANNELS_QUERY,
          { input: { timeWindow, limit } }
        );
        return data.fastestChannels.map((item) => fastestChannelSchema.parse(item));
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error(t('fastestPerformers.loadError'));
        throw new Error(t('fastestPerformers.loadError'));
      }
    },
    refetchInterval: REFETCH_INTERVAL_MS,
    placeholderData: (previousData) => previousData, // Keep previous data while fetching to prevent flash
  });
}

export function useFastestModels(timeWindow: string = 'day', limit: number = 5) {
  const { t } = useTranslation('dashboard');

  return useQuery({
    queryKey: ['fastestModels', timeWindow, limit],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ fastestModels: FastestModel[] }>(
          FASTEST_MODELS_QUERY,
          { input: { timeWindow, limit } }
        );
        return data.fastestModels.map((item) => fastestModelSchema.parse(item));
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error(t('fastestPerformers.loadError'));
        throw new Error(t('fastestPerformers.loadError'));
      }
    },
    refetchInterval: REFETCH_INTERVAL_MS,
    placeholderData: (previousData) => previousData, // Keep previous data while fetching to prevent flash
  });
}

