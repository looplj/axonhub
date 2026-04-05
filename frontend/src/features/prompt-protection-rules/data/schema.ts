import { z } from 'zod';

export const promptProtectionSettingsSchema = z.object({
  action: z.enum(['mask', 'reject']),
  replacement: z.string().nullable().optional(),
  scopes: z.array(z.string()).default([]),
});

export const promptProtectionRuleSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  name: z.string(),
  description: z.string(),
  pattern: z.string(),
  status: z.enum(['enabled', 'disabled', 'archived']),
  settings: promptProtectionSettingsSchema,
});

export const promptProtectionRuleEdgeSchema = z.object({
  node: promptProtectionRuleSchema,
  cursor: z.string(),
});

export const promptProtectionRulePageInfoSchema = z.object({
  hasNextPage: z.boolean(),
  hasPreviousPage: z.boolean(),
  startCursor: z.string().nullable(),
  endCursor: z.string().nullable(),
});

export const promptProtectionRuleConnectionSchema = z.object({
  edges: z.array(promptProtectionRuleEdgeSchema),
  pageInfo: promptProtectionRulePageInfoSchema,
  totalCount: z.number(),
});

export type PromptProtectionSettings = z.infer<typeof promptProtectionSettingsSchema>;
export type PromptProtectionRule = z.infer<typeof promptProtectionRuleSchema>;
export type PromptProtectionRuleConnection = z.infer<typeof promptProtectionRuleConnectionSchema>;

export interface CreatePromptProtectionRuleInput {
  name: string;
  description?: string;
  pattern: string;
  settings: {
    action: 'mask' | 'reject';
    replacement?: string;
    scopes?: string[];
  };
}

export interface UpdatePromptProtectionRuleInput {
  name?: string;
  description?: string;
  pattern?: string;
  status?: 'enabled' | 'disabled' | 'archived';
  settings?: {
    action: 'mask' | 'reject';
    replacement?: string;
    scopes?: string[];
  };
}
