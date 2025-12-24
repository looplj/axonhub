import { z } from 'zod'

export const modelTypeSchema = z.enum(['chat', 'embedding', 'rerank'])
export type ModelType = z.infer<typeof modelTypeSchema>

export const modelStatusSchema = z.enum(['enabled', 'disabled', 'archived'])
export type ModelStatus = z.infer<typeof modelStatusSchema>

export const modelReasoningSchema = z.object({
  supported: z.boolean(),
  default: z.boolean(),
})
export type ModelReasoning = z.infer<typeof modelReasoningSchema>

export const modelModalitiesSchema = z.object({
  input: z.array(z.string()),
  output: z.array(z.string()),
})
export type ModelModalities = z.infer<typeof modelModalitiesSchema>

export const modelCostSchema = z.object({
  input: z.number(),
  output: z.number(),
  cacheRead: z.number().optional(),
  cacheWrite: z.number().optional(),
})
export type ModelCost = z.infer<typeof modelCostSchema>

export const modelLimitSchema = z.object({
  context: z.number(),
  output: z.number(),
})
export type ModelLimit = z.infer<typeof modelLimitSchema>

export const modelCardSchema = z.object({
  reasoning: modelReasoningSchema.optional(),
  toolCall: z.boolean().optional(),
  temperature: z.boolean().optional(),
  modalities: modelModalitiesSchema.optional(),
  vision: z.boolean().optional(),
  cost: modelCostSchema.optional(),
  limit: modelLimitSchema.optional(),
  knowledge: z.string().optional(),
  releaseDate: z.string().optional(),
  lastUpdated: z.string().optional(),
})
export type ModelCard = z.infer<typeof modelCardSchema>

export const channelModelAssociationSchema = z.object({
  channelId: z.number(),
  modelId: z.string(),
})
export type ChannelModelAssociation = z.infer<typeof channelModelAssociationSchema>

export const channelRegexAssociationSchema = z.object({
  channelId: z.number(),
  pattern: z.string(),
})
export type ChannelRegexAssociation = z.infer<typeof channelRegexAssociationSchema>

export const regexAssociationSchema = z.object({
  pattern: z.string(),
})
export type RegexAssociation = z.infer<typeof regexAssociationSchema>

export const modelIDAssociationSchema = z.object({
  modelId: z.string(),
})
export type ModelIDAssociation = z.infer<typeof modelIDAssociationSchema>

export const modelAssociationSchema = z.object({
  type: z.enum(['channel_model', 'channel_regex', 'model', 'regex',]),
  priority: z.number().min(0).max(100).optional().default(0),
  channelModel: channelModelAssociationSchema.optional().nullable(),
  channelRegex: channelRegexAssociationSchema.optional().nullable(),
  regex: regexAssociationSchema.optional().nullable(),
  modelId: modelIDAssociationSchema.optional().nullable(),
})
export type ModelAssociation = z.infer<typeof modelAssociationSchema>

export const modelSettingsSchema = z.object({
  associations: z.array(modelAssociationSchema).optional().default([]),
})
export type ModelSettings = z.infer<typeof modelSettingsSchema>

export const modelSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  developer: z.string(),
  modelID: z.string(),
  type: modelTypeSchema,
  name: z.string(),
  icon: z.string(),
  group: z.string(),
  modelCard: modelCardSchema,
  settings: modelSettingsSchema,
  status: modelStatusSchema,
  remark: z.string().optional().nullable(),
})
export type Model = z.infer<typeof modelSchema>

export const createModelInputSchema = z.object({
  developer: z.string().min(1, 'Developer is required'),
  modelID: z.string().min(1, 'Model ID is required'),
  type: modelTypeSchema,
  name: z.string().min(1, 'Name is required'),
  icon: z.string().min(1, 'Icon is required'),
  group: z.string().min(1, 'Group is required'),
  modelCard: modelCardSchema,
  settings: modelSettingsSchema.optional(),
  status: modelStatusSchema.optional(),
  remark: z.string().optional(),
})
export type CreateModelInput = z.infer<typeof createModelInputSchema>

export const updateModelInputSchema = z.object({
  name: z.string().min(1, 'Name is required').optional(),
  icon: z.string().min(1, 'Icon is required').optional(),
  group: z.string().min(1, 'Group is required').optional(),
  modelCard: modelCardSchema.optional(),
  settings: modelSettingsSchema.optional(),
  status: modelStatusSchema.optional(),
  remark: z.string().optional().nullable(),
})
export type UpdateModelInput = z.infer<typeof updateModelInputSchema>

export const modelConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: modelSchema,
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
export type ModelConnection = z.infer<typeof modelConnectionSchema>
