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
  successCount: z.number(),
  failedCount: z.number(),
  totalCount: z.number(),
  successRate: z.number(),
});

export type RequestStats = z.infer<typeof requestStatsSchema>;
export type DashboardStats = z.infer<typeof dashboardStatsSchema>;
export type RequestsByChannel = z.infer<typeof requestsByChannelSchema>;
export type RequestsByModel = z.infer<typeof requestsByModelSchema>;
export type RequestsByAPIKey = z.infer<typeof requestsByAPIKeySchema>;
export type TokensByAPIKey = z.infer<typeof tokensByAPIKeySchema>;
export type DailyRequestStats = z.infer<typeof dailyRequestStatsSchema>;
export type HourlyRequestStats = z.infer<typeof hourlyRequestStatsSchema>;
export type TopProjects = z.infer<typeof topProjectsSchema>;
export type ChannelSuccessRate = z.infer<typeof channelSuccessRateSchema>;

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
  query GetRequestsByChannel {
    requestStatsByChannel {
      channelName
      count
    }
  }
`;

const REQUESTS_BY_MODEL_QUERY = `
  query GetRequestsByModel {
    requestStatsByModel {
      modelId
      count
    }
  }
`;

const REQUESTS_BY_API_KEY_QUERY = `
  query GetRequestsByAPIKey {
    requestStatsByAPIKey {
      apiKeyId
      apiKeyName
      count
    }
  }
`;

const TOKENS_BY_API_KEY_QUERY = `
  query GetTokensByAPIKey {
    tokenStatsByAPIKey {
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
  query GetChannelSuccessRates {
    channelSuccessRates {
      channelId
      channelName
      channelType
      successCount
      failedCount
      totalCount
      successRate
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

export function useRequestsByChannel() {
  return useQuery({
    queryKey: ['requestStatsByChannel'],
    queryFn: async () => {
      const data = await graphqlRequest<{ requestStatsByChannel: RequestsByChannel[] }>(REQUESTS_BY_CHANNEL_QUERY);
      return data.requestStatsByChannel.map((item) => requestsByChannelSchema.parse(item));
    },
    refetchInterval: 60000,
  });
}

export function useRequestsByModel() {
  return useQuery({
    queryKey: ['requestStatsByModel'],
    queryFn: async () => {
      const data = await graphqlRequest<{ requestStatsByModel: RequestsByModel[] }>(REQUESTS_BY_MODEL_QUERY);
      return data.requestStatsByModel.map((item) => requestsByModelSchema.parse(item));
    },
    refetchInterval: 60000,
  });
}

export function useRequestsByAPIKey() {
  return useQuery({
    queryKey: ['requestStatsByAPIKey'],
    queryFn: async () => {
      const data = await graphqlRequest<{ requestStatsByAPIKey: RequestsByAPIKey[] }>(REQUESTS_BY_API_KEY_QUERY);
      return data.requestStatsByAPIKey.map((item) => requestsByAPIKeySchema.parse(item));
    },
    refetchInterval: 60000,
  });
}

export function useTokensByAPIKey() {
  return useQuery({
    queryKey: ['tokenStatsByAPIKey'],
    queryFn: async () => {
      const data = await graphqlRequest<{ tokenStatsByAPIKey: TokensByAPIKey[] }>(TOKENS_BY_API_KEY_QUERY);
      return data.tokenStatsByAPIKey.map((item) => tokensByAPIKeySchema.parse(item));
    },
    refetchInterval: 60000, // Auto-refresh every 60 seconds
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

export function useChannelSuccessRates() {
  return useQuery({
    queryKey: ['channelSuccessRates'],
    queryFn: async () => {
      const data = await graphqlRequest<{ channelSuccessRates: ChannelSuccessRate[] }>(CHANNEL_SUCCESS_RATES_QUERY);
      return data.channelSuccessRates.map((item) => channelSuccessRateSchema.parse(item));
    },
    refetchInterval: 300000,
  });
}
