package scopes

import (
	"testing"
)

func TestAllScopes(t *testing.T) {
	scopes := AllScopes(nil)

	if len(scopes) == 0 {
		t.Error("AllScopes should return non-empty slice")
	}

	// Check that all expected scopes are present
	expectedScopes := []ScopeSlug{
		ScopeReadChannels,
		ScopeWriteChannels,
		ScopeReadUsers,
		ScopeWriteUsers,
		ScopeReadRoles,
		ScopeWriteRoles,
		ScopeReadAPIKeys,
		ScopeWriteAPIKeys,
		ScopeReadRequests,
		ScopeWriteRequests,
		ScopeReadDashboard,
		ScopeReadSettings,
		ScopeWriteSettings,
	}

	for _, expectedScope := range expectedScopes {
		found := false

		for _, scope := range scopes {
			if scope.Slug == expectedScope {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("expected scope %s not found in AllScopes", expectedScope)
		}
	}
}

func TestAllScopesAsStrings(t *testing.T) {
	scopes := AllScopesAsStrings()

	if len(scopes) == 0 {
		t.Error("AllScopesAsStrings should return non-empty slice")
	}

	// Check that all scopes are strings
	for _, scope := range scopes {
		if scope == "" {
			t.Error("scope string should not be empty")
		}
	}

	// Check that the count matches AllScopes
	allScopes := AllScopes(nil)
	if len(scopes) != len(allScopes) {
		t.Errorf("expected %d scopes, got %d", len(allScopes), len(scopes))
	}
}

func TestIsValidScope(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		expected bool
	}{
		{
			name:     "valid scope - read channels",
			scope:    string(ScopeReadChannels),
			expected: true,
		},
		{
			name:     "valid scope - write users",
			scope:    string(ScopeWriteUsers),
			expected: true,
		},
		{
			name:     "invalid scope - empty string",
			scope:    "",
			expected: false,
		},
		{
			name:     "invalid scope - random string",
			scope:    "invalid_scope",
			expected: false,
		},
		{
			name:     "invalid scope - partial match",
			scope:    "read_",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidScope(tt.scope)
			if result != tt.expected {
				t.Errorf("IsValidScope(%s) = %v, expected %v", tt.scope, result, tt.expected)
			}
		})
	}
}

func TestScopeConstants(t *testing.T) {
	// Test that scope constants are not empty
	scopes := map[string]ScopeSlug{
		"ScopeReadChannels":  ScopeReadChannels,
		"ScopeWriteChannels": ScopeWriteChannels,
		"ScopeReadUsers":     ScopeReadUsers,
		"ScopeWriteUsers":    ScopeWriteUsers,
		"ScopeReadRoles":     ScopeReadRoles,
		"ScopeWriteRoles":    ScopeWriteRoles,
		"ScopeReadAPIKeys":   ScopeReadAPIKeys,
		"ScopeWriteAPIKeys":  ScopeWriteAPIKeys,
		"ScopeReadRequests":  ScopeReadRequests,
		"ScopeWriteRequests": ScopeWriteRequests,
		"ScopeReadDashboard": ScopeReadDashboard,
		"ScopeReadSettings":  ScopeReadSettings,
		"ScopeWriteSettings": ScopeWriteSettings,
	}

	for name, scope := range scopes {
		if scope == "" {
			t.Errorf("scope constant %s should not be empty", name)
		}
	}
}

func TestScopeType(t *testing.T) {
	// Test that Scope type works correctly
	var scope ScopeSlug = "test_scope"

	if string(scope) != "test_scope" {
		t.Errorf("expected 'test_scope', got %s", string(scope))
	}
}
