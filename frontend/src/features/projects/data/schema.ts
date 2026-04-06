import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';

export const channelTagsMatchModeSchema = z.enum(['any', 'all']);
export type ChannelTagsMatchMode = z.infer<typeof channelTagsMatchModeSchema>;

// Project Profile schema
export const projectProfileSchema = z.object({
  name: z.string(),
  channelIDs: z.array(z.number()).optional().nullable(),
  channelTags: z.array(z.string()).optional().nullable(),
  channelTagsMatchMode: channelTagsMatchModeSchema.optional().nullable(),
});
export type ProjectProfile = z.infer<typeof projectProfileSchema>;

// Project Profiles schema
export const projectProfilesSchema = z.object({
  activeProfile: z.string(),
  profiles: z.array(projectProfileSchema),
});
export type ProjectProfiles = z.infer<typeof projectProfilesSchema>;

// Project schema based on GraphQL schema
export const projectSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  name: z.string(),
  description: z.string(),
  status: z.enum(['active', 'archived']),
  profiles: projectProfilesSchema.optional().nullable(),
});
export type Project = z.infer<typeof projectSchema>;

// Project Connection schema for GraphQL pagination
export const projectEdgeSchema = z.object({
  node: projectSchema,
  cursor: z.string(),
});

export const projectConnectionSchema = z.object({
  edges: z.array(projectEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type ProjectConnection = z.infer<typeof projectConnectionSchema>;

// Create Project Input - factory function for i18n support
export const createProjectInputSchemaFactory = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, t('projects.validation.nameRequired')),
    description: z.string().optional(),
  });

// Default schema for backward compatibility
export const createProjectInputSchema = z.object({
  name: z.string().min(1, 'Project name is required'),
  description: z.string().optional(),
});
export type CreateProjectInput = z.infer<typeof createProjectInputSchema>;

// Update Project Input - factory function for i18n support
export const updateProjectInputSchemaFactory = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, t('projects.validation.nameRequired')),
    description: z.string().optional(),
  });

// Default schema for backward compatibility
export const updateProjectInputSchema = z.object({
  name: z.string().min(1, 'Project name is required'),
  description: z.string().optional(),
});
export type UpdateProjectInput = z.infer<typeof updateProjectInputSchema>;

// Project List schema for table display
export const projectListSchema = z.array(projectSchema);
export type ProjectList = z.infer<typeof projectListSchema>;

// Update Project Profiles Input schema - factory function for i18n support
export const updateProjectProfilesInputSchemaFactory = (t: (key: string) => string) =>
  z
    .object({
      activeProfile: z.string(),
      profiles: z
        .array(
          z.object({
            name: z.string().min(1, t('projects.profiles.validation.nameRequired')),
            channelIDs: z.array(z.number()).optional().nullable(),
            channelTags: z.array(z.string()).optional().nullable(),
            channelTagsMatchMode: channelTagsMatchModeSchema.optional().nullable(),
          })
        )
        .min(1, t('projects.profiles.validation.atLeastOneProfile')),
    })
    .refine(
      (data) => {
        if (!data.activeProfile) return true;
        return data.profiles.some((p) => p.name === data.activeProfile);
      },
      { message: 'Active profile must exist in the profiles list', path: ['activeProfile'] }
    )
    .refine(
      (data) => {
        const names = data.profiles.map((p) => p.name.toLowerCase().trim());
        return new Set(names).size === names.length;
      },
      { message: 'Profile names must be unique', path: ['profiles'] }
    );
export type UpdateProjectProfilesInput = z.infer<ReturnType<typeof updateProjectProfilesInputSchemaFactory>>;
