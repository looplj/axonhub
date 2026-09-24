import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
// --- Zod Schemas ---

/** Analytics filter: time range plus the optional dimension selectors. */
export const analyticsFilterSchema = z.object({
  // A rolling window the backend measures from the moment the query runs; it overrides
  // startTime/endTime, which are calendar dates and cannot express it.
  timeWindow: z.string().nullable().optional(),
  startTime: z.string().nullable().optional(), // 'YYYY-MM-DD' 或 ISO timestamp
  endTime: z.string().nullable().optional(),
  projectIDs: z.array(z.string()).optional(),
  channelIDs: z.array(z.string()).optional(),
  modelIDs: z.array(z.string()).optional(),
  apiKeyIDs: z.array(z.string()).optional(),
  userIDs: z.array(z.string()).optional(),
});

export type AnalyticsFilter = z.infer<typeof analyticsFilterSchema>;

/** Aggregate totals for the filtered range, including the success rate computed
 * from request_executions. */
export const analyticsOverviewSchema = z.object({
  totalTokens: z.number(),
  totalInputTokens: z.number(),
  totalCachedInputTokens: z.number(),
  totalUncachedInputTokens: z.number(),
  totalOutputTokens: z.number(),
  totalRequests: z.number(),
  totalCost: z.number(),
  failedRequests: z.number(),
  successRate: z.number(),
});

export type AnalyticsOverview = z.infer<typeof analyticsOverviewSchema>;

/** One row of the daily trend series. */
export const analyticsDailyStatSchema = z.object({
  date: z.string(),
  inputTokens: z.number(),
  cachedInputTokens: z.number(),
  uncachedInputTokens: z.number(),
  outputTokens: z.number(),
  totalTokens: z.number(),
  requestCount: z.number(),
  cost: z.number(),
});

export type AnalyticsDailyStat = z.infer<typeof analyticsDailyStatSchema>;

/** One row of a grouped breakdown (channel, model, API key or user). */
export const analyticsDimensionStatSchema = z.object({
  id: z.string(),
  name: z.string(),
  requestCount: z.number(),
  inputTokens: z.number(),
  cachedInputTokens: z.number(),
  outputTokens: z.number(),
  totalTokens: z.number(),
  cost: z.number(),
});

export type AnalyticsDimensionStat = z.infer<typeof analyticsDimensionStatSchema>;

/** Earliest date with data, used to bound the "all time" preset. */
export const analyticsMetadataSchema = z.object({
  earliestDate: z.string().nullable().optional(),
});

export type AnalyticsMetadata = z.infer<typeof analyticsMetadataSchema>;

// --- GraphQL Queries ---

const ANALYTICS_METADATA_QUERY = `
  query GetAnalyticsMetadata {
    analyticsMetadata {
      earliestDate
    }
  }
`;

const ANALYTICS_OVERVIEW_QUERY = `
  query GetAnalyticsOverview($filter: AnalyticsFilter) {
    analyticsOverview(filter: $filter) {
      totalTokens
      totalInputTokens
      totalCachedInputTokens
      totalUncachedInputTokens
      totalOutputTokens
      totalRequests
      totalCost
      failedRequests
      successRate
    }
  }
`;

const ANALYTICS_DAILY_STATS_QUERY = `
  query GetAnalyticsDailyStats($filter: AnalyticsFilter) {
    analyticsDailyStats(filter: $filter) {
      date
      inputTokens
      cachedInputTokens
      uncachedInputTokens
      outputTokens
      totalTokens
      requestCount
      cost
    }
  }
`;

const ANALYTICS_DIMENSION_STATS_QUERY = `
  query GetAnalyticsDimensionStats($filter: AnalyticsFilter, $dimension: String!) {
    analyticsDimensionStats(filter: $filter, dimension: $dimension) {
      id
      name
      requestCount
      inputTokens
      cachedInputTokens
      outputTokens
      totalTokens
      cost
    }
  }
`;

// --- Helper: convert filter to GraphQL input ---

/** Forward the filter as GraphQL input, dropping empty dimensions so the backend
 * does not receive empty arrays that would narrow the query to nothing. */
export function toGraphQLFilter(filter: AnalyticsFilter | null): Record<string, unknown> | null {
  if (!filter) return null;

  const result: Record<string, unknown> = {};

  if (filter.timeWindow) result.timeWindow = filter.timeWindow;
  if (filter.startTime) result.startTime = filter.startTime;
  if (filter.endTime) result.endTime = filter.endTime;
  if (filter.projectIDs && filter.projectIDs.length > 0) result.projectIDs = filter.projectIDs;
  if (filter.channelIDs && filter.channelIDs.length > 0) result.channelIDs = filter.channelIDs;
  if (filter.modelIDs && filter.modelIDs.length > 0) result.modelIDs = filter.modelIDs;
  if (filter.apiKeyIDs && filter.apiKeyIDs.length > 0) result.apiKeyIDs = filter.apiKeyIDs;
  if (filter.userIDs && filter.userIDs.length > 0) result.userIDs = filter.userIDs;

  return Object.keys(result).length > 0 ? result : null;
}

// --- React Query Hooks ---

/** Earliest date with data; cached for 5 minutes because it changes rarely. */
export function useAnalyticsMetadata() {
  return useQuery({
    queryKey: ['analyticsMetadata'],
    queryFn: async () => {
      const data = await graphqlRequest<{ analyticsMetadata: AnalyticsMetadata }>(
        ANALYTICS_METADATA_QUERY
      );
      return analyticsMetadataSchema.parse(data.analyticsMetadata);
    },
    staleTime: 5 * 60 * 1000,
  });
}

/** Aggregate totals for the filtered range. */
export function useAnalyticsOverview(filter: AnalyticsFilter | null) {
  return useQuery({
    queryKey: ['analyticsOverview', filter],
    queryFn: async () => {
      const gqlFilter = toGraphQLFilter(filter);
      const data = await graphqlRequest<{ analyticsOverview: AnalyticsOverview }>(
        ANALYTICS_OVERVIEW_QUERY,
        { filter: gqlFilter }
      );
      return analyticsOverviewSchema.parse(data.analyticsOverview);
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

/** Daily trend series for the filtered range. */
export function useAnalyticsDailyStats(filter: AnalyticsFilter | null) {
  return useQuery({
    queryKey: ['analyticsDailyStats', filter],
    queryFn: async () => {
      const gqlFilter = toGraphQLFilter(filter);
      const data = await graphqlRequest<{ analyticsDailyStats: AnalyticsDailyStat[] }>(
        ANALYTICS_DAILY_STATS_QUERY,
        { filter: gqlFilter }
      );
      return data.analyticsDailyStats.map((item) => analyticsDailyStatSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

/** Breakdown for one dimension; disabled until a dimension is selected. */
export function useAnalyticsDimensionStats(filter: AnalyticsFilter | null, dimension: string, enabled = true) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['analyticsDimensionStats', filter, dimension, selectedProjectId],
    queryFn: async () => {
      const gqlFilter = toGraphQLFilter(filter);
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ analyticsDimensionStats: AnalyticsDimensionStat[] }>(
        ANALYTICS_DIMENSION_STATS_QUERY,
        { filter: gqlFilter, dimension },
        headers
      );
      return data.analyticsDimensionStats.map((item) => analyticsDimensionStatSchema.parse(item));
    },
    enabled: enabled && !!dimension,
    refetchInterval: 60000,
  });
}
