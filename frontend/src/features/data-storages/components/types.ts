export interface DataStorageFormData {
  name: string;
  description: string;
  type: 'database' | 'fs' | 's3' | 'gcs' | 'webdav';
  directory: string;
  // S3 fields
  s3BucketName: string;
  s3Endpoint: string;
  s3Region: string;
  s3AccessKey: string;
  s3SecretKey: string;
  s3PathStyle: boolean;
  // GCS fields
  gcsBucketName: string;
  gcsCredential: string;
  // WebDAV fields
  webdavURL: string;
  webdavUsername: string;
  webdavPassword: string;
  webdavPath: string;
  webdavInsecureSkipTLS: boolean;
}
