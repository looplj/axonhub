import { useQuery } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { z } from 'zod'

// Schema definitions
export const dashboardStatsSchema = z.object({
  totalUsers: z.number(),
  totalRequests: z.number(),
  requestsToday: z.number(),
  requestsThisWeek: z.number(),
  requestsThisMonth: z.number(),
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

export const topUsersSchema = z.object({
  userId: z.string(),
  userName: z.string(),
  userEmail: z.string(),
  requestCount: z.number(),
})

export type DashboardStats = z.infer<typeof dashboardStatsSchema>
export type RequestsByChannel = z.infer<typeof requestsByChannelSchema>
export type RequestsByModel = z.infer<typeof requestsByModelSchema>
export type DailyRequestStats = z.infer<typeof dailyRequestStatsSchema>
export type HourlyRequestStats = z.infer<typeof hourlyRequestStatsSchema>
export type TopUsers = z.infer<typeof topUsersSchema>

// GraphQL queries
const DASHBOARD_STATS_QUERY = `
  query GetDashboardStats {
    dashboardOverview {
      totalUsers
      totalRequests
      requestsToday
      requestsThisWeek
      requestsThisMonth
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

const TOP_USERS_QUERY = `
  query GetTopUsers($limit: Int) {
    topRequestsUsers(limit: $limit) {
      userId
      userName
      userEmail
      requestCount
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

export function useTopUsers(limit?: number) {
  return useQuery({
    queryKey: ['topRequestsUsers', limit],
    queryFn: async () => {
      const data = await graphqlRequest<{ topRequestsUsers: TopUsers[] }>(
        TOP_USERS_QUERY,
        { limit }
      )
      return data.topRequestsUsers.map(item => topUsersSchema.parse(item))
    },
    refetchInterval: 300000,
  })
}