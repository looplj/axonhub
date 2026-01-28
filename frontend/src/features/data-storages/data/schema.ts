import { z } from 'zod';
import { pageInfoSchema } from '@/gql/pagination';

// S3 Settings schema (without sensitive credentials for list queries)
export const s3SettingsSchema = z.object({
  bucketName: z.string(),
  endpoint: z.string().optional(),
  region: z.string(),
  accessKey: z.string().optional(),
  secretKey: z.string().optional(),
});
export type S3Settings = z.infer<typeof s3SettingsSchema>;

// GCS Settings schema (without credential for list queries)
export const gcsSettingsSchema = z.object({
  bucketName: z.string(),
  credential: z.string().optional(),
});
export type GCSSettings = z.infer<typeof gcsSettingsSchema>;

// WebDAV Settings schema
export const webdavSettingsSchema = z.object({
  url: z.string(),
  username: z.string().optional(),
  password: z.string().optional(),
  insecure_skip_tls: z.boolean().optional(),
  path: z.string().optional(),
});
export type WebDAVSettings = z.infer<typeof webdavSettingsSchema>;

// Data Storage Settings schema
export const dataStorageSettingsSchema = z.object({
  directory: z.string().optional().nullable(),
  s3: s3SettingsSchema.optional().nullable(),
  gcs: gcsSettingsSchema.optional().nullable(),
  webdav: webdavSettingsSchema.optional().nullable(),
});
export type DataStorageSettings = z.infer<typeof dataStorageSettingsSchema>;

// Data Storage schema
export const dataStorageSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  type: z.enum(['database', 'fs', 's3', 'gcs', 'webdav']),
  primary: z.boolean(),
  status: z.enum(['active', 'archived']),
  settings: dataStorageSettingsSchema,
  createdAt: z.string(),
  updatedAt: z.string(),
});
export type DataStorage = z.infer<typeof dataStorageSchema>;

// Data Storage Edge schema
export const dataStorageEdgeSchema = z.object({
  node: dataStorageSchema,
});
export type DataStorageEdge = z.infer<typeof dataStorageEdgeSchema>;

// Data Storages Connection schema (for GraphQL pagination)
export const dataStoragesConnectionSchema = z.object({
  edges: z.array(dataStorageEdgeSchema),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type DataStoragesConnection = z.infer<typeof dataStoragesConnectionSchema>;

// S3 Settings schema with credentials (for update operations)
export const s3SettingsWithCredentialsSchema = s3SettingsSchema.extend({
  accessKey: z.string().optional(),
  secretKey: z.string().optional(),
});
export type S3SettingsWithCredentials = z.infer<typeof s3SettingsWithCredentialsSchema>;

// GCS Settings schema with credentials (for update operations)
export const gcsSettingsWithCredentialsSchema = gcsSettingsSchema.extend({
  credential: z.string().optional(),
});
export type GCSSettingsWithCredentials = z.infer<typeof gcsSettingsWithCredentialsSchema>;

// Data Storage Settings schema with credentials (for update operations)
export const dataStorageSettingsWithCredentialsSchema = z.object({
  directory: z.string().optional().nullable(),
  s3: s3SettingsWithCredentialsSchema.optional().nullable(),
  gcs: gcsSettingsWithCredentialsSchema.optional().nullable(),
  webdav: webdavSettingsSchema.optional().nullable(),
});
export type DataStorageSettingsWithCredentials = z.infer<typeof dataStorageSettingsWithCredentialsSchema>;

// Data Storage schema with credentials (for update responses)
export const dataStorageWithCredentialsSchema = dataStorageSchema.extend({
  settings: dataStorageSettingsWithCredentialsSchema,
});
export type DataStorageWithCredentials = z.infer<typeof dataStorageWithCredentialsSchema>;

// Create Data Storage Input schema
export const createDataStorageInputSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  description: z.string().optional(),
  type: z.enum(['database', 'fs', 's3', 'gcs', 'webdav']),
  settings: dataStorageSettingsWithCredentialsSchema,
});
export type CreateDataStorageInput = z.infer<typeof createDataStorageInputSchema>;

// Update Data Storage Input schema
export const updateDataStorageInputSchema = z.object({
  name: z.string().min(1, 'Name is required').optional(),
  description: z.string().optional(),
  settings: dataStorageSettingsWithCredentialsSchema.optional(),
  status: z.enum(['active', 'archived']).optional(),
});
export type UpdateDataStorageInput = z.infer<typeof updateDataStorageInputSchema>;
