import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';

// --- Schemas ---

export const requestStatsSchema = z.object({
  requestsToday: z.number(),
  requestsThisWeek: z.number(),
  requestsLastWeek: z.number(),
  requestsThisMonth: z.number(),
});

/** Trailing-24h throughput and first-token latency. Every field is null when the
 * window holds no completed generation, which is what an empty install and an idle
 * one both look like — neither is a measured zero. */
export const recentPerformanceStatsSchema = z.object({
  throughput: z.number().nullable(),
  firstTokenP50Ms: z.number().nullable(),
  firstTokenP90Ms: z.number().nullable(),
});

/** Terminal execution counts over the trailing 24 hours. Success rate cannot come
 * from usage_logs, which has no status column. */
export const executionOutcomeStatsSchema = z.object({
  succeeded: z.number(),
  failed: z.number(),
  successRate: z.number(),
});

export const dashboardStatsSchema = z.object({
  totalRequests: z.number(),
  requestStats: requestStatsSchema,
  failedRequests: z.number(),
  averageResponseTime: z.number().nullable(),
  last24HoursPerformance: recentPerformanceStatsSchema,
  last24HoursExecutions: executionOutcomeStatsSchema,
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

export const tokensByChannelSchema = z.object({
  channelId: z.string(),
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

export const tokensByAPIKeySchema = z.object({
  apiKeyId: z.string(),
  apiKeyName: z.string(),
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

export const usageStatsByUserSchema = z.object({
  userId: z.string(),
  userName: z.string(),
  requestCount: z.number(),
  totalTokens: z.number(),
  totalCost: z.number(),
});

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

export type RequestStats = z.infer<typeof requestStatsSchema>;
export type DashboardStats = z.infer<typeof dashboardStatsSchema>;
export type RequestsByChannel = z.infer<typeof requestsByChannelSchema>;
export type RequestsByModel = z.infer<typeof requestsByModelSchema>;
export type RequestsByAPIKey = z.infer<typeof requestsByAPIKeySchema>;
export type TokensByChannel = z.infer<typeof tokensByChannelSchema>;
export type TokensByModel = z.infer<typeof tokensByModelSchema>;
export type TokensByAPIKey = z.infer<typeof tokensByAPIKeySchema>;
export type CostByChannel = z.infer<typeof costByChannelSchema>;
export type CostByModel = z.infer<typeof costByModelSchema>;
export type CostByAPIKey = z.infer<typeof costByAPIKeySchema>;
export type ChannelSuccessRate = z.infer<typeof channelSuccessRateSchema>;
export type ModelPerformanceStat = z.infer<typeof modelPerformanceStatSchema>;
export type ChannelPerformanceStat = z.infer<typeof channelPerformanceStatSchema>;
export type UsageStatsByUser = z.infer<typeof usageStatsByUserSchema>;
export type TokenStats = z.infer<typeof tokenStatsSchema>;

// --- GraphQL queries ---

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
      last24HoursPerformance {
        throughput
        firstTokenP50Ms
        firstTokenP90Ms
      }
      last24HoursExecutions {
        succeeded
        failed
        successRate
      }
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
      channelId
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
  query ModelPerformanceStats($startTime: String, $endTime: String, $timeWindow: String) {
    modelPerformanceStats(startTime: $startTime, endTime: $endTime, timeWindow: $timeWindow) {
      date
      modelId
      throughput
      ttftMs
      requestCount
    }
  }
`;

const CHANNEL_PERFORMANCE_STATS_QUERY = `
  query ChannelPerformanceStats($startTime: String, $endTime: String, $timeWindow: String) {
    channelPerformanceStats(startTime: $startTime, endTime: $endTime, timeWindow: $timeWindow) {
      date
      channelId
      channelName
      throughput
      ttftMs
      requestCount
    }
  }
`;

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

// --- Hooks ---

/** All-time totals plus today/this week/this month request counts. */
export function useDashboardStats() {
  return useQuery({
    queryKey: ['dashboardStats'],
    queryFn: async () => {
      const data = await graphqlRequest<{ dashboardOverview: DashboardStats }>(DASHBOARD_STATS_QUERY);
      return dashboardStatsSchema.parse(data.dashboardOverview);
    },
    refetchInterval: 30000,
  });
}

/** Request counts per channel for a coarse time window. */
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

/** Request counts per model for a coarse time window. */
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

/** Request counts per API key for a coarse time window. */
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

/** Token breakdown per API key for a coarse time window. */
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

/** Token breakdown per channel for a coarse time window. */
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

/** Token breakdown per model for a coarse time window. */
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

/** Cost per channel for a coarse time window. */
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

/** Cost per model for a coarse time window. */
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

/** Cost per API key for a coarse time window. */
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

/** Token totals for today/this week/this month/all time, refreshed every 5 minutes. */
export function useTokenStats() {
  return useQuery({
    queryKey: ['tokenStats'],
    queryFn: async () => {
      const data = await graphqlRequest<{ tokenStats: TokenStats }>(TOKEN_STATS_AGGR_QUERY);
      return tokenStatsSchema.parse(data.tokenStats);
    },
    refetchInterval: 300000,
  });
}

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

/** Throughput and time to first token per model, refreshed every 5 minutes. Bucketing is
 * decided by the server from the range: hourly for short ranges, daily otherwise. */
export function useModelPerformanceStats(startTime?: string | null, endTime?: string | null, timeWindow?: string | null) {
  return useQuery({
    queryKey: ['modelPerformanceStats', startTime, endTime, timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ modelPerformanceStats: ModelPerformanceStat[] }>(MODEL_PERFORMANCE_STATS_QUERY, {
        ...(startTime != null && { startTime }),
        ...(endTime != null && { endTime }),
        ...(timeWindow != null && { timeWindow }),
      });
      return data.modelPerformanceStats.map((item) => modelPerformanceStatSchema.parse(item));
    },
    refetchInterval: 300000,
  });
}

/** Throughput and time to first token per channel, refreshed every 5 minutes. */
export function useChannelPerformanceStats(startTime?: string | null, endTime?: string | null, timeWindow?: string | null) {
  return useQuery({
    queryKey: ['channelPerformanceStats', startTime, endTime, timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ channelPerformanceStats: ChannelPerformanceStat[] }>(CHANNEL_PERFORMANCE_STATS_QUERY, {
        ...(startTime != null && { startTime }),
        ...(endTime != null && { endTime }),
        ...(timeWindow != null && { timeWindow }),
      });
      return data.channelPerformanceStats.map((item) => channelPerformanceStatSchema.parse(item));
    },
    refetchInterval: 300000,
  });
}

/** Token, request and cost totals per user of the selected project. */
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
