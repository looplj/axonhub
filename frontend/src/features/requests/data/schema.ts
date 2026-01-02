import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';
import { apiKeySchema } from '@/features/apikeys/data/schema';
import { channelSchema } from '@/features/channels/data';

// Request Status
export const requestStatusSchema = z.enum(['pending', 'processing', 'completed', 'failed', 'canceled']);
export type RequestStatus = z.infer<typeof requestStatusSchema>;

// Request Source
export const requestSourceSchema = z.enum(['api', 'playground', 'test']);
export type RequestSource = z.infer<typeof requestSourceSchema>;

// Request Execution Status
export const requestExecutionStatusSchema = z.enum(['pending', 'processing', 'completed', 'failed', 'canceled']);
export type RequestExecutionStatus = z.infer<typeof requestExecutionStatusSchema>;

// Request Execution
export const requestExecutionSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  // userID: z.number(),
  // requestID: z.string(),
  // channelID: z.number(),
  channel: channelSchema.partial().nullable().optional(),
  modelID: z.string(),
  requestBody: z.any(), // JSONRawMessage
  responseBody: z.any().nullable(), // JSONRawMessage
  responseChunks: z.array(z.any()).nullable(), // [JSONRawMessage!]
  errorMessage: z.string().nullable(),
  status: requestExecutionStatusSchema,
  metricsLatencyMs: z.number().nullable().optional(),
  metricsFirstTokenLatencyMs: z.number().nullable().optional(),
});
export type RequestExecution = z.infer<typeof requestExecutionSchema>;

// Request
export const requestSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  // apiKeyID: z.string().optional().nullable(),
  apiKey: apiKeySchema.partial().nullable().optional(),
  // channelID: z.string().optional().nullable(),
  channel: channelSchema.partial().nullable().optional(),
  source: requestSourceSchema,
  modelID: z.string(),
  requestBody: z.any().nullable().optional(), // JSONRawMessage
  responseBody: z.any().nullable().optional(), // JSONRawMessage
  responseChunks: z.array(z.any()).nullable().optional(), // [JSONRawMessage!]
  status: requestStatusSchema,
  stream: z.boolean().nullable(),
  metricsLatencyMs: z.number().nullable().optional(),
  metricsFirstTokenLatencyMs: z.number().nullable().optional(),
  executions: z
    .object({
      edges: z.array(
        z.object({
          node: requestExecutionSchema,
          cursor: z.string(),
        })
      ),
      pageInfo: pageInfoSchema,
      totalCount: z.number(),
    })
    .optional(),
});

export type Request = z.infer<typeof requestSchema>;

// Request Connection (for pagination)
export const requestConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: requestSchema,
      cursor: z.string(),
    })
  ),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type RequestConnection = z.infer<typeof requestConnectionSchema>;

// Request Execution Connection (for pagination)
export const requestExecutionConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: requestExecutionSchema,
      cursor: z.string(),
    })
  ),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type RequestExecutionConnection = z.infer<typeof requestExecutionConnectionSchema>;
