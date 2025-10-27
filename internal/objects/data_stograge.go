package objects

type DataStorageSettings struct {
	// DSN is the database data storage.
	DSN *string `json:"dsn"`

	// Directory is the directory of the fs data storage.
	Directory *string `json:"directory"`

	// S3 is the s3 data storage.
	S3 *S3 `json:"s3"`

	// GCS is the gcs data storage.
	GCS *GCS `json:"gcs"`
}

type S3 struct {
	BucketName string `json:"bucketName"`
	Endpoint   string `json:"endpoint"`
	AccessKey  string `json:"accessKey"`
	SecretKey  string `json:"secretKey"`
}

type GCS struct {
	BucketName string        `json:"bucketName"`
	Credential GCPCredential `json:"credential"`
}
