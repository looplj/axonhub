import { z } from 'zod';

// Time window filter
export const StatisticsTimeWindowSchema = z.enum(['day', 'week', 'month']);
export type StatisticsTimeWindow = z.infer<typeof StatisticsTimeWindowSchema>;

// Channel Statistics
export const ChannelStatisticsSchema = z.object({
  channelId: z.string(),
  channelName: z.string(),
  channelType: z.string(),
  requestCount: z.number(),
  promptTokens: z.number(),
  completionTokens: z.number(),
  cachedTokens: z.number(),
  avgTtftMs: z.number().nullable(),
  avgLatencyMs: z.number().nullable(),
  avgTps: z.number().nullable(),
  totalCost: z.number(),
});

export type ChannelStatistics = z.infer<typeof ChannelStatisticsSchema>;

// Model Statistics
export const ModelStatisticsSchema = z.object({
  modelId: z.string(),
  channelId: z.string(),
  channelName: z.string(),
  requestCount: z.number(),
  promptTokens: z.number(),
  completionTokens: z.number(),
  cachedTokens: z.number(),
  avgTtftMs: z.number().nullable(),
  avgLatencyMs: z.number().nullable(),
  avgTps: z.number().nullable(),
  totalCost: z.number(),
});

export type ModelStatistics = z.infer<typeof ModelStatisticsSchema>;

// Channel Time Series (for charts)
export const ChannelStatisticsTimeSeriesSchema = z.object({
  date: z.string(),
  requestCount: z.number(),
  promptTokens: z.number(),
  completionTokens: z.number(),
  avgLatencyMs: z.number().nullable(),
});

export type ChannelStatisticsTimeSeries = z.infer<typeof ChannelStatisticsTimeSeriesSchema>;
