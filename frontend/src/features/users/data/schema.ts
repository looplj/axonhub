import { z } from "zod";

export const userSchema = z.object({
  id: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
  email: z.string(),
  firstName: z.string(),
  lastName: z.string(),
  isOwner: z.boolean(),
  scopes: z.array(z.string()).nullable().optional(),
  roles: z.object({
    edges: z.array(z.object({
      node: z.object({
        id: z.string(),
        name: z.string(),
      }),
    })),
  }).optional(),
});

export const userConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: userSchema,
    })
  ),
  pageInfo: z.object({
    hasNextPage: z.boolean(),
    hasPreviousPage: z.boolean(),
    startCursor: z.string().nullable(),
    endCursor: z.string().nullable(),
  }),
});

export const createUserInputSchema = z.object({
  createdAt: z.string().optional(),
  updatedAt: z.string().optional(),
  email: z.string().email("Invalid email address"),
  firstName: z.string().optional(),
  lastName: z.string().optional(),
  isOwner: z.boolean().optional(),
  scopes: z.array(z.string()).optional(),
  roleIDs: z.array(z.string()).optional(),
});

export const updateUserInputSchema = z.object({
  updatedAt: z.string().optional(),
  email: z.string().email("Invalid email address").optional(),
  firstName: z.string().optional(),
  lastName: z.string().optional(),
  isOwner: z.boolean().optional(),
  scopes: z.array(z.string()).optional(),
  appendScopes: z.array(z.string()).optional(),
  clearScopes: z.boolean().optional(),
  addRoleIDs: z.array(z.string()).optional(),
  removeRoleIDs: z.array(z.string()).optional(),
  clearRoles: z.boolean().optional(),
});

export type User = z.infer<typeof userSchema>;
export type UserConnection = z.infer<typeof userConnectionSchema>;
export type CreateUserInput = z.infer<typeof createUserInputSchema>;
export type UpdateUserInput = z.infer<typeof updateUserInputSchema>;

// User List schema for table display
export const userListSchema = z.array(userSchema)
export type UserList = z.infer<typeof userListSchema>
