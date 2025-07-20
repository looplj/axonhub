import { z } from 'zod'

// Request Status
export const requestStatusSchema = z.enum([
  'pending',
  'processing', 
  'completed',
  'failed'
])
export type RequestStatus = z.infer<typeof requestStatusSchema>

// Request Execution Status
export const requestExecutionStatusSchema = z.enum([
  'pending',
  'processing',
  'completed',
  'failed'
])
export type RequestExecutionStatus = z.infer<typeof requestExecutionStatusSchema>

// Request Execution
export const requestExecutionSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  userID: z.number(),
  requestID: z.string(),
  channelID: z.number(),
  modelID: z.string(),
  requestBody: z.any(), // JSONRawMessage
  responseBody: z.any().nullable(), // JSONRawMessage
  responseChunks: z.array(z.any()).nullable(), // [JSONRawMessage!]
  errorMessage: z.string().nullable(),
  status: requestExecutionStatusSchema,
})
export type RequestExecution = z.infer<typeof requestExecutionSchema>

// Request
export const requestSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  userID: z.string(),
  apiKeyID: z.string(),
  requestBody: z.any(), // JSONRawMessage
  responseBody: z.any().nullable(), // JSONRawMessage
  status: requestStatusSchema,
  executions: z.object({
    edges: z.array(z.object({
      node: requestExecutionSchema,
      cursor: z.string(),
    })),
    pageInfo: z.object({
      hasNextPage: z.boolean(),
      hasPreviousPage: z.boolean(),
      startCursor: z.string().nullable(),
      endCursor: z.string().nullable(),
    }),
    totalCount: z.number(),
  }).optional(),
})
export type Request = z.infer<typeof requestSchema>

// Request Connection (for pagination)
export const requestConnectionSchema = z.object({
  edges: z.array(z.object({
    node: requestSchema,
    cursor: z.string(),
  })),
  pageInfo: z.object({
    hasNextPage: z.boolean(),
    hasPreviousPage: z.boolean(),
    startCursor: z.string().nullable(),
    endCursor: z.string().nullable(),
  }),
  totalCount: z.number(),
})
export type RequestConnection = z.infer<typeof requestConnectionSchema>

// Request Execution Connection (for pagination)
export const requestExecutionConnectionSchema = z.object({
  edges: z.array(z.object({
    node: requestExecutionSchema,
    cursor: z.string(),
  })),
  pageInfo: z.object({
    hasNextPage: z.boolean(),
    hasPreviousPage: z.boolean(),
    startCursor: z.string().nullable(),
    endCursor: z.string().nullable(),
  }),
  totalCount: z.number(),
})
export type RequestExecutionConnection = z.infer<typeof requestExecutionConnectionSchema>