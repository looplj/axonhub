import { z } from 'zod'
import { userSchema } from '@/features/users/data/schema'

// Validation message functions for i18n
const getValidationMessages = () => ({
  nameRequired: '名称不能为空',
  userIdRequired: '用户ID不能为空', 
  keyRequired: 'API Key不能为空',
})

// API Key Status
export const apiKeyStatusSchema = z.enum(['enabled', 'disabled'])
export type ApiKeyStatus = z.infer<typeof apiKeyStatusSchema>

// API Key schema based on GraphQL schema
export const apiKeySchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  user: userSchema.partial().optional(),
  key: z.string(),
  name: z.string(),
  status: apiKeyStatusSchema,
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

// Create API Key Input - factory function for i18n support
export const createApiKeyInputSchemaFactory = (t: (key: string) => string) => z.object({
  name: z.string().min(1, t('apikeys.validation.nameRequired')),
  userID: z.string().min(1, t('apikeys.validation.userIdRequired')),
  key: z.string().min(1, t('apikeys.validation.keyRequired')),
})

// Default schema for backward compatibility
export const createApiKeyInputSchema = z.object({
  name: z.string().min(1, '名称不能为空'),
})
export type CreateApiKeyInput = z.infer<typeof createApiKeyInputSchema>

// Update API Key Input - factory function for i18n support
export const updateApiKeyInputSchemaFactory = (t: (key: string) => string) => z.object({
  name: z.string().min(1, t('apikeys.validation.nameRequired')).optional(),
})

// Default schema for backward compatibility
export const updateApiKeyInputSchema = z.object({
  name: z.string().min(1, '名称不能为空').optional(),
})
export type UpdateApiKeyInput = z.infer<typeof updateApiKeyInputSchema>
