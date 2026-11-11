package scopes

import (
	"context"
	"errors"
	"testing"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
)

// Mock query for testing.
type mockQuery struct{}

func TestAPIKeyQueryRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope ScopeSlug
		expectAllow   bool
	}{
		{
			name:          "no API key in context",
			ctx:           context.Background(),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name:          "nil API key in context",
			ctx:           contexts.WithAPIKey(context.Background(), nil),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name: "API key with required scope",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{"read_users", "write_users"},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   true,
		},
		{
			name: "API key without required scope",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{"read_channels", "write_channels"},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name: "API key with empty scopes",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := APIKeyScopeQueryRule(tt.requiredScope)
			err := rule.EvalQuery(tt.ctx, &mockQuery{})

			if tt.expectAllow {
				if !errors.Is(err, privacy.Allow) {
					t.Errorf("expected privacy.Allow, got %v", err)
				}
			} else {
				if errors.Is(err, privacy.Allow) {
					t.Error("expected error or deny, got privacy.Allow")
				}
			}
		})
	}
}

func TestAPIKeyScopeMutationRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope ScopeSlug
		expectAllow   bool
	}{
		{
			name:          "no API key in context",
			ctx:           context.Background(),
			requiredScope: ScopeWriteUsers,
			expectAllow:   false,
		},
		{
			name:          "nil API key in context",
			ctx:           contexts.WithAPIKey(context.Background(), nil),
			requiredScope: ScopeWriteUsers,
			expectAllow:   false,
		},
		{
			name: "API key with required scope",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{"read_users", "write_users"},
			}),
			requiredScope: ScopeWriteUsers,
			expectAllow:   true,
		},
		{
			name: "API key without required scope",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{"read_users"},
			}),
			requiredScope: ScopeWriteUsers,
			expectAllow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := APIKeyScopeMutationRule(tt.requiredScope)
			err := rule.EvalMutation(tt.ctx, &mockMutation{})

			if tt.expectAllow {
				if !errors.Is(err, privacy.Allow) {
					t.Errorf("expected privacy.Allow, got %v", err)
				}
			} else {
				if errors.Is(err, privacy.Allow) {
					t.Error("expected error or deny, got privacy.Allow")
				}
			}
		})
	}
}

// Mock mutation for testing.
type mockMutation struct {
	ent.Mutation
}

func (m *mockMutation) Op() ent.Op {
	return ent.OpCreate
}

func (m *mockMutation) Type() string {
	return "Mock"
}

// TestAPIKeyProjectScopeReadRuleAppliesProjectFilter verifies the Request
// schema policy keeps an API key inside its own project: write_requests is
// granted at project level, so the key must not read other projects' requests.
func TestAPIKeyProjectScopeReadRuleAppliesProjectFilter(t *testing.T) {
	ctx := context.Background()
	client := openScopesTestClient(t)

	seedCtx := contexts.WithUser(ctx, &ent.User{ID: 999, IsOwner: true})

	p100 := client.Project.Create().SetName("P100").SaveX(seedCtx)
	p200 := client.Project.Create().SetName("P200").SaveX(seedCtx)

	createRequest := func(projectID int, modelID string) *ent.Request {
		return client.Request.Create().
			SetProjectID(projectID).
			SetModelID(modelID).
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStatus(request.StatusCompleted).
			SaveX(seedCtx)
	}

	ownRequest := createRequest(p100.ID, "own")
	createRequest(p200.ID, "other")

	apiKeyCtx := contexts.WithAPIKey(ctx, &ent.APIKey{
		ID:        1,
		ProjectID: p100.ID,
		Type:      apikey.TypeServiceAccount,
		Scopes:    []string{string(ScopeWriteRequests)},
	})

	got, err := client.Request.Query().All(apiKeyCtx)
	if err != nil {
		t.Fatalf("query requests: %v", err)
	}

	if len(got) != 1 || got[0].ID != ownRequest.ID {
		t.Fatalf("expected only the key's own project request, got %d requests", len(got))
	}

	noScopeCtx := contexts.WithAPIKey(ctx, &ent.APIKey{
		ID:        2,
		ProjectID: p100.ID,
		Type:      apikey.TypeServiceAccount,
		Scopes:    []string{},
	})
	if _, err := client.Request.Query().All(noScopeCtx); err == nil {
		t.Fatal("expected an API key without write_requests to be denied")
	}
}

// TestAPIKeyDataStorageRequiresSystemScope verifies that request-scoped keys
// cannot read system data storage settings (DSN, credentials).
func TestAPIKeyDataStorageRequiresSystemScope(t *testing.T) {
	ctx := context.Background()
	client := openScopesTestClient(t)

	seedCtx := contexts.WithUser(ctx, &ent.User{ID: 999, IsOwner: true})

	dir := t.TempDir()
	client.DataStorage.Create().
		SetName("local").
		SetDescription("local storage").
		SetType(datastorage.TypeFs).
		SetSettings(&objects.DataStorageSettings{Directory: &dir}).
		SaveX(seedCtx)

	writeRequestsCtx := contexts.WithAPIKey(ctx, &ent.APIKey{
		ID:        1,
		ProjectID: 1,
		Type:      apikey.TypeServiceAccount,
		Scopes:    []string{string(ScopeWriteRequests)},
	})
	if _, err := client.DataStorage.Query().All(writeRequestsCtx); err == nil {
		t.Fatal("expected a key without read_data_storages to be denied")
	}

	readStoragesCtx := contexts.WithAPIKey(ctx, &ent.APIKey{
		ID:        2,
		ProjectID: 1,
		Type:      apikey.TypeServiceAccount,
		Scopes:    []string{string(ScopeReadDataStorages)},
	})

	got, err := client.DataStorage.Query().All(readStoragesCtx)
	if err != nil {
		t.Fatalf("query data storages: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected one data storage, got %d", len(got))
	}
}

func TestAPIKeyProjectScopeReadRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope ScopeSlug
		expectAllow   bool
	}{
		{
			name:          "no API key in context",
			ctx:           context.Background(),
			requiredScope: ScopeReadAPIKeys,
			expectAllow:   false,
		},
		{
			name:          "nil API key in context",
			ctx:           contexts.WithAPIKey(context.Background(), nil),
			requiredScope: ScopeReadAPIKeys,
			expectAllow:   false,
		},
		{
			name: "API key with required scope and project",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:        1,
				ProjectID: 100,
				Scopes:    []string{"read_api_keys", "write_api_keys"},
			}),
			requiredScope: ScopeReadAPIKeys,
			expectAllow:   true,
		},
		{
			name: "API key without required scope",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:        1,
				ProjectID: 100,
				Scopes:    []string{"read_channels"},
			}),
			requiredScope: ScopeReadAPIKeys,
			expectAllow:   false,
		},
		{
			name: "API key with empty scopes",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:        1,
				ProjectID: 100,
				Scopes:    []string{},
			}),
			requiredScope: ScopeReadAPIKeys,
			expectAllow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := APIKeyProjectScopeReadRule(tt.requiredScope)
			// Use real ent query that implements ProjectOwnedFilter.
			query := &ent.APIKeyQuery{}
			err := rule.EvalQuery(tt.ctx, query)

			if tt.expectAllow {
				if !errors.Is(err, privacy.Allow) {
					t.Errorf("expected privacy.Allow, got %v", err)
				}
			} else {
				if errors.Is(err, privacy.Allow) {
					t.Error("expected error or deny, got privacy.Allow")
				}
			}
		})
	}
}
