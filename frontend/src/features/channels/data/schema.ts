import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';

export const apiFormatSchema = z.enum(['openai/chat_completions', 'openai/responses', 'anthropic/messages', 'gemini/contents']);

export type ApiFormat = z.infer<typeof apiFormatSchema>;

// Channel Types
export const channelTypeSchema = z.enum([
  'openai',
  'openai_responses',
  'codex',
  'anthropic',
  'anthropic_aws',
  'anthropic_gcp',
  'gemini_openai',
  'gemini',
  'gemini_vertex',
  'deepseek',
  'deepseek_anthropic',
  'deepinfra',
  'doubao',
  'doubao_anthropic',
  'moonshot',
  'moonshot_anthropic',
  'zhipu',
  'zai',
  'zhipu_anthropic',
  'zai_anthropic',
  'vercel',
  'anthropic_fake',
  'openai_fake',
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
  'github',
  'claudecode',
]);
export type ChannelType = z.infer<typeof channelTypeSchema>;

// Channel Status
export const channelStatusSchema = z.enum(['enabled', 'disabled', 'archived']);
export type ChannelStatus = z.infer<typeof channelStatusSchema>;

// Model Mapping
export const modelMappingSchema = z.object({
  from: z.string(),
  to: z.string(),
});
export type ModelMapping = z.infer<typeof modelMappingSchema>;

// Header Entry
export const headerEntrySchema = z.object({
  key: z.string().min(1, 'Header key is required'),
  value: z.string(),
});
export type HeaderEntry = z.infer<typeof headerEntrySchema>;

// Proxy Type
export const proxyTypeSchema = z.enum(['disabled', 'environment', 'url']);
export type ProxyType = z.infer<typeof proxyTypeSchema>;

// Proxy Config
export const proxyConfigSchema = z.object({
  type: proxyTypeSchema,
  url: z.string().optional(),
  username: z.string().optional(),
  password: z.string().optional(),
});
export type ProxyConfig = z.infer<typeof proxyConfigSchema>;

// Transform Options
export const transformOptionsSchema = z.object({
  forceArrayInstructions: z.boolean().optional(),
  forceArrayInputs: z.boolean().optional(),
  replaceDeveloperRoleWithSystem: z.boolean().optional(),
});
export type TransformOptions = z.infer<typeof transformOptionsSchema>;

// Channel Performance
export const channelPerformanceSchema = z.object({
  avgLatencyMs: z.number(),
  avgTokenPerSecond: z.number(),
  avgStreamFirstTokenLatencyMs: z.number(),
  avgStreamTokenPerSecond: z.number(),
});
export type ChannelPerformance = z.infer<typeof channelPerformanceSchema>;

// Channel Probe
export const channelProbePointSchema = z.object({
  timestamp: z.number(),
  totalRequestCount: z.number(),
  successRequestCount: z.number(),
});
export type ChannelProbePoint = z.infer<typeof channelProbePointSchema>;

export const channelProbeDataSchema = z.object({
  channelID: z.string(),
  points: z.array(channelProbePointSchema),
});
export type ChannelProbeData = z.infer<typeof channelProbeDataSchema>;

// Channel Settings
export const channelSettingsSchema = z.object({
  extraModelPrefix: z.string().optional(),
  modelMappings: z.array(modelMappingSchema).nullable(),
  autoTrimedModelPrefixes: z.array(z.string()).optional().nullable(),
  hideOriginalModels: z.boolean().optional(),
  hideMappedModels: z.boolean().optional(),
  overrideParameters: z.string().optional(),
  overrideHeaders: z.array(headerEntrySchema).optional().nullable(),
  proxy: proxyConfigSchema.optional().nullable(),
  transformOptions: transformOptionsSchema.optional(),
});
export type ChannelSettings = z.infer<typeof channelSettingsSchema>;

// Channel Model Entry
export const channelModelEntrySchema = z.object({
  requestModel: z.string(),
  actualModel: z.string(),
  source: z.string(),
});
export type ChannelModelEntry = z.infer<typeof channelModelEntrySchema>;

// Channel Credentials
export const channelCredentialsSchema = z.object({
  apiKey: z.string().optional().nullable(),
  platformType: z.string().optional().nullable(),
  aws: z
    .object({
      accessKeyID: z.string(),
      secretAccessKey: z.string(),
      region: z.string(),
    })
    .optional()
    .nullable(),
  gcp: z
    .object({
      region: z.string(),
      projectID: z.string(),
      jsonData: z.string(),
    })
    .optional()
    .nullable(),
});
export type ChannelCredentials = z.infer<typeof channelCredentialsSchema>;

// Channel
export const channelSchema = z.object({
  id: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
  type: channelTypeSchema,
  baseURL: z.string(),
  name: z.string(),
  status: channelStatusSchema,
  credentials: channelCredentialsSchema.optional().nullable(),
  supportedModels: z.array(z.string()),
  autoSyncSupportedModels: z.boolean().default(false),
  tags: z.array(z.string()).optional().default([]).nullable(),
  defaultTestModel: z.string(),
  settings: channelSettingsSchema.optional().nullable(),
  orderingWeight: z.number().optional().default(0),
  errorMessage: z.string().optional().nullable(),
  remark: z.string().optional().nullable(),
  channelPerformance: channelPerformanceSchema.optional().nullable(),
  allModelEntries: z.array(channelModelEntrySchema).optional(),
});
export type Channel = z.infer<typeof channelSchema>;

// Pricing Schemas
export const pricingModeSchema = z.enum(['flat_fee', 'usage_per_unit', 'usage_tiered']);
export type PricingMode = z.infer<typeof pricingModeSchema>;

export const priceItemCodeSchema = z.enum([
  'prompt_tokens',
  'completion_tokens',
  'prompt_cached_tokens',
  'prompt_write_cached_tokens',
]);
export type PriceItemCode = z.infer<typeof priceItemCodeSchema>;

export const priceTierSchema = z.object({
  upTo: z.number().nullable().optional(),
  pricePerUnit: z.union([z.string(), z.number()]),
});
export type PriceTier = z.infer<typeof priceTierSchema>;

export const tieredPricingSchema = z.object({
  tiers: z.array(priceTierSchema),
});
export type TieredPricing = z.infer<typeof tieredPricingSchema>;

export const pricingSchema = z.object({
  mode: pricingModeSchema,
  flatFee: z.union([z.string(), z.number()]).nullable().optional(),
  usagePerUnit: z.union([z.string(), z.number()]).nullable().optional(),
  usageTiered: tieredPricingSchema.nullable().optional(),
});
export type Pricing = z.infer<typeof pricingSchema>;

export const promptWriteCacheVariantSchema = z.object({
  variantCode: z.enum(['five_min', 'one_hour']),
  pricing: pricingSchema,
});
export type PromptWriteCacheVariant = z.infer<typeof promptWriteCacheVariantSchema>;

export const modelPriceItemSchema = z.object({
  itemCode: priceItemCodeSchema,
  pricing: pricingSchema,
  promptWriteCacheVariants: z.array(promptWriteCacheVariantSchema).nullable().optional(),
});
export type ModelPriceItem = z.infer<typeof modelPriceItemSchema>;

export const modelPriceSchema = z.object({
  items: z.array(modelPriceItemSchema),
});
export type ModelPrice = z.infer<typeof modelPriceSchema>;

export const channelModelPriceSchema = z.object({
  id: z.string(),
  modelID: z.string(),
  price: modelPriceSchema,
});
export type ChannelModelPrice = z.infer<typeof channelModelPriceSchema>;

export const saveChannelModelPriceInputSchema = z.object({
  modelId: z.string(),
  price: modelPriceSchema,
});
export type SaveChannelModelPriceInput = z.infer<typeof saveChannelModelPriceInputSchema>;

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
    remark: z.string().optional(),
    settings: channelSettingsSchema.optional(),
    credentials: z.object({
      apiKey: z.string().min(1, 'API Key is required'),
      platformType: z.string().optional().nullable(),
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
    if (data.type === 'codex') {
      const issue = {
        code: 'custom' as const,
        message: 'channels.dialogs.fields.supportedModels.codexOAuthCredentialsRequired',
        path: ['credentials', 'apiKey'] as const,
      };

      if (data.credentials?.platformType !== 'codex') {
        ctx.addIssue(issue);
        return;
      }

      let json: unknown;
      try {
        json = JSON.parse(data.credentials.apiKey);
      } catch {
        ctx.addIssue(issue);
        return;
      }

      const parsed = z
        .object({
          access_token: z.string().min(1),
          refresh_token: z.string().min(1),
        })
        .safeParse(json);

      if (!parsed.success) {
        ctx.addIssue(issue);
      }
    }

    // 如果是 anthropic_gcp 类型，GCP 字段必填（精确到字段级报错）
    if (data.type === 'anthropic_gcp') {
      const gcp = data.credentials?.gcp;
      if (!gcp?.region) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Region is required',
          path: ['credentials', 'gcp', 'region'],
        });
      }
      if (!gcp?.projectID) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Project ID is required',
          path: ['credentials', 'gcp', 'projectID'],
        });
      }
      if (!gcp?.jsonData) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Service Account JSON is required',
          path: ['credentials', 'gcp', 'jsonData'],
        });
      }
    }
  });
export type CreateChannelInput = z.infer<typeof createChannelInputSchema>;

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
        platformType: z.string().optional().nullable(),
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
    if (data.type === 'codex') {
      const issue = {
        code: 'custom' as const,
        message: 'channels.dialogs.fields.supportedModels.codexOAuthCredentialsRequired',
        path: ['credentials', 'apiKey'] as const,
      };

      if (!data.credentials) return;

      const platformType = data.credentials.platformType;
      const apiKey = data.credentials.apiKey;

      if (platformType || apiKey) {
        if (platformType !== 'codex' || !apiKey) {
          ctx.addIssue(issue);
          return;
        }

        let json: unknown;
        try {
          json = JSON.parse(apiKey);
        } catch {
          ctx.addIssue(issue);
          return;
        }

        const parsed = z
          .object({
            access_token: z.string().min(1),
            refresh_token: z.string().min(1),
          })
          .safeParse(json);

        if (!parsed.success) {
          ctx.addIssue(issue);
        }
      }
    }

    // 如果是 anthropic_gcp 类型且提供了 credentials，GCP 字段必填（字段级报错）
    if (data.type === 'anthropic_gcp' && data.credentials) {
      const gcp = data.credentials.gcp;
      if (!gcp?.region) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Region is required',
          path: ['credentials', 'gcp', 'region'],
        });
      }
      if (!gcp?.projectID) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Project ID is required',
          path: ['credentials', 'gcp', 'projectID'],
        });
      }
      if (!gcp?.jsonData) {
        ctx.addIssue({
          code: 'custom',
          message: 'GCP Service Account JSON is required',
          path: ['credentials', 'gcp', 'jsonData'],
        });
      }
    }
  });
export type UpdateChannelInput = z.infer<typeof updateChannelInputSchema>;

// Channel Connection (for pagination)
export const channelConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: channelSchema,
      cursor: z.string(),
    })
  ),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type ChannelConnection = z.infer<typeof channelConnectionSchema>;

// Bulk Import Schemas
export const bulkImportChannelItemSchema = z.object({
  type: channelTypeSchema,
  name: z.string().min(1, 'Name is required'),
  baseURL: z.string().url('Please enter a valid URL').min(1, 'Base URL is required'),
  apiKey: z.string().min(1, 'API Key is required'),
  supportedModels: z.array(z.string()).min(1, 'At least one supported model is required'),
  defaultTestModel: z.string().min(1, 'Please select a default test model'),
});
export type BulkImportChannelItem = z.infer<typeof bulkImportChannelItemSchema>;

export const bulkImportChannelsInputSchema = z.object({
  channels: z.array(bulkImportChannelItemSchema).min(1, 'At least one channel is required'),
});
export type BulkImportChannelsInput = z.infer<typeof bulkImportChannelsInputSchema>;

export const bulkImportChannelsResultSchema = z.object({
  success: z.boolean(),
  created: z.number(),
  failed: z.number(),
  errors: z.array(z.string()).optional().nullable(),
  channels: z.array(channelSchema).nullable(),
});
export type BulkImportChannelsResult = z.infer<typeof bulkImportChannelsResultSchema>;

// Raw text input for bulk import
export const bulkImportTextSchema = z.object({
  text: z.string().min(1, 'Please enter data to import'),
});
export type BulkImportText = z.infer<typeof bulkImportTextSchema>;

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
  allModelEntries: z.array(channelModelEntrySchema).optional(),
});
export type ChannelOrderingItem = z.infer<typeof channelOrderingItemSchema>;

export const channelOrderingConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: channelOrderingItemSchema,
    })
  ),
  totalCount: z.number(),
});
export type ChannelOrderingConnection = z.infer<typeof channelOrderingConnectionSchema>;

export const bulkUpdateChannelOrderingInputSchema = z.object({
  channels: z
    .array(
      z.object({
        id: z.string(),
        orderingWeight: z.number(),
      })
    )
    .min(1, 'At least one channel is required'),
});
export type BulkUpdateChannelOrderingInput = z.infer<typeof bulkUpdateChannelOrderingInputSchema>;

export const bulkUpdateChannelOrderingResultSchema = z.object({
  success: z.boolean(),
  updated: z.number(),
  channels: z.array(channelSchema),
});
export type BulkUpdateChannelOrderingResult = z.infer<typeof bulkUpdateChannelOrderingResultSchema>;

// Re-export template types from templates.ts
export type {
  ChannelOverrideTemplate,
  ChannelOverrideTemplateConnection,
  CreateChannelOverrideTemplateInput,
  UpdateChannelOverrideTemplateInput,
  ApplyChannelOverrideTemplateInput,
  ApplyChannelOverrideTemplatePayload,
} from './templates';
