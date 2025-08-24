import { z } from 'zod'

// Channel Types
export const channelTypeSchema = z.enum([
  'openai',
  'anthropic',
  'anthropic_aws',
  'anthropic_gcp',
  'gemini',
  'deepseek',
  'doubao',
  'kimi',
  'anthropic_fake',
  'openai_fake'
])
export type ChannelType = z.infer<typeof channelTypeSchema>

// Channel Status
export const channelStatusSchema = z.enum(['enabled', 'disabled', 'archived'])
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
  settings: channelSettingsSchema.optional().nullable(),

})
export type Channel = z.infer<typeof channelSchema>

// Create Channel Input
export const createChannelInputSchema = z.object({
  type: channelTypeSchema,
  baseURL: z.string().url('请输入有效的 URL'),
  name: z.string().min(1, '名称不能为空'),
  supportedModels: z.array(z.string()).min(0, '至少选择一个支持的模型'),
  defaultTestModel: z.string().min(1, '请选择默认测试模型'),
  settings: channelSettingsSchema.optional(),
  credentials: z.object({
    apiKey: z.string().min(1, 'API Key 不能为空'),
    aws: z.object({
      accessKeyID: z.string().optional(),
      secretAccessKey: z.string().optional(),
      region: z.string().optional(),
    }).optional(),
    gcp: z.object({
      region: z.string().optional(),
      projectID: z.string().optional(),
      jsonData: z.string().optional(),
    }).optional(),
  }),
}).superRefine((data, ctx) => {
  // 如果是 anthropic_aws 类型，AWS 字段必填（精确到字段级报错）
  if (data.type === 'anthropic_aws') {
    const aws = data.credentials?.aws
    if (!aws?.accessKeyID) {
      ctx.addIssue({ code: 'custom', message: 'AWS Access Key ID 不能为空', path: ['credentials', 'aws', 'accessKeyID'] })
    }
    if (!aws?.secretAccessKey) {
      ctx.addIssue({ code: 'custom', message: 'AWS Secret Access Key 不能为空', path: ['credentials', 'aws', 'secretAccessKey'] })
    }
    if (!aws?.region) {
      ctx.addIssue({ code: 'custom', message: 'AWS Region 不能为空', path: ['credentials', 'aws', 'region'] })
    }
  }
  // 如果是 anthropic_gcp 类型，GCP 字段必填（精确到字段级报错）
  if (data.type === 'anthropic_gcp') {
    const gcp = data.credentials?.gcp
    if (!gcp?.region) {
      ctx.addIssue({ code: 'custom', message: 'GCP Region 不能为空', path: ['credentials', 'gcp', 'region'] })
    }
    if (!gcp?.projectID) {
      ctx.addIssue({ code: 'custom', message: 'GCP Project ID 不能为空', path: ['credentials', 'gcp', 'projectID'] })
    }
    if (!gcp?.jsonData) {
      ctx.addIssue({ code: 'custom', message: 'GCP Service Account JSON 不能为空', path: ['credentials', 'gcp', 'jsonData'] })
    }
  }
})
export type CreateChannelInput = z.infer<typeof createChannelInputSchema>

// Update Channel Input
export const updateChannelInputSchema = z.object({
  type: channelTypeSchema.optional(),
  baseURL: z.string().url('请输入有效的 URL').optional(),
  name: z.string().min(1, '名称不能为空').optional(),
  supportedModels: z.array(z.string()).min(1, '至少选择一个支持的模型').optional(),
  defaultTestModel: z.string().min(1, '请选择默认测试模型').optional(),
  settings: channelSettingsSchema.optional(),
  credentials: z.object({
    apiKey: z.string().optional(),
    aws: z.object({
      accessKeyID: z.string().optional(),
      secretAccessKey: z.string().optional(),
      region: z.string().optional(),
    }).optional(),
    gcp: z.object({
      region: z.string().optional(),
      projectID: z.string().optional(),
      jsonData: z.string().optional(),
    }).optional(),
  }).optional(),
}).superRefine((data, ctx) => {
  // 如果是 anthropic_aws 类型且提供了 credentials，AWS 字段必填（字段级报错）
  if (data.type === 'anthropic_aws' && data.credentials) {
    const aws = data.credentials.aws
    if (!aws?.accessKeyID) {
      ctx.addIssue({ code: 'custom', message: 'AWS Access Key ID 不能为空', path: ['credentials', 'aws', 'accessKeyID'] })
    }
    if (!aws?.secretAccessKey) {
      ctx.addIssue({ code: 'custom', message: 'AWS Secret Access Key 不能为空', path: ['credentials', 'aws', 'secretAccessKey'] })
    }
    if (!aws?.region) {
      ctx.addIssue({ code: 'custom', message: 'AWS Region 不能为空', path: ['credentials', 'aws', 'region'] })
    }
  }
  // 如果是 anthropic_gcp 类型且提供了 credentials，GCP 字段必填（字段级报错）
  if (data.type === 'anthropic_gcp' && data.credentials) {
    const gcp = data.credentials.gcp
    if (!gcp?.region) {
      ctx.addIssue({ code: 'custom', message: 'GCP Region 不能为空', path: ['credentials', 'gcp', 'region'] })
    }
    if (!gcp?.projectID) {
      ctx.addIssue({ code: 'custom', message: 'GCP Project ID 不能为空', path: ['credentials', 'gcp', 'projectID'] })
    }
    if (!gcp?.jsonData) {
      ctx.addIssue({ code: 'custom', message: 'GCP Service Account JSON 不能为空', path: ['credentials', 'gcp', 'jsonData'] })
    }
  }
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

// Bulk Import Schemas
export const bulkImportChannelItemSchema = z.object({
  type: channelTypeSchema,
  name: z.string().min(1, '名称不能为空'),
  baseURL: z.string().url('请输入有效的 URL').min(1, 'Base URL 不能为空'),
  apiKey: z.string().min(1, 'API Key 不能为空'),
  supportedModels: z.array(z.string()).min(1, '至少选择一个支持的模型'),
  defaultTestModel: z.string().min(1, '请选择默认测试模型'),
})
export type BulkImportChannelItem = z.infer<typeof bulkImportChannelItemSchema>

export const bulkImportChannelsInputSchema = z.object({
  channels: z.array(bulkImportChannelItemSchema).min(1, '至少需要一个 channel'),
})
export type BulkImportChannelsInput = z.infer<typeof bulkImportChannelsInputSchema>

export const bulkImportChannelsResultSchema = z.object({
  success: z.boolean(),
  created: z.number(),
  failed: z.number(),
  errors: z.array(z.string()).optional().nullable(),
  channels: z.array(channelSchema).nullable(),
})
export type BulkImportChannelsResult = z.infer<typeof bulkImportChannelsResultSchema>

// Raw text input for bulk import
export const bulkImportTextSchema = z.object({
  text: z.string().min(1, '请输入要导入的数据'),
})
export type BulkImportText = z.infer<typeof bulkImportTextSchema>