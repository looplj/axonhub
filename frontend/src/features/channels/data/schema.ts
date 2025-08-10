import { z } from 'zod'

// Channel Types
export const channelTypeSchema = z.enum([
  'openai',
  'anthropic', 
  'gemini',
  'deepseek',
  'doubao',
  'kimi'
])
export type ChannelType = z.infer<typeof channelTypeSchema>

// Channel Status
export const channelStatusSchema = z.enum(['enabled', 'disabled'])
export type ChannelStatus = z.infer<typeof channelStatusSchema>

// Model Mapping
export const modelMappingSchema = z.object({
  from: z.string(),
  to: z.string(),
})
export type ModelMapping = z.infer<typeof modelMappingSchema>

// Channel Settings
export const channelSettingsSchema = z.object({
  modelMappings: z.array(modelMappingSchema),
})
export type ChannelSettings = z.infer<typeof channelSettingsSchema>

// Channel
export const channelSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  type: channelTypeSchema,
  baseURL: z.string(),
  name: z.string(),
  status: channelStatusSchema,
  supportedModels: z.array(z.string()),
  defaultTestModel: z.string(),
  settings: channelSettingsSchema,
})
export type Channel = z.infer<typeof channelSchema>

// Create Channel Input
export const createChannelInputSchema = z.object({
  type: channelTypeSchema,
  baseURL: z.string().url('请输入有效的 URL'),
  name: z.string().min(1, '名称不能为空'),
  apiKey: z.string().min(1, 'API Key 不能为空'),
  supportedModels: z.array(z.string()).min(1, '至少选择一个支持的模型'),
  defaultTestModel: z.string().min(1, '请选择默认测试模型'),
  settings: channelSettingsSchema.optional(),
})
export type CreateChannelInput = z.infer<typeof createChannelInputSchema>

// Update Channel Input
export const updateChannelInputSchema = z.object({
  baseURL: z.string().url('请输入有效的 URL').optional(),
  name: z.string().min(1, '名称不能为空').optional(),
  apiKey: z.string().min(1, 'API Key 不能为空').optional(),
  supportedModels: z.array(z.string()).min(1, '至少选择一个支持的模型').optional(),
  defaultTestModel: z.string().min(1, '请选择默认测试模型').optional(),
  settings: channelSettingsSchema.optional(),
})
export type UpdateChannelInput = z.infer<typeof updateChannelInputSchema>

// Channel Connection (for pagination)
export const channelConnectionSchema = z.object({
  edges: z.array(z.object({
    node: channelSchema,
    cursor: z.string(),
  })),
  pageInfo: z.object({
    hasNextPage: z.boolean(),
    hasPreviousPage: z.boolean(),
    startCursor: z.string().nullable(),
    endCursor: z.string().nullable(),
  }),
  totalCount: z.number(),
})
export type ChannelConnection = z.infer<typeof channelConnectionSchema>