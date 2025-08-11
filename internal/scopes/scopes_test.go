package scopes

import (
	"testing"

	"github.com/samber/lo"
)

func TestAllScopes(t *testing.T) {
	scopes := AllScopes()

	if len(scopes) == 0 {
		t.Error("AllScopes should return non-empty slice")
	}

	// Check that all expected scopes are present
	expectedScopes := []Scope{
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
		ScopeReadJobs,
		ScopeWriteJobs,
		ScopeReadDashboard,
		ScopeReadSettings,
		ScopeWriteSettings,
	}

	for _, expectedScope := range expectedScopes {
		if !lo.Contains(scopes, expectedScope) {
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
	allScopes := AllScopes()
	if len(scopes) != len(allScopes) {
		t.Errorf("expected %d scopes, got %d", len(allScopes), len(scopes))
	}
}

func TestScopeDescriptions(t *testing.T) {
	descriptions := ScopeDescriptions()

	if len(descriptions) == 0 {
		t.Error("ScopeDescriptions should return non-empty map")
	}

	// Check that all scopes have descriptions
	allScopes := AllScopes()
	for _, scope := range allScopes {
		if description, exists := descriptions[scope]; !exists {
			t.Errorf("scope %s missing description", scope)
		} else if description == "" {
			t.Errorf("scope %s has empty description", scope)
		}
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
	scopes := map[string]Scope{
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
		"ScopeReadJobs":      ScopeReadJobs,
		"ScopeWriteJobs":     ScopeWriteJobs,
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
	var scope Scope = "test_scope"

	if string(scope) != "test_scope" {
		t.Errorf("expected 'test_scope', got %s", string(scope))
	}
}
