import { z } from 'zod'

// User schema based on GraphQL schema
export const userSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  email: z.string(),
  name: z.string(),
})
export type User = z.infer<typeof userSchema>

// User Connection schema for GraphQL pagination
export const userEdgeSchema = z.object({
  node: userSchema,
  cursor: z.string(),
})

export const pageInfoSchema = z.object({
  hasNextPage: z.boolean(),
  hasPreviousPage: z.boolean(),
  startCursor: z.string().nullable(),
  endCursor: z.string().nullable(),
})

export const userConnectionSchema = z.object({
  edges: z.array(userEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
})
export type UserConnection = z.infer<typeof userConnectionSchema>

// Create User Input
export const createUserInputSchema = z.object({
  email: z.string().email('请输入有效的邮箱地址'),
  name: z.string().min(1, '姓名不能为空'),
})
export type CreateUserInput = z.infer<typeof createUserInputSchema>

// Update User Input
export const updateUserInputSchema = z.object({
  name: z.string().min(1, '姓名不能为空').optional(),
})
export type UpdateUserInput = z.infer<typeof updateUserInputSchema>

// User List schema for table display
export const userListSchema = z.array(userSchema)
export type UserList = z.infer<typeof userListSchema>
