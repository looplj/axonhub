import { z } from 'zod'

// Model cost schema
export const modelCostSchema = z.object({
  input: z.number().optional(),
  output: z.number().optional(),
  cache_read: z.number().optional(),
  cache_write: z.number().optional(),
})

// Model limit schema
export const modelLimitSchema = z.object({
  context: z.number().optional().nullable(),
  output: z.number().optional().nullable(),
})

// Model reasoning schema
export const modelReasoningSchema = z.object({
  supported: z.boolean().optional(),
  default: z.boolean().optional(),
})

// Model modalities schema
export const modelModalitiesSchema = z.object({
  input: z.array(z.string()).optional().nullable(),
  output: z.array(z.string()).optional().nullable(),
})

// Single model schema
export const providerModelSchema = z.object({
  id: z.string(),
  name: z.string().optional(),
  family: z.string().optional(),
  attachment: z.boolean().optional(),
  reasoning: modelReasoningSchema.optional(),
  tool_call: z.boolean().optional(),
  temperature: z.boolean().optional(),
  knowledge: z.string().optional(),
  release_date: z.string().optional(),
  last_updated: z.string().optional(),
  modalities: modelModalitiesSchema.optional(),
  open_weights: z.boolean().optional(),
  cost: modelCostSchema.optional(),
  limit: modelLimitSchema.optional().nullable(),
  display_name: z.string().optional(),
  vision: z.boolean().optional(),
  type: z.string().optional(),
  metadata: z.record(z.string(), z.unknown()).optional(),
})

// Provider schema
export const providerSchema = z.object({
  id: z.string().optional(),
  api: z.string().optional(),
  name: z.string().optional(),
  doc: z.string().optional(),
  display_name: z.string().optional(),
  vision: z.boolean().optional(),
  models: z.array(providerModelSchema).optional(),
  metadata: z.record(z.string(), z.unknown()).optional(),
})

// Providers data schema
export const providersDataSchema = z.object({
  providers: z.record(z.string(), providerSchema),
  updated_at: z.string().optional(),
})

// Type exports
export type ProviderModel = z.infer<typeof providerModelSchema>
export type Provider = z.infer<typeof providerSchema>
export type ProvidersData = z.infer<typeof providersDataSchema>
