import { z } from 'zod'

const apiKeyStatusSchema = z.union([
  z.literal('active'),
  z.literal('inactive'),
  z.literal('suspended'),
])
export const apiKeySchema = z.object({
  id: z.string(),
  name: z.string(),
  status: apiKeyStatusSchema,
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
})
export type ApiKey = z.infer<typeof apiKeySchema>

export const apiKeyListSchema = z.array(apiKeySchema)
