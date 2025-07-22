import { z } from 'zod'
import { userSchema } from '@/features/users/data/schema'

// API Key schema based on GraphQL schema
export const apiKeySchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  user: userSchema.partial(),
  key: z.string(),
  name: z.string(),
})
export type ApiKey = z.infer<typeof apiKeySchema>

// API Key Connection schema for GraphQL pagination
export const apiKeyEdgeSchema = z.object({
  node: apiKeySchema,
  cursor: z.string(),
})

export const pageInfoSchema = z.object({
  hasNextPage: z.boolean(),
  hasPreviousPage: z.boolean(),
  startCursor: z.string().nullable(),
  endCursor: z.string().nullable(),
})

export const apiKeyConnectionSchema = z.object({
  edges: z.array(apiKeyEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
})
export type ApiKeyConnection = z.infer<typeof apiKeyConnectionSchema>

// Create API Key Input
export const createApiKeyInputSchema = z.object({
  name: z.string().min(1, '名称不能为空'),
  userID: z.string().min(1, '用户ID不能为空'),
  key: z.string().min(1, 'API Key不能为空'),
})
export type CreateApiKeyInput = z.infer<typeof createApiKeyInputSchema>

// Update API Key Input
export const updateApiKeyInputSchema = z.object({
  name: z.string().min(1, '名称不能为空').optional(),
})
export type UpdateApiKeyInput = z.infer<typeof updateApiKeyInputSchema>
