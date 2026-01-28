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

	// WebDAV is the webdav data storage.
	WebDAV *WebDAV `json:"webdav"`
}

type S3 struct {
	BucketName string `json:"bucketName"`
	Endpoint   string `json:"endpoint"`
	Region     string `json:"region"`
	AccessKey  string `json:"accessKey"`
	SecretKey  string `json:"secretKey"`
}

type GCS struct {
	BucketName string `json:"bucketName"`
	Credential string `json:"credential"`
}

type WebDAV struct {
	URL             string `json:"url"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	InsecureSkipTLS bool   `json:"insecure_skip_tls"`
	Path            string `json:"path"`
}
