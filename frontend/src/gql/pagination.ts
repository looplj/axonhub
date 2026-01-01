import { z } from 'zod'

export const pageInfoSchema = z.object({
  hasNextPage: z.boolean(),
  hasPreviousPage: z.boolean(),
  startCursor: z.string().optional().nullable(),
  endCursor: z.string().optional().nullable(),
})

export type PageInfo = z.infer<typeof pageInfoSchema>
