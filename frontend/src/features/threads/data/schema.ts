import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';
import { traceConnectionSchema, usageMetadataSchema } from '@/features/traces/data/schema';

const projectSchema = z
  .object({
    id: z.string(),
    name: z.string().nullable().optional(),
  })
  .nullable()
  .optional();

const threadTracesSummarySchema = z
  .object({
    totalCount: z.number().nullable().optional(),
  })
  .nullable()
  .optional();

export const threadSchema = z.object({
  id: z.string(),
  threadID: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  project: projectSchema,
  tracesSummary: threadTracesSummarySchema,
  firstUserQuery: z.string().nullable().optional(),
  usageMetadata: usageMetadataSchema,
});

export type Thread = z.infer<typeof threadSchema>;

export const threadConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: threadSchema,
      cursor: z.string(),
    })
  ),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});

export type ThreadConnection = z.infer<typeof threadConnectionSchema>;

export const threadDetailSchema = threadSchema.extend({
  tracesConnection: traceConnectionSchema.optional(),
});

export type ThreadDetail = z.infer<typeof threadDetailSchema>;
