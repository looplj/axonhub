import { z } from 'zod'

// Re-export all schemas from the global roles feature
export {
  roleSchema,
  roleEdgeSchema,
  pageInfoSchema,
  roleConnectionSchema,
  updateRoleInputSchema,
  roleListSchema,
  type Role,
  type RoleConnection,
  type UpdateRoleInput,
  type RoleList,
} from '@/features/roles/data/schema'

// Project-specific Create Role Input - extends base schema with projectID
export const createRoleInputSchema = z.object({
  projectID: z.string().min(1, '项目ID不能为空'),
  code: z.string().min(1, '角色代码不能为空').regex(/^[a-zA-Z0-9_]+$/, '角色代码只能包含字母、数字和下划线'),
  name: z.string().min(1, '角色名称不能为空'),
  scopes: z.array(z.string()).min(1, '至少需要选择一个权限'),
})
export type CreateRoleInput = z.infer<typeof createRoleInputSchema>