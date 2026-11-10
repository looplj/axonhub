package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/looplj/axonhub/internal/objects"
)

const gcpCloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

func hasGCPCredential(credential *objects.GCPCredential) bool {
	return credential != nil && (strings.TrimSpace(credential.ProjectID) != "" || strings.TrimSpace(credential.Region) != "" || strings.TrimSpace(credential.JSONData) != "")
}

func gcpTokenSource(ctx context.Context, credential *objects.GCPCredential) (oauth2.TokenSource, error) {
	if credential == nil {
		return nil, fmt.Errorf("gcp credentials are required")
	}

	var (
		credentials *google.Credentials
		err         error
	)
	if jsonData := strings.TrimSpace(credential.JSONData); jsonData != "" {
		if err := validateServiceAccountJSON(jsonData); err != nil {
			return nil, err
		}
		credentials, err = google.CredentialsFromJSON(ctx, []byte(jsonData), gcpCloudPlatformScope)
	} else {
		credentials, err = google.FindDefaultCredentials(ctx, gcpCloudPlatformScope)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load GCP credentials: %w", err)
	}
	if credentials.TokenSource == nil {
		return nil, fmt.Errorf("GCP credentials do not provide a token source")
	}

	return credentials.TokenSource, nil
}

func validateServiceAccountJSON(jsonData string) error {
	var document struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(jsonData), &document); err != nil {
		return fmt.Errorf("invalid GCP credentials JSON: %w", err)
	}
	if document.Type != "service_account" {
		return fmt.Errorf("GCP credentials JSON must have type service_account")
	}
	return nil
}

func validateGCPJSON(credential *objects.GCPCredential) error {
	if credential == nil || strings.TrimSpace(credential.JSONData) == "" {
		return nil
	}
	if err := validateServiceAccountJSON(strings.TrimSpace(credential.JSONData)); err != nil {
		return err
	}
	if _, err := google.CredentialsFromJSON(context.Background(), []byte(credential.JSONData), gcpCloudPlatformScope); err != nil {
		return fmt.Errorf("invalid GCP credentials JSON: %w", err)
	}
	return nil
}

func vertexProjectBaseURL(baseURL string, credential *objects.GCPCredential) string {
	return fmt.Sprintf(
		"%s/v1/projects/%s/locations/%s",
		vertexServiceBaseURL(baseURL),
		strings.TrimSpace(credential.ProjectID),
		strings.TrimSpace(credential.Region),
	)
}
