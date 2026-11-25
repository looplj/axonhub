// 职责: 校验 Vertex 混合凭证并为同一快照解析认证和资源地址。
// 关联文件:
//
//	internal/server/biz/channel_llm.go
//	internal/objects/channel.go
//	llm/transformer/gemini/outbound.go
//
// 注意事项: Token source 仅属于当前快照，固定 Google token endpoint，不捕获保存请求的 context。
package biz

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/gemini"
)

const vertexOAuthEndpoint = "https://oauth2.googleapis.com/token"

func parseVertexServiceAccount(credential objects.GCPCredential) (*jwt.Config, error) {
	var identity struct {
		Type    string `json:"type"`
		Email   string `json:"client_email"`
		KeyID   string `json:"private_key_id"`
		Key     string `json:"private_key"`
		Project string `json:"project_id"`
	}
	if json.Unmarshal([]byte(credential.JSONData), &identity) != nil {
		return nil, errors.New("invalid Vertex service-account JSON")
	}
	if identity.Type != "service_account" || strings.TrimSpace(identity.Email) == "" || strings.TrimSpace(identity.KeyID) == "" || strings.TrimSpace(identity.Project) == "" {
		return nil, errors.New("Vertex service_account type, client_email, private_key_id and project_id are required")
	}
	if strings.TrimSpace(credential.ProjectID) == "" || strings.TrimSpace(credential.Region) == "" || !validVertexResourceSegment(credential.ProjectID) || !validVertexResourceSegment(credential.Region) {
		return nil, errors.New("valid Vertex service-account target project ID and region are required")
	}
	block, rest := pem.Decode([]byte(identity.Key))
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, errors.New("invalid Vertex service-account RSA private key")
	}
	var key *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, _ = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			key, _ = parsed.(*rsa.PrivateKey)
		}
	}
	if key == nil || key.Validate() != nil {
		return nil, errors.New("invalid Vertex service-account RSA private key")
	}
	return &jwt.Config{
		Email: identity.Email, PrivateKeyID: identity.KeyID, PrivateKey: []byte(identity.Key),
		Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"}, TokenURL: vertexOAuthEndpoint,
	}, nil
}

func validateVertexServiceAccounts(channelType channel.Type, credentials objects.ChannelCredentials) error {
	if channelType != channel.TypeGeminiVertex {
		if len(credentials.VertexServiceAccounts) > 0 {
			return errors.New("vertex service accounts are only supported by gemini_vertex channels")
		}
		return nil
	}
	for _, key := range credentials.GetAllAPIKeys() {
		if strings.HasPrefix(strings.TrimSpace(key), "vertex-sa:") {
			return errors.New("Vertex API keys cannot use the reserved credential reference prefix")
		}
	}
	seen := make(map[string]struct{}, len(credentials.VertexServiceAccounts))
	for i, credential := range credentials.VertexServiceAccounts {
		if _, err := parseVertexServiceAccount(credential); err != nil {
			return fmt.Errorf("Vertex service account %d: %w", i+1, err)
		}
		ref := credential.CredentialRef()
		if _, exists := seen[ref]; exists {
			return errors.New("duplicate Vertex service-account credential reference")
		}
		seen[ref] = struct{}{}
	}
	return nil
}

func validateVertexCredentialEndpoints(channelType channel.Type, credentials objects.ChannelCredentials, endpoints []objects.ChannelEndpoint) error {
	if channelType != channel.TypeGeminiVertex || len(credentials.VertexServiceAccounts) == 0 {
		return nil
	}
	for _, endpoint := range endpoints {
		if endpoint.APIFormat != string(llm.APIFormatGeminiContents) && endpoint.APIFormat != string(llm.APIFormatGeminiEmbedding) {
			return errors.New("Vertex service-account credentials require Gemini endpoints")
		}
	}
	return nil
}

func newVertexOutbound(ch *Channel, baseURL, endpointPath string) (transformer.Outbound, error) {
	if primary, ok := ch.Outbound.(*gemini.OutboundTransformer); ok && primary.CredentialResolver() != nil {
		resolve := primary.CredentialResolver()
		return gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL: baseURL, EndpointPath: endpointPath, PlatformType: gemini.PlatformVertex,
			CredentialResolver: func(ctx context.Context) (string, *httpclient.AuthConfig, error) {
				root, credentials, err := resolve(ctx)
				if err != nil {
					return "", nil, err
				}
				_, bound := ch.Credentials.VertexAPIKey(credentials.APIKey)
				endpointBaseURL := baseURL
				if bound || credentials.Type == "bearer" {
					endpointBaseURL = vertexServiceBaseURL(baseURL) + strings.TrimPrefix(root, vertexServiceBaseURL(ch.BaseURL))
				}
				return endpointBaseURL, credentials, nil
			},
		})
	}
	if err := ValidateVertexAPIKeys(ch.Type, ch.Credentials); err != nil {
		return nil, err
	}
	provider, err := getAPIKeyProvider(ch)
	if err != nil {
		return nil, err
	}
	sources := make(map[string]oauth2.TokenSource, len(ch.Credentials.VertexServiceAccounts))
	if len(ch.Credentials.VertexServiceAccounts) > 0 {
		client := *ch.HTTPClient.GetNativeClient()
		client.Timeout = 30 * time.Second
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		tokenContext := context.WithValue(context.Background(), oauth2.HTTPClient, &client)
		for _, credential := range ch.Credentials.VertexServiceAccounts {
			config, err := parseVertexServiceAccount(credential)
			if err != nil {
				return nil, err
			}
			sources[credential.CredentialRef()] = config.TokenSource(tokenContext)
		}
	}
	return gemini.NewOutboundTransformerWithConfig(gemini.Config{
		BaseURL: baseURL, EndpointPath: endpointPath, PlatformType: gemini.PlatformVertex,
		CredentialResolver: func(ctx context.Context) (string, *httpclient.AuthConfig, error) {
			ref := provider.Get(ctx)
			if ref == "" {
				return "", nil, errors.New("no enabled Vertex credential")
			}
			if credential, ok := ch.Credentials.VertexServiceAccount(ref); ok {
				token, err := sources[ref].Token()
				if err != nil {
					// OAuth errors may contain the response body or signed assertion.
					return "", nil, &VertexCredentialError{CredentialRef: ref}
				}
				return vertexResourceURL(baseURL, credential.ProjectID, credential.Region), &httpclient.AuthConfig{Type: "bearer", APIKey: token.AccessToken}, nil
			}
			if strings.HasPrefix(ref, "vertex-sa:") {
				return "", nil, errors.New("Vertex service-account credential reference not found")
			}
			root := baseURL
			if credential, ok := ch.Credentials.VertexAPIKey(ref); ok {
				root = vertexResourceURL(baseURL, credential.ProjectID, credential.Region)
			}
			return root, &httpclient.AuthConfig{Type: "api_key", APIKey: ref, HeaderKey: "x-goog-api-key"}, nil
		},
	})
}

func vertexResourceURL(baseURL, projectID, region string) string {
	return fmt.Sprintf("%s/v1/projects/%s/locations/%s", vertexServiceBaseURL(baseURL), strings.TrimSpace(projectID), strings.TrimSpace(region))
}
