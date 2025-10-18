import { z } from 'zod'

// Role schema based on GraphQL schema
export const roleSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  name: z.string(),
  scopes: z.array(z.string()),
})
export type Role = z.infer<typeof roleSchema>

// Role Connection schema for GraphQL pagination
export const roleEdgeSchema = z.object({
  node: roleSchema,
  cursor: z.string(),
})

export const pageInfoSchema = z.object({
  hasNextPage: z.boolean(),
  hasPreviousPage: z.boolean(),
  startCursor: z.string().nullable(),
  endCursor: z.string().nullable(),
})

export const roleConnectionSchema = z.object({
  edges: z.array(roleEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
})
export type RoleConnection = z.infer<typeof roleConnectionSchema>

// Create Role Input
export const createRoleInputSchema = z.object({
  name: z.string().min(1, '角色名称不能为空'),
  scopes: z.array(z.string()).min(1, '至少需要选择一个权限'),
})
export type CreateRoleInput = z.infer<typeof createRoleInputSchema>

// Update Role Input
export const updateRoleInputSchema = z.object({
  name: z.string().min(1, '角色名称不能为空').optional(),
  scopes: z.array(z.string()).min(1, '至少需要选择一个权限').optional(),
})
export type UpdateRoleInput = z.infer<typeof updateRoleInputSchema>

// Role List schema for table display
export const roleListSchema = z.array(roleSchema)
export type RoleList = z.infer<typeof roleListSchema>
