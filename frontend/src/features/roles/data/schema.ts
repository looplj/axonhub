import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';

// Role schema based on GraphQL schema
export const roleSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  name: z.string(),
  scopes: z.array(z.string()),
});
export type Role = z.infer<typeof roleSchema>;

// Role Connection schema for GraphQL pagination
export const roleEdgeSchema = z.object({
  node: roleSchema,
  cursor: z.string(),
});

export const roleConnectionSchema = z.object({
  edges: z.array(roleEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type RoleConnection = z.infer<typeof roleConnectionSchema>;

// Create Role Input - factory function for i18n support
export const createRoleInputSchemaFactory = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, t('roles.validation.nameRequired')),
    scopes: z.array(z.string()).min(1, t('roles.validation.scopesRequired')),
  });

// Default schema for backward compatibility
export const createRoleInputSchema = z.object({
  name: z.string().min(1, 'Role name is required'),
  scopes: z.array(z.string()).min(1, 'At least one permission is required'),
});
export type CreateRoleInput = z.infer<typeof createRoleInputSchema>;

// Update Role Input - factory function for i18n support
export const updateRoleInputSchemaFactory = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, t('roles.validation.nameRequired')).optional(),
    scopes: z.array(z.string()).min(1, t('roles.validation.scopesRequired')).optional(),
  });

// Default schema for backward compatibility
export const updateRoleInputSchema = z.object({
  name: z.string().min(1, 'Role name is required').optional(),
  scopes: z.array(z.string()).min(1, 'At least one permission is required').optional(),
});
export type UpdateRoleInput = z.infer<typeof updateRoleInputSchema>;

// Role List schema for table display
export const roleListSchema = z.array(roleSchema);
export type RoleList = z.infer<typeof roleListSchema>;
