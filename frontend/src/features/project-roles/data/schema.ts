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

// Project-specific Create Role Input - factory function for i18n support
export const createRoleInputSchemaFactory = (t: (key: string) => string) => z.object({
  projectID: z.string().min(1, t('roles.validation.projectIdRequired')),
  name: z.string().min(1, t('roles.validation.nameRequired')),
  scopes: z.array(z.string()).min(1, t('roles.validation.scopesRequired')),
})

// Default schema for backward compatibility - extends base schema with projectID
export const createRoleInputSchema = z.object({
  projectID: z.string().min(1, 'Project ID is required'),
  name: z.string().min(1, 'Role name is required'),
  scopes: z.array(z.string()).min(1, 'At least one permission is required'),
})
export type CreateRoleInput = z.infer<typeof createRoleInputSchema>