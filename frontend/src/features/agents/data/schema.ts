import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';

export const agentStatusSchema = z.enum(['enabled', 'disabled', 'archived']);
export type AgentStatus = z.infer<typeof agentStatusSchema>;

export const reasoningEffortSchema = z.enum(['none', 'low', 'medium', 'high']);
export type ReasoningEffort = z.infer<typeof reasoningEffortSchema>;

export const agentBuiltinToolSchema = z.object({
  name: z.string(),
  enabled: z.boolean(),
  order: z.number(),
});
export type AgentBuiltinTool = z.infer<typeof agentBuiltinToolSchema>;

export const agentBuiltinSkillSchema = z.object({
  name: z.string(),
  enabled: z.boolean(),
  order: z.number(),
});
export type AgentBuiltinSkill = z.infer<typeof agentBuiltinSkillSchema>;

export const agentSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  projectID: z.string(),
  createdByUserID: z.string(),
  name: z.string(),
  description: z.string(),
  status: agentStatusSchema,
  model: z.string(),
  reasoningEffort: reasoningEffortSchema,
  agentBuiltinTools: z.any(),
  agentBuiltinSkills: z.any(),
  skillsPolicy: z.any(),
  prompt: z
    .object({
      id: z.string(),
      content: z.string(),
    })
    .partial()
    .optional(),
});
export type Agent = z.infer<typeof agentSchema>;

export const agentEdgeSchema = z.object({
  node: agentSchema,
  cursor: z.string(),
});

export const agentConnectionSchema = z.object({
  edges: z.array(agentEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type AgentConnection = z.infer<typeof agentConnectionSchema>;

export const agentSkillsPolicyInputSchema = z.object({
  add: z.enum(['open', 'approval_required', 'registry_only']),
});
export type AgentSkillsPolicyInput = z.infer<typeof agentSkillsPolicyInputSchema>;

export const agentBuiltinToolInputSchema = z.object({
  name: z.string(),
  enabled: z.boolean().default(true),
  order: z.number().default(0),
});
export type AgentBuiltinToolInput = z.infer<typeof agentBuiltinToolInputSchema>;

export const agentBuiltinSkillInputSchema = z.object({
  name: z.string(),
  enabled: z.boolean().default(true),
  order: z.number().default(0),
});
export type AgentBuiltinSkillInput = z.infer<typeof agentBuiltinSkillInputSchema>;

export const createAgentInputSchema = z.object({
  name: z.string().min(1),
  description: z.string().optional(),
  status: agentStatusSchema.optional(),
  model: z.string().optional(),
  reasoningEffort: reasoningEffortSchema.optional(),
  systemPrompt: z.string().min(1),
  builtinTools: z.array(agentBuiltinToolInputSchema).optional(),
  builtinSkills: z.array(agentBuiltinSkillInputSchema).optional(),
  skillsPolicy: agentSkillsPolicyInputSchema.optional(),
});
export type CreateAgentInput = z.infer<typeof createAgentInputSchema>;

export const updateAgentInputSchema = z.object({
  name: z.string().optional(),
  description: z.string().optional(),
  status: agentStatusSchema.optional(),
  model: z.string().optional(),
  reasoningEffort: reasoningEffortSchema.optional(),
  systemPrompt: z.string().optional(),
  builtinTools: z.array(agentBuiltinToolInputSchema).optional(),
  builtinSkills: z.array(agentBuiltinSkillInputSchema).optional(),
  skillsPolicy: agentSkillsPolicyInputSchema.optional(),
});
export type UpdateAgentInput = z.infer<typeof updateAgentInputSchema>;

