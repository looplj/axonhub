// Package events is the in-process plugin event bus.
// Publishers own when an event is built. This package does not import biz or plugin.
package events

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"time"

	"github.com/looplj/axonhub/internal/pkg/xsecret"
)

const (
	TypeChannelAutoEnabled        = "channel.auto_enabled"
	TypeChannelCredentialDisabled = "channel.credential_disabled"
	TypeChannelCredentialEnabled  = "channel.credential_enabled"
	TypeRequestAttemptFailed      = "request.attempt_failed"
	TypeRequestFailover           = "request.failover"
	TypeRequestFailed             = "request.failed"
	TypeRequestCompleted          = "request.completed"
	//nolint:gosec // false positive.
	TypeAPIKeyQuotaExceeded = "api_key.quota_exceeded"
)

const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// OAuthFingerprint is the fingerprint and mask for OAuth credentials.
// OAuth has no key string. Callers set both CredentialRef.Fingerprint and
// CredentialRef.Masked to OAuthFingerprint and do not call Fingerprint or Mask.
const OAuthFingerprint = "oauth"

var openEventTypes = []string{
	TypeChannelAutoEnabled,
	TypeChannelCredentialDisabled,
	TypeChannelCredentialEnabled,
	TypeRequestAttemptFailed,
	TypeRequestFailover,
	TypeRequestFailed,
	TypeRequestCompleted,
	TypeAPIKeyQuotaExceeded,
}

// OpenEventTypes returns the event names plugins may subscribe to, in publish order.
// The result is a copy.
func OpenEventTypes() []string {
	out := make([]string, len(openEventTypes))
	copy(out, openEventTypes)
	return out
}

// Event is one bus message. Data keys are already camelCase.
// Credential plaintext is never stored on Event.
type Event struct {
	ID         string
	Type       string
	Severity   string
	OccurredAt time.Time
	Subject    Subject
	Data       map[string]any
	Origin     string
	Depth      int
}

// Subject is the set of objects an event is about.
// Nil refs are omitted from the JS view.
type Subject struct {
	Channel    *ChannelRef
	Credential *CredentialRef
	Model      string
	APIKey     *APIKeyRef
	Project    *ProjectRef
	Request    *RequestRef
}

// ChannelRef identifies a channel without its credentials.
type ChannelRef struct {
	ID      int
	Name    string
	Type    string
	BaseURL string
	Status  string
}

// CredentialRef is a non-secret credential identity.
// Fingerprint is Fingerprint(key) and Masked is Mask(key), except OAuth,
// where both fields are OAuthFingerprint.
type CredentialRef struct {
	Fingerprint string
	Masked      string
}

// APIKeyRef identifies an API key by id and name.
type APIKeyRef struct {
	ID   int
	Name string
}

// ProjectRef identifies a project.
type ProjectRef struct {
	ID int
}

// RequestRef identifies a gateway request.
type RequestRef struct {
	ID int
}

// Fingerprint returns hex(sha256(key))[:12].
func Fingerprint(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:12]
}

// Mask uses the same credential display policy as channel testing.
func Mask(key string) string {
	return xsecret.MaskAPIKey(key)
}

// Map is the JS view of the event. Field names are camelCase.
// OccurredAt is RFC3339 UTC. Nil subject refs and an empty model are omitted.
// Data keys are copied as given. The map never adds credential plaintext.
func (e Event) Map() map[string]any {
	subject := map[string]any{}
	if e.Subject.Channel != nil {
		channel := e.Subject.Channel
		subject["channel"] = map[string]any{
			"id":      channel.ID,
			"name":    channel.Name,
			"type":    channel.Type,
			"baseUrl": channel.BaseURL,
			"status":  channel.Status,
		}
	}
	if e.Subject.Credential != nil {
		credential := e.Subject.Credential
		subject["credential"] = map[string]any{
			"fingerprint": credential.Fingerprint,
			"masked":      credential.Masked,
		}
	}
	if e.Subject.Model != "" {
		subject["model"] = e.Subject.Model
	}
	if e.Subject.APIKey != nil {
		subject["apiKey"] = map[string]any{
			"id":   e.Subject.APIKey.ID,
			"name": e.Subject.APIKey.Name,
		}
	}
	if e.Subject.Project != nil {
		subject["project"] = map[string]any{
			"id": e.Subject.Project.ID,
		}
	}
	if e.Subject.Request != nil {
		subject["request"] = map[string]any{
			"id": e.Subject.Request.ID,
		}
	}

	data := maps.Clone(e.Data)
	if data == nil {
		data = map[string]any{}
	}

	return map[string]any{
		"id":         e.ID,
		"type":       e.Type,
		"severity":   e.Severity,
		"occurredAt": e.OccurredAt.UTC().Format(time.RFC3339),
		"subject":    subject,
		"data":       data,
		"origin":     e.Origin,
		"depth":      e.Depth,
	}
}
