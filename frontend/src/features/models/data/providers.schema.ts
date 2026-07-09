import { z } from 'zod';

const modelTokenCostSchema = z.object({
  input: z.number().optional(),
  output: z.number().optional(),
  cache_read: z.number().optional(),
  cache_write: z.number().optional(),
});

const modelCostTierSchema = modelTokenCostSchema
  .extend({
    tier: z
      .object({
        type: z.string(),
        size: z.number().optional(),
      })
      .passthrough(),
  })
  .passthrough();

// Model cost schema
export const modelCostSchema = modelTokenCostSchema.extend({
  tiers: z.array(modelCostTierSchema).optional(),
  context_over_200k: modelTokenCostSchema.optional(),
});

// Model limit schema
export const modelLimitSchema = z.object({
  context: z.number().optional().nullable(),
  input: z.number().optional().nullable(),
  output: z.number().optional().nullable(),
});

// Model reasoning schema
export const modelReasoningSchema = z.object({
  supported: z.boolean().optional(),
  default: z.boolean().optional(),
});

// Model modalities schema
export const modelModalitiesSchema = z.object({
  input: z.array(z.string()).optional().nullable(),
  output: z.array(z.string()).optional().nullable(),
});

const modelReasoningOptionSchema = z.object({
  type: z.string(),
  values: z.array(z.string().nullable()).optional(),
  min: z.number().optional(),
  max: z.number().optional(),
});

const modelExperimentalModeSchema = z
  .object({
    cost: modelTokenCostSchema.optional(),
    provider: z.record(z.string(), z.unknown()).optional(),
  })
  .passthrough();

const modelExperimentalSchema = z
  .object({
    modes: z.record(z.string(), modelExperimentalModeSchema).optional(),
  })
  .passthrough();

// Single model schema
export const providerModelSchema = z.object({
  id: z.string(),
  name: z.string().optional(),
  description: z.string().optional(),
  family: z.string().optional(),
  attachment: z.boolean().optional(),
  reasoning: modelReasoningSchema.optional(),
  reasoning_options: z.array(modelReasoningOptionSchema).optional(),
  tool_call: z.boolean().optional(),
  structured_output: z.boolean().optional(),
  temperature: z.boolean().optional(),
  knowledge: z.string().optional(),
  release_date: z.string().optional(),
  last_updated: z.string().optional(),
  modalities: modelModalitiesSchema.optional(),
  open_weights: z.boolean().optional(),
  cost: modelCostSchema.optional(),
  limit: modelLimitSchema.optional().nullable(),
  experimental: modelExperimentalSchema.optional(),
  display_name: z.string().optional(),
  extra_capabilities: z.record(z.string(), z.unknown()).optional(),
  vision: z.boolean().optional(),
  type: z.string().optional(),
  metadata: z.record(z.string(), z.unknown()).optional(),
});

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
});

// Providers data schema
export const providersDataSchema = z.object({
  providers: z.record(z.string(), providerSchema),
  updated_at: z.string().optional(),
});

// Type exports
export type ProviderModel = z.infer<typeof providerModelSchema>;
export type Provider = z.infer<typeof providerSchema>;
export type ProvidersData = z.infer<typeof providersDataSchema>;

export function resolveVision(model: ProviderModel): boolean {
  if (model.vision !== undefined) return model.vision;
  return !!model.modalities?.input?.includes('image');
}
