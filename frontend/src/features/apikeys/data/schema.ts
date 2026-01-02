import { z } from 'zod'
import { userSchema } from '@/features/users/data/schema'
import { pageInfoSchema } from '@/gql/pagination'

// API Key Type
export const apiKeyTypeSchema = z.enum(['user', 'service_account'])
export type ApiKeyType = z.infer<typeof apiKeyTypeSchema>

// API Key Status
export const apiKeyStatusSchema = z.enum(['enabled', 'disabled', 'archived'])
export type ApiKeyStatus = z.infer<typeof apiKeyStatusSchema>

// API Key schema based on GraphQL schema
export const apiKeySchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  user: userSchema.partial().optional(),
  key: z.string(),
  name: z.string(),
  type: apiKeyTypeSchema,
  status: apiKeyStatusSchema,
  scopes: z.array(z.string()).optional().nullable(),
  // Optional profiles for detailed view (may be omitted in list queries)
  profiles: z
    .object({
      activeProfile: z.string(),
      profiles: z.array(
        z.object({
          name: z.string(),
          modelMappings: z.array(
            z.object({
              from: z.string(),
              to: z.string(),
            })
          ),
          channelIDs: z.array(z.number()).optional().nullable(),
          channelTags: z.array(z.string()).optional().nullable(),
          modelIDs: z.array(z.string()).optional().nullable(),
        })
      ).nullable(),
    })
    .optional()
    .nullable(),
})
export type ApiKey = z.infer<typeof apiKeySchema>

// API Key Connection schema for GraphQL pagination
export const apiKeyEdgeSchema = z.object({
  node: apiKeySchema,
  cursor: z.string(),
})

export const apiKeyConnectionSchema = z.object({
  edges: z.array(apiKeyEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
})
export type ApiKeyConnection = z.infer<typeof apiKeyConnectionSchema>

// Create API Key Input - factory function for i18n support
export const createApiKeyInputSchemaFactory = (t: (key: string) => string) => z.object({
  name: z.string().min(1, t('apikeys.validation.nameRequired')),
  type: apiKeyTypeSchema.optional(),
  scopes: z.array(z.string()).optional(),
  projectID: z.number().optional(),
})

// Default schema for backward compatibility
export const createApiKeyInputSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  type: apiKeyTypeSchema.optional(),
  scopes: z.array(z.string()).optional(),
  projectID: z.number().optional(),
})
export type CreateApiKeyInput = z.infer<typeof createApiKeyInputSchema>

// Update API Key Input - factory function for i18n support
export const updateApiKeyInputSchemaFactory = (t: (key: string) => string) => z.object({
  name: z.string().min(1, t('apikeys.validation.nameRequired')).optional(),
  scopes: z.array(z.string()).optional(),
})

// Default schema for backward compatibility
export const updateApiKeyInputSchema = z.object({
  name: z.string().min(1, 'Name is required').optional(),
  scopes: z.array(z.string()).optional(),
})
export type UpdateApiKeyInput = z.infer<typeof updateApiKeyInputSchema>

// Model Mapping schema
export const modelMappingSchema = z.object({
  from: z.string(),
  to: z.string(),
})
export type ModelMapping = z.infer<typeof modelMappingSchema>

// API Key Profile schema
export const apiKeyProfileSchema = z.object({
  name: z.string(),
  modelMappings: z.array(modelMappingSchema),
  channelIDs: z.array(z.number()).optional().nullable(),
  channelTags: z.array(z.string()).optional().nullable(),
  modelIDs: z.array(z.string()).optional().nullable(),
})
export type ApiKeyProfile = z.infer<typeof apiKeyProfileSchema>

// API Key Profiles schema
export const apiKeyProfilesSchema = z.object({
  activeProfile: z.string(),
  profiles: z.array(apiKeyProfileSchema),
})
export type ApiKeyProfiles = z.infer<typeof apiKeyProfilesSchema>

// Update API Key Profiles Input schema - factory function for i18n support
export const updateApiKeyProfilesInputSchemaFactory = (t: (key: string) => string) => z.object({
  activeProfile: z.string().min(1, t('apikeys.validation.activeProfileRequired')),
  profiles: z.array(z.object({
    name: z.string().min(1, t('apikeys.validation.profileNameRequired')),
    modelMappings: z.array(z.object({
      from: z.string().min(1, t('apikeys.validation.sourceModelRequired')),
      to: z.string().min(1, t('apikeys.validation.targetModelRequired')),
    })),
    channelIDs: z.array(z.number()).optional().nullable(),
    channelTags: z.array(z.string()).optional().nullable(),
    modelIDs: z.array(z.string()).optional().nullable(),
  })).min(1, t('apikeys.validation.atLeastOneProfile')),
}).refine(
  (data) => data.profiles.some(profile => profile.name === data.activeProfile),
  {
    message: t('apikeys.validation.activeProfileMustExist'),
    path: ['activeProfile']
  }
).refine(
  (data) => {
    const names = data.profiles.map(p => p.name.trim().toLowerCase())
    return names.length === new Set(names).size
  },
  {
    message: t('apikeys.validation.duplicateProfileName'),
    path: ['profiles']
  }
)

// Default schema for backward compatibility
export const updateApiKeyProfilesInputSchema = z.object({
  activeProfile: z.string(),
  profiles: z.array(z.object({
    name: z.string().min(1, 'Profile name is required'),
    modelMappings: z.array(z.object({
      from: z.string().min(1, 'Source model is required'),
      to: z.string().min(1, 'Target model is required'),
    })),
    channelIDs: z.array(z.number()).optional().nullable(),
    channelTags: z.array(z.string()).optional().nullable(),
    modelIDs: z.array(z.string()).optional().nullable(),
  })),
})
export type UpdateApiKeyProfilesInput = z.infer<typeof updateApiKeyProfilesInputSchema>
