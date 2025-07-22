import { z } from 'zod'

// Model schema based on GraphQL
export const modelSchema = z.object({
  id: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
  name: z.string(),
  description: z.string().optional(),
  provider: z.string(),
  modelID: z.string(),
  config: z.record(z.any()).optional(),
})

export type Model = z.infer<typeof modelSchema>

// GraphQL connection types
export const modelEdgeSchema = z.object({
  node: modelSchema,
  cursor: z.string(),
})

export const pageInfoSchema = z.object({
  hasNextPage: z.boolean(),
  hasPreviousPage: z.boolean(),
  startCursor: z.string().nullable(),
  endCursor: z.string().nullable(),
})

export const modelConnectionSchema = z.object({
  edges: z.array(modelEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number().optional(),
})

export type ModelConnection = z.infer<typeof modelConnectionSchema>

// Input schemas for mutations
export const createModelInputSchema = z.object({
  name: z.string().min(1, '名称不能为空'),
  description: z.string().optional(),
  provider: z.string().min(1, '提供商不能为空'),
  modelID: z.string().min(1, '模型ID不能为空'),
  config: z.record(z.any()).optional(),
})

export const updateModelInputSchema = z.object({
  name: z.string().min(1, '名称不能为空').optional(),
  description: z.string().optional(),
  provider: z.string().min(1, '提供商不能为空').optional(),
  modelID: z.string().min(1, '模型ID不能为空').optional(),
  config: z.record(z.any()).optional(),
})

export type CreateModelInput = z.infer<typeof createModelInputSchema>
export type UpdateModelInput = z.infer<typeof updateModelInputSchema>