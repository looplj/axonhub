package objects

type ModelAuditStatus string

const (
	ModelAuditMatched     ModelAuditStatus = "matched"
	ModelAuditMismatched  ModelAuditStatus = "mismatched"
	ModelAuditUnknown     ModelAuditStatus = "unknown"
	ModelAuditConflicting ModelAuditStatus = "conflicting"
)

// RequestModelAudit summarizes the complete execution set, independently of UI pagination.
type RequestModelAudit struct {
	Status              ModelAuditStatus `json:"status"`
	MatchedUpstreamIds  []string         `json:"matchedUpstreamIds"`
	UpstreamModelIds    []string         `json:"upstreamModelIds"`
	MismatchedModelIds  []string         `json:"mismatchedModelIds"`
	ConflictingModelIds []string         `json:"conflictingModelIds"`
	UnknownCount        int              `json:"unknownCount"`
	ComparedCount       int              `json:"comparedCount"`
	ConflictCount       int              `json:"conflictCount"`
}
