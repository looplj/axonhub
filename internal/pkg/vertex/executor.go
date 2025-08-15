package vertex

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

// Executor implements a Vertex AI-specific executor that handles Google Cloud authentication
// and request transformation for Google Vertex AI API calls.
type Executor struct {
	// region is the Google Cloud region for Vertex AI
	region string
	// projectID is the Google Cloud project ID
	projectID string
	// HTTP client with Google Cloud authentication
	client *httpclient.HttpClient
	// baseURL for Vertex AI API
	baseURL string
}

// NewExecutor creates a new Vertex AI executor with Google Cloud credentials.
func NewExecutor(region, projectID string, creds *google.Credentials) (*Executor, error) {
	if region == "" {
		return nil, fmt.Errorf("region must be provided")
	}

	if projectID == "" {
		return nil, fmt.Errorf("projectID must be provided")
	}

	if creds == nil {
		return nil, fmt.Errorf("credentials must be provided")
	}

	// Create HTTP client with Google Cloud authentication
	client, _, err := transport.NewHTTPClient(context.Background(), option.WithTokenSource(creds.TokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Determine base URL based on region
	var baseURL string
	if region == "global" {
		baseURL = "https://aiplatform.googleapis.com/"
	} else {
		baseURL = fmt.Sprintf("https://%s-aiplatform.googleapis.com/", region)
	}

	return &Executor{
		region:    region,
		projectID: projectID,
		client:    httpclient.NewHttpClientWithClient(client),
		baseURL:   baseURL,
	}, nil
}

// NewExecutorFromJSON creates a new Vertex AI executor from JSON credentials.
func NewExecutorFromJSON(region, projectID string, jsonData string) (*Executor, error) {
	creds, err := google.CredentialsFromJSON(context.Background(), []byte(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return NewExecutor(region, projectID, creds)
}

// Do executes a HTTP request using the Vertex AI client.
func (e *Executor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	// Transform the request for Vertex AI
	transformedReq, err := e.transformRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	return e.client.Do(ctx, transformedReq)
}

// DoStream executes a streaming HTTP request using the Vertex AI client.
func (e *Executor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	// Transform the request for Vertex AI
	transformedReq, err := e.transformRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	return e.client.DoStream(ctx, transformedReq)
}

// transformRequest transforms the request for Vertex AI API.
func (e *Executor) transformRequest(request *httpclient.Request) (*httpclient.Request, error) {
	// Create a copy of the request
	transformed := &httpclient.Request{
		Method:    request.Method,
		URL:       request.URL,
		Headers:   request.Headers,
		Body:      request.Body,
		Auth:      nil, // Remove auth as it's handled by the HTTP client
		RequestID: request.RequestID,
	}

	// Process request body for Vertex AI compatibility
	body := request.Body
	if body != nil {
		// Add anthropic_version if not present
		if !gjson.GetBytes(body, "anthropic_version").Exists() {
			body, _ = sjson.SetBytes(body, "anthropic_version", "vertex-2023-10-16")
		}

		// Transform URL path for Vertex AI
		if request.URL == "/v1/messages" && request.Method == http.MethodPost {
			model := gjson.GetBytes(body, "model").String()
			if model == "" {
				return nil, fmt.Errorf("model field is required")
			}

			// Remove model from body as it's part of the URL in Vertex AI
			body, _ = sjson.DeleteBytes(body, "model")

			// Determine if this is a streaming request
			stream := gjson.GetBytes(body, "stream").Bool()

			specifier := "rawPredict"
			if stream {
				specifier = "streamRawPredict"
			}

			transformed.URL = fmt.Sprintf("%sv1/projects/%s/locations/%s/publishers/anthropic/models/%s:%s",
				e.baseURL, e.projectID, e.region, model, specifier)
		} else if request.URL == "/v1/messages/count_tokens" && request.Method == http.MethodPost {
			transformed.URL = fmt.Sprintf("%sv1/projects/%s/locations/%s/publishers/anthropic/models/count-tokens:rawPredict",
				e.baseURL, e.projectID, e.region)
		} else {
			// For other endpoints, just prepend the base URL
			transformed.URL = e.baseURL + request.URL
		}
	}

	// Update the body with modifications
	transformed.Body = body

	return transformed, nil
}

// Ensure Executor implements the pipeline.Executor interface.
var _ pipeline.Executor = (*Executor)(nil)
