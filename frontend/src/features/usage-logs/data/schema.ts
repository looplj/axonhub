import { z } from 'zod'
import { userSchema } from '@/features/users/data/schema'
import { channelSchema } from '@/features/channels/data'

// Usage Log Source
export const usageLogSourceSchema = z.enum([
  'api',
  'playground',
  'test',
])
export type UsageLogSource = z.infer<typeof usageLogSourceSchema>

// Usage Log schema based on backend entity structure
export const usageLogSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  user: userSchema.partial().optional(),
  requestID: z.string(),
  channel: channelSchema.partial().nullable().optional(),
  modelID: z.string(),
  promptTokens: z.number(),
  completionTokens: z.number(),
  totalTokens: z.number(),
  promptAudioTokens: z.number().nullable().optional(),
  promptCachedTokens: z.number().nullable().optional(),
  completionAudioTokens: z.number().nullable().optional(),
  completionReasoningTokens: z.number().nullable().optional(),
  completionAcceptedPredictionTokens: z.number().nullable().optional(),
  completionRejectedPredictionTokens: z.number().nullable().optional(),
  source: usageLogSourceSchema,
  format: z.string(),
})
export type UsageLog = z.infer<typeof usageLogSchema>

// Usage Log Connection schema for GraphQL pagination
export const usageLogEdgeSchema = z.object({
  node: usageLogSchema,
  cursor: z.string(),
})

export const pageInfoSchema = z.object({
  hasNextPage: z.boolean(),
  hasPreviousPage: z.boolean(),
  startCursor: z.string().nullable(),
  endCursor: z.string().nullable(),
})

export const usageLogConnectionSchema = z.object({
  edges: z.array(usageLogEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
})
export type UsageLogConnection = z.infer<typeof usageLogConnectionSchema>

// Usage Log Edge type
export type UsageLogEdge = z.infer<typeof usageLogEdgeSchema>

// Page Info type
export type PageInfo = z.infer<typeof pageInfoSchema>

// Usage Log List schema for table display
export const usageLogListSchema = z.array(usageLogSchema)
export type UsageLogList = z.infer<typeof usageLogListSchema>