import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

// Schema definitions
export const requestStatsSchema = z.object({
  requestsToday: z.number(),
  requestsThisWeek: z.number(),
  requestsLastWeek: z.number(),
  requestsThisMonth: z.number(),
});

export const dashboardStatsSchema = z.object({
  totalRequests: z.number(),
  requestStats: requestStatsSchema,
  failedRequests: z.number(),
  averageResponseTime: z.number().nullable(),
});

export const requestsByChannelSchema = z.object({
  channelName: z.string(),
  count: z.number(),
});

export const requestsByModelSchema = z.object({
  modelId: z.string(),
  count: z.number(),
});

export const requestsByAPIKeySchema = z.object({
  apiKeyId: z.string(),
  apiKeyName: z.string(),
  count: z.number(),
});

export const tokensByAPIKeySchema = z.object({
  apiKeyId: z.string(),
  apiKeyName: z.string(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  cachedTokens: z.number(),
  reasoningTokens: z.number(),
  totalTokens: z.number(),
});

export const tokensByChannelSchema = z.object({
  channelName: z.string(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  cachedTokens: z.number(),
  reasoningTokens: z.number(),
  totalTokens: z.number(),
});

export const tokensByModelSchema = z.object({
  modelId: z.string(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  cachedTokens: z.number(),
  reasoningTokens: z.number(),
  totalTokens: z.number(),
});

export const costByChannelSchema = z.object({
  channelName: z.string(),
  cost: z.number(),
});

export const costByModelSchema = z.object({
  modelId: z.string(),
  cost: z.number(),
});

export const costByAPIKeySchema = z.object({
  apiKeyId: z.string(),
  apiKeyName: z.string(),
  cost: z.number(),
});

export const dailyRequestStatsSchema = z.object({
  date: z.string(),
  count: z.number(),
  tokens: z.number(),
  cost: z.number(),
});

export const hourlyRequestStatsSchema = z.object({
  hour: z.number(),
  count: z.number(),
});

export const topProjectsSchema = z.object({
  projectId: z.string(),
  projectName: z.string(),
  projectDescription: z.string(),
  requestCount: z.number(),
});

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

export const modelPerformanceStatSchema = z.object({
  date: z.string(),
  modelId: z.string(),
  throughput: z.number().nullable(),
  ttftMs: z.number().nullable(),
  requestCount: z.number(),
});

export const channelPerformanceStatSchema = z.object({
  date: z.string(),
  channelId: z.string(),
  channelName: z.string(),
  throughput: z.number().nullable(),
  ttftMs: z.number().nullable(),
  requestCount: z.number(),
});

export type RequestStats = z.infer<typeof requestStatsSchema>;
export type DashboardStats = z.infer<typeof dashboardStatsSchema>;
export type RequestsByChannel = z.infer<typeof requestsByChannelSchema>;
export type RequestsByModel = z.infer<typeof requestsByModelSchema>;
export type RequestsByAPIKey = z.infer<typeof requestsByAPIKeySchema>;
export type TokensByAPIKey = z.infer<typeof tokensByAPIKeySchema>;
export type TokensByChannel = z.infer<typeof tokensByChannelSchema>;
export type TokensByModel = z.infer<typeof tokensByModelSchema>;
export type CostByChannel = z.infer<typeof costByChannelSchema>;
export type CostByModel = z.infer<typeof costByModelSchema>;
export type CostByAPIKey = z.infer<typeof costByAPIKeySchema>;
export type DailyRequestStats = z.infer<typeof dailyRequestStatsSchema>;
export type HourlyRequestStats = z.infer<typeof hourlyRequestStatsSchema>;
export type TopProjects = z.infer<typeof topProjectsSchema>;
export type ChannelSuccessRate = z.infer<typeof channelSuccessRateSchema>;
export type ModelPerformanceStat = z.infer<typeof modelPerformanceStatSchema>;
export type ChannelPerformanceStat = z.infer<typeof channelPerformanceStatSchema>;

export const tokenStatsSchema = z.object({
  totalInputTokensToday: z.number(),
  totalOutputTokensToday: z.number(),
  totalCachedTokensToday: z.number(),
  totalInputTokensThisWeek: z.number(),
  totalOutputTokensThisWeek: z.number(),
  totalCachedTokensThisWeek: z.number(),
  totalInputTokensThisMonth: z.number(),
  totalOutputTokensThisMonth: z.number(),
  totalCachedTokensThisMonth: z.number(),
  totalInputTokensAllTime: z.number(),
  totalOutputTokensAllTime: z.number(),
  totalCachedTokensAllTime: z.number(),
  lastUpdated: z.string().nullable(),
});

export type TokenStats = z.infer<typeof tokenStatsSchema>;

// GraphQL queries
const DASHBOARD_STATS_QUERY = `
  query GetDashboardStats {
    dashboardOverview {
      totalRequests
      requestStats {
        requestsToday
        requestsThisWeek
        requestsLastWeek
        requestsThisMonth
      }
      failedRequests
      averageResponseTime
    }
  }
`;

const REQUESTS_BY_CHANNEL_QUERY = `
  query GetRequestsByChannel($timeWindow: String) {
    requestStatsByChannel(timeWindow: $timeWindow) {
      channelName
      count
    }
  }
`;

const REQUESTS_BY_MODEL_QUERY = `
  query GetRequestsByModel($timeWindow: String) {
    requestStatsByModel(timeWindow: $timeWindow) {
      modelId
      count
    }
  }
`;

const REQUESTS_BY_API_KEY_QUERY = `
  query GetRequestsByAPIKey($timeWindow: String) {
    requestStatsByAPIKey(timeWindow: $timeWindow) {
      apiKeyId
      apiKeyName
      count
    }
  }
`;

const TOKENS_BY_API_KEY_QUERY = `
  query GetTokensByAPIKey($timeWindow: String) {
    tokenStatsByAPIKey(timeWindow: $timeWindow) {
      apiKeyId
      apiKeyName
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      totalTokens
    }
  }
`;

const TOKENS_BY_CHANNEL_QUERY = `
  query GetTokensByChannel($timeWindow: String) {
    tokenStatsByChannel(timeWindow: $timeWindow) {
      channelName
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      totalTokens
    }
  }
`;

const TOKENS_BY_MODEL_QUERY = `
  query GetTokensByModel($timeWindow: String) {
    tokenStatsByModel(timeWindow: $timeWindow) {
      modelId
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      totalTokens
    }
  }
`;

const COST_BY_CHANNEL_QUERY = `
  query GetCostByChannel($timeWindow: String) {
    costStatsByChannel(timeWindow: $timeWindow) {
      channelName
      cost
    }
  }
`;

const COST_BY_MODEL_QUERY = `
  query GetCostByModel($timeWindow: String) {
    costStatsByModel(timeWindow: $timeWindow) {
      modelId
      cost
    }
  }
`;

const COST_BY_API_KEY_QUERY = `
  query GetCostByAPIKey($timeWindow: String) {
    costStatsByAPIKey(timeWindow: $timeWindow) {
      apiKeyId
      apiKeyName
      cost
    }
  }
`;

const DAILY_REQUEST_STATS_QUERY = `
  query GetDailyRequestStats {
    dailyRequestStats {
      date
      count
      tokens
      cost
    }
  }
`;

const HOURLY_REQUEST_STATS_QUERY = `
  query GetHourlyRequestStats($date: String) {
    hourlyRequestStats(date: $date) {
      hour
      count
    }
  }
`;

const TOP_PROJECTS_QUERY = `
  query GetTopProjects {
    topRequestsProjects {
      projectId
      projectName
      projectDescription
      requestCount
    }
  }
`;

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

const MODEL_PERFORMANCE_STATS_QUERY = `
  query ModelPerformanceStats {
    modelPerformanceStats {
      date
      modelId
      throughput
      ttftMs
      requestCount
    }
  }
`;

const CHANNEL_PERFORMANCE_STATS_QUERY = `
  query ChannelPerformanceStats {
    channelPerformanceStats {
      date
      channelId
      channelName
      throughput
      ttftMs
      requestCount
    }
  }
`;

// (removed) Old usageLogs-based token stats query is deprecated in favor of backend tokenStats aggregation

// Backend-provided token stats aggregation
const TOKEN_STATS_AGGR_QUERY = `
  query GetTokenStats {
    tokenStats {
      totalInputTokensToday
      totalOutputTokensToday
      totalCachedTokensToday
      totalInputTokensThisWeek
      totalOutputTokensThisWeek
      totalCachedTokensThisWeek
      totalInputTokensThisMonth
      totalOutputTokensThisMonth
      totalCachedTokensThisMonth
      totalInputTokensAllTime
      totalOutputTokensAllTime
      totalCachedTokensAllTime
      lastUpdated
    }
  }
`;

// Query hooks
export function useDashboardStats() {
  return useQuery({
    queryKey: ['dashboardStats'],
    queryFn: async () => {
      const data = await graphqlRequest<{ dashboardOverview: DashboardStats }>(DASHBOARD_STATS_QUERY);
      return dashboardStatsSchema.parse(data.dashboardOverview);
    },
    refetchInterval: 30000, // Refetch every 30 seconds
  });
}

export function useRequestsByChannel(timeWindow?: string) {
  return useQuery({
    queryKey: ['requestStatsByChannel', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ requestStatsByChannel: RequestsByChannel[] }>(
        REQUESTS_BY_CHANNEL_QUERY,
        { timeWindow }
      );
      return data.requestStatsByChannel.map((item) => requestsByChannelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useRequestsByModel(timeWindow?: string) {
  return useQuery({
    queryKey: ['requestStatsByModel', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ requestStatsByModel: RequestsByModel[] }>(
        REQUESTS_BY_MODEL_QUERY,
        { timeWindow }
      );
      return data.requestStatsByModel.map((item) => requestsByModelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useRequestsByAPIKey(timeWindow?: string) {
  return useQuery({
    queryKey: ['requestStatsByAPIKey', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ requestStatsByAPIKey: RequestsByAPIKey[] }>(
        REQUESTS_BY_API_KEY_QUERY,
        { timeWindow }
      );
      return data.requestStatsByAPIKey.map((item) => requestsByAPIKeySchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useTokensByAPIKey(timeWindow?: string) {
  return useQuery({
    queryKey: ['tokenStatsByAPIKey', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ tokenStatsByAPIKey: TokensByAPIKey[] }>(
        TOKENS_BY_API_KEY_QUERY,
        { timeWindow }
      );
      return data.tokenStatsByAPIKey.map((item) => tokensByAPIKeySchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useTokensByChannel(timeWindow?: string) {
  return useQuery({
    queryKey: ['tokenStatsByChannel', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ tokenStatsByChannel: TokensByChannel[] }>(
        TOKENS_BY_CHANNEL_QUERY,
        { timeWindow }
      );
      return data.tokenStatsByChannel.map((item) => tokensByChannelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useTokensByModel(timeWindow?: string) {
  return useQuery({
    queryKey: ['tokenStatsByModel', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ tokenStatsByModel: TokensByModel[] }>(
        TOKENS_BY_MODEL_QUERY,
        { timeWindow }
      );
      return data.tokenStatsByModel.map((item) => tokensByModelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useCostByChannel(timeWindow?: string) {
  return useQuery({
    queryKey: ['costStatsByChannel', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ costStatsByChannel: CostByChannel[] }>(
        COST_BY_CHANNEL_QUERY,
        { timeWindow }
      );
      return data.costStatsByChannel.map((item) => costByChannelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useCostByModel(timeWindow?: string) {
  return useQuery({
    queryKey: ['costStatsByModel', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ costStatsByModel: CostByModel[] }>(
        COST_BY_MODEL_QUERY,
        { timeWindow }
      );
      return data.costStatsByModel.map((item) => costByModelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useCostByAPIKey(timeWindow?: string) {
  return useQuery({
    queryKey: ['costStatsByAPIKey', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ costStatsByAPIKey: CostByAPIKey[] }>(
        COST_BY_API_KEY_QUERY,
        { timeWindow }
      );
      return data.costStatsByAPIKey.map((item) => costByAPIKeySchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useDailyRequestStats() {
  return useQuery({
    queryKey: ['dailyRequestStats'],
    queryFn: async () => {
      const data = await graphqlRequest<{ dailyRequestStats: DailyRequestStats[] }>(DAILY_REQUEST_STATS_QUERY);
      return data.dailyRequestStats.map((item) => dailyRequestStatsSchema.parse(item));
    },
    refetchInterval: 300000, // Refetch every 5 minutes
  });
}

export function useHourlyRequestStats(date?: string) {
  return useQuery({
    queryKey: ['hourlyRequestStats', date],
    queryFn: async () => {
      const data = await graphqlRequest<{ hourlyRequestStats: HourlyRequestStats[] }>(HOURLY_REQUEST_STATS_QUERY, { date });
      return data.hourlyRequestStats.map((item) => hourlyRequestStatsSchema.parse(item));
    },
    refetchInterval: 300000,
  });
}

export function useTopProjects() {
  return useQuery({
    queryKey: ['topRequestsProjects'],
    queryFn: async () => {
      const data = await graphqlRequest<{ topRequestsProjects: TopProjects[] }>(TOP_PROJECTS_QUERY);
      return data.topRequestsProjects.map((item) => topProjectsSchema.parse(item));
    },
    refetchInterval: 300000,
  });
}

export function useTokenStats() {
  return useQuery({
    queryKey: ['tokenStats'],
    queryFn: async () => {
      const data = await graphqlRequest<{ tokenStats: TokenStats }>(TOKEN_STATS_AGGR_QUERY);
      return tokenStatsSchema.parse(data.tokenStats);
    },
    refetchInterval: 300000, // Refetch every 5 minutes
  });
}

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

export function useModelPerformanceStats() {
  return useQuery({
    queryKey: ['modelPerformanceStats'],
    queryFn: async () => {
      const data = await graphqlRequest<{ modelPerformanceStats: ModelPerformanceStat[] }>(MODEL_PERFORMANCE_STATS_QUERY);
      return data.modelPerformanceStats.map((item) => modelPerformanceStatSchema.parse(item));
    },
    refetchInterval: 300000, // Refetch every 5 minutes
  });
}

export function useChannelPerformanceStats() {
  return useQuery({
    queryKey: ['channelPerformanceStats'],
    queryFn: async () => {
      const data = await graphqlRequest<{ channelPerformanceStats: ChannelPerformanceStat[] }>(CHANNEL_PERFORMANCE_STATS_QUERY);
      return data.channelPerformanceStats.map((item) => channelPerformanceStatSchema.parse(item));
    },
    refetchInterval: 300000, // Refetch every 5 minutes
  });
}
