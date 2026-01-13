import { z } from 'zod';

export const promptActionSchema = z.object({
  type: z.enum(['prepend', 'append']),
});

export const promptActivationConditionSchema = z.object({
  type: z.enum(['model_id', 'model_pattern']),
  modelId: z.string().nullable().optional(),
  modelPattern: z.string().nullable().optional(),
});

export const promptActivationConditionCompositeSchema = z.object({
  conditions: z.array(promptActivationConditionSchema),
});

export const promptSettingsSchema = z.object({
  action: promptActionSchema,
  conditions: z.array(promptActivationConditionCompositeSchema).nullable().optional(),
});

export const promptSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  projectID: z.number(),
  name: z.string(),
  description: z.string(),
  role: z.string(),
  content: z.string(),
  status: z.enum(['enabled', 'disabled']),
  settings: promptSettingsSchema,
});

export const promptEdgeSchema = z.object({
  node: promptSchema,
  cursor: z.string(),
});

export const pageInfoSchema = z.object({
  hasNextPage: z.boolean(),
  hasPreviousPage: z.boolean(),
  startCursor: z.string().nullable(),
  endCursor: z.string().nullable(),
});

export const promptConnectionSchema = z.object({
  edges: z.array(promptEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});

export type PromptAction = z.infer<typeof promptActionSchema>;
export type PromptActivationCondition = z.infer<typeof promptActivationConditionSchema>;
export type PromptActivationConditionComposite = z.infer<typeof promptActivationConditionCompositeSchema>;
export type PromptSettings = z.infer<typeof promptSettingsSchema>;
export type Prompt = z.infer<typeof promptSchema>;
export type PromptEdge = z.infer<typeof promptEdgeSchema>;
export type PageInfo = z.infer<typeof pageInfoSchema>;
export type PromptConnection = z.infer<typeof promptConnectionSchema>;

export interface CreatePromptInput {
  name: string;
  description?: string;
  role: string;
  content: string;
  status?: 'enabled' | 'disabled';
  settings: {
    action: {
      type: 'prepend' | 'append';
    };
    conditions: Array<{
      conditions: Array<{
        type: 'model_id' | 'model_pattern';
        modelId?: string;
        modelPattern?: string;
      }>;
    }>;
  };
}

export interface UpdatePromptInput {
  name?: string;
  description?: string;
  role?: string;
  content?: string;
  status?: 'enabled' | 'disabled';
  settings?: {
    action: {
      type: 'prepend' | 'append';
    };
    conditions: Array<{
      conditions: Array<{
        type: 'model_id' | 'model_pattern';
        modelId?: string;
        modelPattern?: string;
      }>;
    }>;
  };
}
