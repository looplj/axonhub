import { z } from 'zod'

const projectSchema = z
  .object({
    id: z.string(),
    name: z.string().nullable().optional(),
  })
  .nullable()
  .optional()

const threadSchema = z
  .object({
    id: z.string(),
    threadID: z.string().nullable().optional(),
  })
  .nullable()
  .optional()

const traceRequestsSummarySchema = z
  .object({
    totalCount: z.number().nullable().optional(),
  })
  .nullable()
  .optional()

export const traceSchema = z.object({
  id: z.string(),
  traceID: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  project: projectSchema,
  thread: threadSchema,
  requests: traceRequestsSummarySchema,
})

export type Trace = z.infer<typeof traceSchema>

export const traceConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: traceSchema,
      cursor: z.string(),
    })
  ),
  pageInfo: z.object({
    hasNextPage: z.boolean(),
    hasPreviousPage: z.boolean(),
    startCursor: z.string().nullable(),
    endCursor: z.string().nullable(),
  }),
  totalCount: z.number(),
})

export type TraceConnection = z.infer<typeof traceConnectionSchema>

const spanQuerySchema = z
  .object({
    modelId: z.string().nullable().optional(),
    prompt: z.string().nullable().optional(),
  })
  .nullable()
  .optional()

const spanTextSchema = z
  .object({
    text: z.string().nullable().optional(),
  })
  .nullable()
  .optional()

const spanImageURLSchema = z
  .object({
    url: z.string().nullable().optional(),
  })
  .nullable()
  .optional()

const spanThinkingSchema = z
  .object({
    thinking: z.string().nullable().optional(),
  })
  .nullable()
  .optional()

const spanFunctionCallSchema = z
  .object({
    id: z.string().nullable().optional(),
    name: z.string(),
    arguments: z.string().nullable().optional(),
  })
  .nullable()
  .optional()

const spanFunctionResultSchema = z
  .object({
    id: z.string().nullable().optional(),
    isError: z.boolean().nullable().optional(),
    type: z.string().nullable().optional(),
    text: z.string().nullable().optional(),
  })
  .nullable()
  .optional()

const spanValueSchema = z
  .object({
    query: spanQuerySchema,
    text: spanTextSchema,
    imageUrl: spanImageURLSchema,
    thinking: spanThinkingSchema,
    functionCall: spanFunctionCallSchema,
    functionResult: spanFunctionResultSchema,
  })
  .nullable()
  .optional()

export const spanSchema = z.object({
  id: z.string(),
  name: z.string().nullable().optional(),
  type: z.string(),
  startTime: z.coerce.date().nullable().optional(),
  endTime: z.coerce.date().nullable().optional(),
  value: spanValueSchema,
})

export type Span = z.infer<typeof spanSchema>

const requestMetadataSchema = z
  .object({
    cost: z.number().nullable().optional(),
    itemCount: z.number().nullable().optional(),
    tokens: z.number().nullable().optional(),
  })
  .nullable()
  .optional()

export type RequestMetadata = z.infer<typeof requestMetadataSchema>

export const requestTraceSchema: z.ZodType<any> = z.lazy(() =>
  z.object({
    id: z.string(),
    parentId: z.string().nullable().optional(),
    model: z.string(),
    duration: z.string(),
    startTime: z.coerce.date(),
    endTime: z.coerce.date(),
    metadata: requestMetadataSchema,
    requestSpans: z.array(spanSchema).nullable().optional().default([]),
    responseSpans: z.array(spanSchema).nullable().optional().default([]),
    children: z.array(requestTraceSchema).nullable().optional().default([]),
  })
)

export type RequestTrace = z.infer<typeof requestTraceSchema>

export const traceDetailSchema = z.object({
  id: z.string(),
  traceID: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  project: projectSchema,
  thread: threadSchema,
  requests: traceRequestsSummarySchema,
  rootRequestTrace: requestTraceSchema.nullable().optional(),
})

export type TraceDetail = z.infer<typeof traceDetailSchema>
