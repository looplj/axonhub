import { z } from 'zod'

// Project schema based on GraphQL schema
export const projectSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  name: z.string(),
  description: z.string(),
  status: z.enum(['active', 'archived']),
})
export type Project = z.infer<typeof projectSchema>

// Project Connection schema for GraphQL pagination
export const projectEdgeSchema = z.object({
  node: projectSchema,
  cursor: z.string(),
})

export const pageInfoSchema = z.object({
  hasNextPage: z.boolean(),
  hasPreviousPage: z.boolean(),
  startCursor: z.string().nullable(),
  endCursor: z.string().nullable(),
})

export const projectConnectionSchema = z.object({
  edges: z.array(projectEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
})
export type ProjectConnection = z.infer<typeof projectConnectionSchema>

// Create Project Input
export const createProjectInputSchema = z.object({
  name: z.string().min(1, '项目名称不能为空'),
  description: z.string().optional(),
})
export type CreateProjectInput = z.infer<typeof createProjectInputSchema>

// Update Project Input
export const updateProjectInputSchema = z.object({
  name: z.string().min(1, '项目名称不能为空'),
})
export type UpdateProjectInput = z.infer<typeof updateProjectInputSchema>

// Project List schema for table display
export const projectListSchema = z.array(projectSchema)
export type ProjectList = z.infer<typeof projectListSchema>
