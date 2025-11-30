import { useQuery } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { z } from 'zod'

// Schema definitions
export const requestStatsSchema = z.object({
  requestsToday: z.number(),
  requestsThisWeek: z.number(),
  requestsThisMonth: z.number(),
})

export const dashboardStatsSchema = z.object({
  totalUsers: z.number(),
  totalRequests: z.number(),
  requestStats: requestStatsSchema,
  failedRequests: z.number(),
  averageResponseTime: z.number().nullable(),
})

export const requestsByChannelSchema = z.object({
  channelName: z.string(),
  channelType: z.string(),
  count: z.number(),
})

export const requestsByModelSchema = z.object({
  modelId: z.string(),
  count: z.number(),
})

export const dailyRequestStatsSchema = z.object({
  date: z.string(),
  count: z.number(),
})

export const hourlyRequestStatsSchema = z.object({
  hour: z.number(),
  count: z.number(),
})

export const topProjectsSchema = z.object({
  projectId: z.string(),
  projectName: z.string(),
  projectDescription: z.string(),
  requestCount: z.number(),
})

export type RequestStats = z.infer<typeof requestStatsSchema>
export type DashboardStats = z.infer<typeof dashboardStatsSchema>
export type RequestsByChannel = z.infer<typeof requestsByChannelSchema>
export type RequestsByModel = z.infer<typeof requestsByModelSchema>
export type DailyRequestStats = z.infer<typeof dailyRequestStatsSchema>
export type HourlyRequestStats = z.infer<typeof hourlyRequestStatsSchema>
export type TopProjects = z.infer<typeof topProjectsSchema>

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
})

export const modelTokenStatsSchema = z.object({
  modelId: z.string(),
  modelName: z.string(),
  totalInputTokens: z.number(),
  totalOutputTokens: z.number(),
  totalCachedTokens: z.number(),
  totalTokens: z.number(),
  period: z.string(),
  date: z.string(),
})

export const modelTokenTrendSchema = z.object({
  modelId: z.string(),
  modelName: z.string(),
  date: z.string(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  cachedTokens: z.number(),
  totalTokens: z.number(),
})

export const modelTokenTrendDataSchema = z.object({
  trends: z.array(modelTokenTrendSchema),
  models: z.array(z.string()),
  dates: z.array(z.string()),
})

export const modelTokenStatsSummarySchema = z.object({
  currentPeriod: z.array(modelTokenStatsSchema),
  trends: modelTokenTrendDataSchema,
})

export type TokenStats = z.infer<typeof tokenStatsSchema>
export type ModelTokenStats = z.infer<typeof modelTokenStatsSchema>
export type ModelTokenTrend = z.infer<typeof modelTokenTrendSchema>
export type ModelTokenTrendData = z.infer<typeof modelTokenTrendDataSchema>
export type ModelTokenStatsSummary = z.infer<typeof modelTokenStatsSummarySchema>

// GraphQL queries
const DASHBOARD_STATS_QUERY = `
  query GetDashboardStats {
    dashboardOverview {
      totalUsers
      totalRequests
      requestStats {
        requestsToday
        requestsThisWeek
        requestsThisMonth
      }
      failedRequests
      averageResponseTime
    }
  }
`

const REQUESTS_BY_CHANNEL_QUERY = `
  query GetRequestsByChannel {
    requestStatsByChannel {
      channelName
      channelType
      count
    }
  }
`

const REQUESTS_BY_MODEL_QUERY = `
  query GetRequestsByModel {
    requestStatsByModel {
      modelId
      count
    }
  }
`

const DAILY_REQUEST_STATS_QUERY = `
  query GetDailyRequestStats($days: Int) {
    dailyRequestStats(days: $days) {
      date
      count
    }
  }
`

const HOURLY_REQUEST_STATS_QUERY = `
  query GetHourlyRequestStats($date: String) {
    hourlyRequestStats(date: $date) {
      hour
      count
    }
  }
`

const TOP_PROJECTS_QUERY = `
  query GetTopProjects($limit: Int) {
    topRequestsProjects(limit: $limit) {
      projectId
      projectName
      projectDescription
      requestCount
    }
  }
`

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
`

// Model-specific token statistics
const MODEL_TOKEN_STATS_QUERY = `
  query GetModelTokenStats($models: [String!], $period: String, $date: String) {
    modelTokenStats(models: $models, period: $period, date: $date) {
      currentPeriod {
        modelId
        modelName
        totalInputTokens
        totalOutputTokens
        totalCachedTokens
        totalTokens
        period
        date
      }
      trends {
        trends {
          modelId
          modelName
          date
          inputTokens
          outputTokens
          cachedTokens
          totalTokens
        }
        models
        dates
      }
    }
  }
`

// Query hooks
export function useDashboardStats() {
  return useQuery({
    queryKey: ['dashboardStats'],
    queryFn: async () => {
      const data = await graphqlRequest<{ dashboardOverview: DashboardStats }>(
        DASHBOARD_STATS_QUERY
      )
      return dashboardStatsSchema.parse(data.dashboardOverview)
    },
    refetchInterval: 30000, // Refetch every 30 seconds
  })
}

export function useRequestsByChannel() {
  return useQuery({
    queryKey: ['requestStatsByChannel'],
    queryFn: async () => {
      const data = await graphqlRequest<{ requestStatsByChannel: RequestsByChannel[] }>(
        REQUESTS_BY_CHANNEL_QUERY
      )
      return data.requestStatsByChannel.map(item => requestsByChannelSchema.parse(item))
    },
    refetchInterval: 60000,
  })
}

export function useRequestsByModel() {
  return useQuery({
    queryKey: ['requestStatsByModel'],
    queryFn: async () => {
      const data = await graphqlRequest<{ requestStatsByModel: RequestsByModel[] }>(
        REQUESTS_BY_MODEL_QUERY
      )
      return data.requestStatsByModel.map(item => requestsByModelSchema.parse(item))
    },
    refetchInterval: 60000,
  })
}

export function useDailyRequestStats(days?: number) {
  return useQuery({
    queryKey: ['dailyRequestStats', days],
    queryFn: async () => {
      const data = await graphqlRequest<{ dailyRequestStats: DailyRequestStats[] }>(
        DAILY_REQUEST_STATS_QUERY,
        { days }
      )
      return data.dailyRequestStats.map(item => dailyRequestStatsSchema.parse(item))
    },
    refetchInterval: 300000, // Refetch every 5 minutes
  })
}

export function useHourlyRequestStats(date?: string) {
  return useQuery({
    queryKey: ['hourlyRequestStats', date],
    queryFn: async () => {
      const data = await graphqlRequest<{ hourlyRequestStats: HourlyRequestStats[] }>(
        HOURLY_REQUEST_STATS_QUERY,
        { date }
      )
      return data.hourlyRequestStats.map(item => hourlyRequestStatsSchema.parse(item))
    },
    refetchInterval: 300000,
  })
}

export function useTopProjects(limit?: number) {
  return useQuery({
    queryKey: ['topRequestsProjects', limit],
    queryFn: async () => {
      const data = await graphqlRequest<{ topRequestsProjects: TopProjects[] }>(
        TOP_PROJECTS_QUERY,
        { limit }
      )
      return data.topRequestsProjects.map(item => topProjectsSchema.parse(item))
    },
    refetchInterval: 300000,
  })
}

export function useTokenStats() {
  return useQuery({
    queryKey: ['tokenStats'],
    queryFn: async () => {
      const data = await graphqlRequest<{ tokenStats: TokenStats }>(TOKEN_STATS_AGGR_QUERY)
      return tokenStatsSchema.parse(data.tokenStats)
    },
    refetchInterval: 300000, // Refetch every 5 minutes
  })
}

export function useModelTokenStats(models?: string[], period?: string, date?: string) {
  return useQuery({
    queryKey: ['modelTokenStats', models, period, date],
    queryFn: async () => {
      const data = await graphqlRequest<{ modelTokenStats: ModelTokenStatsSummary }>(
        MODEL_TOKEN_STATS_QUERY,
        { models, period, date }
      )
      return modelTokenStatsSummarySchema.parse(data.modelTokenStats)
    },
    refetchInterval: 300000, // Refetch every 5 minutes
    enabled: !!models && models.length > 0, // Only fetch when models are specified
  })
}