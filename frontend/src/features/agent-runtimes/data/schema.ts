import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';

// Agent Runtime Type
export const agentRuntimeTypeSchema = z.enum(['vm', 'docker', 'local']);
export type AgentRuntimeType = z.infer<typeof agentRuntimeTypeSchema>;

// Agent Runtime Status
export const agentRuntimeStatusSchema = z.enum(['active', 'inactive', 'error']);
export type AgentRuntimeStatus = z.infer<typeof agentRuntimeStatusSchema>;

// Agent Runtime Auth Method
export const agentRuntimeAuthMethodSchema = z.enum(['password', 'ssh_key']);
export type AgentRuntimeAuthMethod = z.infer<typeof agentRuntimeAuthMethodSchema>;

// Agent Runtime
export const agentRuntimeSchema = z.object({
  id: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
  name: z.string(),
  type: agentRuntimeTypeSchema,
  status: agentRuntimeStatusSchema,
  host: z.string(),
  user: z.string(),
  password: z.string(),
  authMethod: agentRuntimeAuthMethodSchema,
  sshPrivateKey: z.string(),
});
export type AgentRuntime = z.infer<typeof agentRuntimeSchema>;

// Create Agent Runtime Input
export const createAgentRuntimeInputSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  type: z.enum(['vm', 'docker']),
  status: agentRuntimeStatusSchema.optional(),
  host: z.string().optional(),
  user: z.string().optional(),
  password: z.string().optional(),
  authMethod: agentRuntimeAuthMethodSchema.optional(),
  sshPrivateKey: z.string().optional(),
}).refine(
  (data) => {
    if (!data.host || data.host.length === 0) {
      return false;
    }
    const isLocalhost = data.host === 'localhost' || data.host === '127.0.0.1';
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
    path: ['host'],
  }
);
export type CreateAgentRuntimeInput = z.infer<typeof createAgentRuntimeInputSchema>;

// Update Agent Runtime Input
export const updateAgentRuntimeInputSchema = z.object({
  name: z.string().min(1, 'Name is required').optional(),
  type: agentRuntimeTypeSchema.optional(),
  status: agentRuntimeStatusSchema.optional(),
  host: z.string().min(1, 'Host is required').optional(),
  user: z.string().optional(),
  password: z.string().optional(),
  authMethod: agentRuntimeAuthMethodSchema.optional(),
  sshPrivateKey: z.string().optional(),
});
export type UpdateAgentRuntimeInput = z.infer<typeof updateAgentRuntimeInputSchema>;

// Agent Runtime Connection (for pagination)
export const agentRuntimeConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: agentRuntimeSchema,
      cursor: z.string(),
    })
  ),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type AgentRuntimeConnection = z.infer<typeof agentRuntimeConnectionSchema>;
