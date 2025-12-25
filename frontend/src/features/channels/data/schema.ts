import { z } from 'zod'

const apiFormatSchema = z.enum(['openai/chat_completions', 'openai/responses', 'anthropic/messages', 'gemini/contents'])

export type ApiFormat = z.infer<typeof apiFormatSchema>

// Channel Types
export const channelTypeSchema = z.enum([
  'openai',
  'openai_responses',
  'anthropic',
  'anthropic_aws',
  'anthropic_gcp',
  'gemini_openai',
  'gemini',
  'gemini_vertex',
  'deepseek',
  'doubao',
  'doubao_anthropic',
  'moonshot',
  'zhipu',
  'zai',
  'vercel',
  'anthropic_fake',
  'openai_fake',
  'deepseek_anthropic',
  'moonshot_anthropic',
  'zhipu_anthropic',
  'zai_anthropic',
  'openrouter',
  'xai',
  'ppio',
  'siliconflow',
  'volcengine',
  'longcat',
  'longcat_anthropic',
  'minimax',
  'minimax_anthropic',
  'aihubmix',
  'burncloud',
  'modelscope',
  'bailian',
  'jina',
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

// Header Entry
export const headerEntrySchema = z.object({
  key: z.string().min(1, 'Header key is required'),
  value: z.string(),
})
export type HeaderEntry = z.infer<typeof headerEntrySchema>

// Proxy Type
export const proxyTypeSchema = z.enum(['disabled', 'environment', 'url'])
export type ProxyType = z.infer<typeof proxyTypeSchema>

// Proxy Config
export const proxyConfigSchema = z.object({
  type: proxyTypeSchema,
  url: z.string().optional(),
  username: z.string().optional(),
  password: z.string().optional(),
})
export type ProxyConfig = z.infer<typeof proxyConfigSchema>

// Channel Performance
export const channelPerformanceSchema = z.object({
  avgLatencyMs: z.number(),
  avgTokenPerSecond: z.number(),
  avgStreamFirstTokenLatencyMs: z.number(),
  avgStreamTokenPerSecond: z.number(),
})
export type ChannelPerformance = z.infer<typeof channelPerformanceSchema>

// Channel Settings
export const channelSettingsSchema = z.object({
  extraModelPrefix: z.string().optional(),
  modelMappings: z.array(modelMappingSchema).nullable(),
  autoTrimedModelPrefixes: z.array(z.string()).optional().nullable(),
  overrideParameters: z.string().optional(),
  overrideHeaders: z.array(headerEntrySchema).optional().nullable(),
  proxy: proxyConfigSchema.optional().nullable(),
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
  autoSyncSupportedModels: z.boolean().default(false),
  tags: z.array(z.string()).optional().default([]).nullable(),
  defaultTestModel: z.string(),
  settings: channelSettingsSchema.optional().nullable(),
  orderingWeight: z.number().default(0),
  errorMessage: z.string().optional().nullable(),
  remark: z.string().optional().nullable(),
  channelPerformance: channelPerformanceSchema.optional().nullable(),
})
export type Channel = z.infer<typeof channelSchema>

// Create Channel Input
export const createChannelInputSchema = z
  .object({
    type: channelTypeSchema,
    baseURL: z.string().url('Please enter a valid URL'),
    name: z.string().min(1, 'Name is required'),
    supportedModels: z.array(z.string()).min(0, 'At least one supported model is required'),
    autoSyncSupportedModels: z.boolean().optional().default(false),
    tags: z.array(z.string()).optional().default([]),
    defaultTestModel: z.string().min(1, 'Please select a default test model'),
    settings: channelSettingsSchema.optional(),
    credentials: z.object({
      apiKey: z.string().min(1, 'API Key is required'),
      aws: z
        .object({
          accessKeyID: z.string().optional(),
          secretAccessKey: z.string().optional(),
          region: z.string().optional(),
        })
        .optional(),
      gcp: z
        .object({
          region: z.string().optional(),
          projectID: z.string().optional(),
          jsonData: z.string().optional(),
        })
        .optional(),
    }),
  })
  .superRefine((data, ctx) => {
    // 如果是 anthropic_aws 类型，AWS 字段必填（精确到字段级报错）
    if (data.type === 'anthropic_aws') {
      const aws = data.credentials?.aws
      if (!aws?.accessKeyID) {
        ctx.addIssue({
          code: 'custom',
          message: 'AWS Access Key ID is required',
          path: ['credentials', 'aws', 'accessKeyID'],
        })
      }
      if (!aws?.secretAccessKey) {
        ctx.addIssue({
          code: 'custom',
          message: 'AWS Secret Access Key is required',
          path: ['credentials', 'aws', 'secretAccessKey'],
        })
      }
      if (!aws?.region) {
        ctx.addIssue({
          code: 'custom',
          message: 'AWS Region is required',
          path: ['credentials', 'aws', 'region'],
        })
      }
    }
    // 如果是 anthropic_gcp 类型，GCP 字段必填（精确到字段级报错）
    if (data.type === 'anthropic_gcp') {
      const gcp = data.credentials?.gcp
      if (!gcp?.region) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Region is required',
          path: ['credentials', 'gcp', 'region'],
        })
      }
      if (!gcp?.projectID) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Project ID is required',
          path: ['credentials', 'gcp', 'projectID'],
        })
      }
      if (!gcp?.jsonData) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Service Account JSON is required',
          path: ['credentials', 'gcp', 'jsonData'],
        })
      }
    }
  })
export type CreateChannelInput = z.infer<typeof createChannelInputSchema>

// Update Channel Input
export const updateChannelInputSchema = z
  .object({
    type: channelTypeSchema.optional(),
    baseURL: z.string().url('Please enter a valid URL').optional(),
    name: z.string().min(1, 'Name is required').optional(),
    supportedModels: z.array(z.string()).min(1, 'At least one supported model is required').optional(),
    autoSyncSupportedModels: z.boolean().optional(),
    tags: z.array(z.string()).optional(),
    defaultTestModel: z.string().min(1, 'Please select a default test model').optional(),
    settings: channelSettingsSchema.optional(),
    errorMessage: z.string().optional().nullable(),
    remark: z.string().optional().nullable(),
    credentials: z
      .object({
        apiKey: z.string().optional(),
        aws: z
          .object({
            accessKeyID: z.string().optional(),
            secretAccessKey: z.string().optional(),
            region: z.string().optional(),
          })
          .optional(),
        gcp: z
          .object({
            region: z.string().optional(),
            projectID: z.string().optional(),
            jsonData: z.string().optional(),
          })
          .optional(),
      })
      .optional(),
    orderingWeight: z.number().optional(),
  })
  .superRefine((data, ctx) => {
    // 如果是 anthropic_aws 类型且提供了 credentials，AWS 字段必填（字段级报错）
    if (data.type === 'anthropic_aws' && data.credentials) {
      const aws = data.credentials.aws
      if (!aws?.accessKeyID) {
        ctx.addIssue({
          code: 'custom',
          message: 'AWS Access Key ID is required',
          path: ['credentials', 'aws', 'accessKeyID'],
        })
      }
      if (!aws?.secretAccessKey) {
        ctx.addIssue({
          code: 'custom',
          message: 'AWS Secret Access Key is required',
          path: ['credentials', 'aws', 'secretAccessKey'],
        })
      }
      if (!aws?.region) {
        ctx.addIssue({
          code: 'custom',
          message: 'AWS Region is required',
          path: ['credentials', 'aws', 'region'],
        })
      }
    }
    // 如果是 anthropic_gcp 类型且提供了 credentials，GCP 字段必填（字段级报错）
    if (data.type === 'anthropic_gcp' && data.credentials) {
      const gcp = data.credentials.gcp
      if (!gcp?.region) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Region is required',
          path: ['credentials', 'gcp', 'region'],
        })
      }
      if (!gcp?.projectID) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Project ID is required',
          path: ['credentials', 'gcp', 'projectID'],
        })
      }
      if (!gcp?.jsonData) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Service Account JSON is required',
          path: ['credentials', 'gcp', 'jsonData'],
        })
      }
    }
  })
export type UpdateChannelInput = z.infer<typeof updateChannelInputSchema>

// Channel Connection (for pagination)
export const channelConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: channelSchema,
      cursor: z.string(),
    })
  ),
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
  name: z.string().min(1, 'Name is required'),
  baseURL: z.string().url('Please enter a valid URL').min(1, 'Base URL is required'),
  apiKey: z.string().min(1, 'API Key is required'),
  supportedModels: z.array(z.string()).min(1, 'At least one supported model is required'),
  defaultTestModel: z.string().min(1, 'Please select a default test model'),
})
export type BulkImportChannelItem = z.infer<typeof bulkImportChannelItemSchema>

export const bulkImportChannelsInputSchema = z.object({
  channels: z.array(bulkImportChannelItemSchema).min(1, 'At least one channel is required'),
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
  text: z.string().min(1, 'Please enter data to import'),
})
export type BulkImportText = z.infer<typeof bulkImportTextSchema>

// Bulk Ordering Schemas
export const channelOrderingItemSchema = z.object({
  id: z.string(),
  name: z.string(),
  type: channelTypeSchema,
  status: channelStatusSchema,
  baseURL: z.string(),
  orderingWeight: z.number(),
  tags: z.array(z.string()).optional().default([]).nullable(),
  supportedModels: z.array(z.string()).optional().default([]).nullable(),
})
export type ChannelOrderingItem = z.infer<typeof channelOrderingItemSchema>

export const channelOrderingConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: channelOrderingItemSchema,
    })
  ),
  totalCount: z.number(),
})
export type ChannelOrderingConnection = z.infer<typeof channelOrderingConnectionSchema>

export const bulkUpdateChannelOrderingInputSchema = z.object({
  channels: z
    .array(
      z.object({
        id: z.string(),
        orderingWeight: z.number(),
      })
    )
    .min(1, 'At least one channel is required'),
})
export type BulkUpdateChannelOrderingInput = z.infer<typeof bulkUpdateChannelOrderingInputSchema>

export const bulkUpdateChannelOrderingResultSchema = z.object({
  success: z.boolean(),
  updated: z.number(),
  channels: z.array(channelSchema),
})
export type BulkUpdateChannelOrderingResult = z.infer<typeof bulkUpdateChannelOrderingResultSchema>

// Re-export template types from templates.ts
export type {
  ChannelOverrideTemplate,
  ChannelOverrideTemplateConnection,
  CreateChannelOverrideTemplateInput,
  UpdateChannelOverrideTemplateInput,
  ApplyChannelOverrideTemplateInput,
  ApplyChannelOverrideTemplatePayload,
} from './templates'
