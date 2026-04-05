import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';

export const messageChannelTypeSchema = z.enum(['feishu']);
export type MessageChannelType = z.infer<typeof messageChannelTypeSchema>;

export const messageChannelStatusSchema = z.enum(['enabled', 'disabled']);
export type MessageChannelStatus = z.infer<typeof messageChannelStatusSchema>;

export const feishuSettingsSchema = z.object({
  appId: z.string().optional(),
  appSecret: z.string().optional(),
  encryptKey: z.string().optional(),
  verificationToken: z.string().optional(),
  allowFrom: z.array(z.string()).nullable(),
  excludeKeywords: z.array(z.string()).nullable(),
});
export type FeishuSettings = z.infer<typeof feishuSettingsSchema>;

export const messageChannelSettingsSchema = z.object({
  feishu: feishuSettingsSchema.optional(),
});
export type MessageChannelSettings = z.infer<typeof messageChannelSettingsSchema>;

export const messageChatTypeSchema = z.enum(['dm', 'group']);
export type MessageChatType = z.infer<typeof messageChatTypeSchema>;

export const messageChannelAgentInstanceBindingSchema = z.object({
  chatType: messageChatTypeSchema.optional(),
  chatID: z.string().optional(),
  allowFrom: z.array(z.string()).optional().nullable(),
  excludeKeywords: z.array(z.string()).optional().nullable(),
  allowWithoutMention: z.boolean().optional(),
});
export type MessageChannelAgentInstanceBinding = z.infer<typeof messageChannelAgentInstanceBindingSchema>;

export const messageChannelAgentInstanceSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  messageChannelID: z.string(),
  agentInstanceID: z.string(),
  enabled: z.boolean(),
  config: messageChannelAgentInstanceBindingSchema,
  agentInstance: z.object({
    id: z.string(),
    name: z.string(),
    description: z.string(),
    status: z.string(),
  }),
});
export type MessageChannelAgentInstance = z.infer<typeof messageChannelAgentInstanceSchema>;

export const messageChannelAgentInstanceEdgeSchema = z.object({
  node: messageChannelAgentInstanceSchema,
  cursor: z.string(),
});

export const messageChannelAgentInstanceConnectionSchema = z.object({
  edges: z.array(messageChannelAgentInstanceEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type MessageChannelAgentInstanceConnection = z.infer<typeof messageChannelAgentInstanceConnectionSchema>;

export const messageChannelSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  projectID: z.string(),
  name: z.string(),
  description: z.string(),
  type: messageChannelTypeSchema,
  status: messageChannelStatusSchema,
  settings: messageChannelSettingsSchema,
  agentInstanceBindings: messageChannelAgentInstanceConnectionSchema.optional(),
});
export type MessageChannel = z.infer<typeof messageChannelSchema>;

export const messageChannelEdgeSchema = z.object({
  node: messageChannelSchema,
  cursor: z.string(),
});

export const messageChannelConnectionSchema = z.object({
  edges: z.array(messageChannelEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type MessageChannelConnection = z.infer<typeof messageChannelConnectionSchema>;

export const feishuSettingsInputSchema = z.object({
  appId: z.string().optional(),
  appSecret: z.string().optional(),
  encryptKey: z.string().optional(),
  verificationToken: z.string().optional(),
  allowFrom: z.array(z.string()).optional().nullable(),
  excludeKeywords: z.array(z.string()).optional().nullable(),
});
export type FeishuSettingsInput = z.infer<typeof feishuSettingsInputSchema>;

export const messageChannelSettingsInputSchema = z.object({
  feishu: feishuSettingsInputSchema.optional(),
});
export type MessageChannelSettingsInput = z.infer<typeof messageChannelSettingsInputSchema>;

export const createMessageChannelInputSchema = z.object({
  name: z.string().min(1),
  description: z.string().optional(),
  type: messageChannelTypeSchema.default('feishu'),
  status: messageChannelStatusSchema.default('enabled'),
  settings: messageChannelSettingsInputSchema.optional(),
});
export type CreateMessageChannelInput = z.infer<typeof createMessageChannelInputSchema>;

export const updateMessageChannelInputSchema = z.object({
  name: z.string().optional(),
  description: z.string().optional(),
  type: messageChannelTypeSchema.optional(),
  status: messageChannelStatusSchema.optional(),
  settings: messageChannelSettingsInputSchema.optional(),
});
export type UpdateMessageChannelInput = z.infer<typeof updateMessageChannelInputSchema>;

// Binding management inputs
export const messageChannelAgentInstanceBindingInputSchema = z.object({
  chatType: messageChatTypeSchema.optional(),
  chatID: z.string().optional(),
  allowFrom: z.array(z.string()).optional().nullable(),
  excludeKeywords: z.array(z.string()).optional().nullable(),
  allowWithoutMention: z.boolean().optional(),
});
export type MessageChannelAgentInstanceBindingInput = z.infer<typeof messageChannelAgentInstanceBindingInputSchema>;

export const createAgentInstanceBindingInputSchema = z.object({
  messageChannelID: z.string(),
  agentInstanceID: z.string(),
  enabled: z.boolean().default(true),
  config: messageChannelAgentInstanceBindingInputSchema.optional(),
});
export type CreateAgentInstanceBindingInput = z.infer<typeof createAgentInstanceBindingInputSchema>;

export const updateAgentInstanceBindingInputSchema = z.object({
  chatType: messageChatTypeSchema.optional(),
  chatID: z.string().optional(),
  enabled: z.boolean().optional(),
  config: messageChannelAgentInstanceBindingInputSchema.optional(),
});
export type UpdateAgentInstanceBindingInput = z.infer<typeof updateAgentInstanceBindingInputSchema>;
