import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';

export const agentHostTypeSchema = z.enum(['vm', 'docker', 'local']);
export type AgentHostType = z.infer<typeof agentHostTypeSchema>;

export const agentHostStatusSchema = z.enum(['active', 'inactive', 'error']);
export type AgentHostStatus = z.infer<typeof agentHostStatusSchema>;

export const agentHostAuthMethodSchema = z.enum(['password', 'ssh_key']);
export type AgentHostAuthMethod = z.infer<typeof agentHostAuthMethodSchema>;

export const agentHostSchema = z.object({
  id: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
  name: z.string(),
  type: agentHostTypeSchema,
  status: agentHostStatusSchema,
  addr: z.string(),
  user: z.string(),
  password: z.string(),
  authMethod: agentHostAuthMethodSchema,
  sshPrivateKey: z.string(),
});
export type AgentHost = z.infer<typeof agentHostSchema>;

export const createAgentHostInputSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  type: z.enum(['vm', 'docker']),
  status: agentHostStatusSchema.optional(),
  addr: z.string().optional(),
  user: z.string().optional(),
  password: z.string().optional(),
  authMethod: agentHostAuthMethodSchema.optional(),
  sshPrivateKey: z.string().optional(),
}).refine(
  (data) => {
    if (!data.addr || data.addr.length === 0) {
      return false;
    }
    const isLocalhost = data.addr === 'localhost' || data.addr === '127.0.0.1';
    if (isLocalhost) {
      return true;
    }
    if (!data.user || data.user.length === 0) {
      return false;
    }
    const method = data.authMethod || 'password';
    if (method === 'password') {
      return !!(data.password && data.password.length > 0);
    }
    return !!(data.sshPrivateKey && data.sshPrivateKey.length > 0);
  },
  {
    message: 'Host is required for VM and Docker types. User and credentials are required for non-localhost hosts.',
    path: ['addr'],
  }
);
export type CreateAgentHostInput = z.infer<typeof createAgentHostInputSchema>;

export const updateAgentHostInputSchema = z.object({
  name: z.string().min(1, 'Name is required').optional(),
  type: agentHostTypeSchema.optional(),
  status: agentHostStatusSchema.optional(),
  addr: z.string().min(1, 'Host is required').optional(),
  user: z.string().optional(),
  password: z.string().optional(),
  authMethod: agentHostAuthMethodSchema.optional(),
  sshPrivateKey: z.string().optional(),
});
export type UpdateAgentHostInput = z.infer<typeof updateAgentHostInputSchema>;

export const agentHostConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: agentHostSchema,
      cursor: z.string(),
    })
  ),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type AgentHostConnection = z.infer<typeof agentHostConnectionSchema>;
