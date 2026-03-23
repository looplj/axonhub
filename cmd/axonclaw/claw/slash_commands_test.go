package claw

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildSubAgentTools(t *testing.T) {
	tests := []struct {
		name              string
		tools             map[string]bool
		wantAllowed       []string
		wantDenied        []string
		wantAllowedSorted bool
		wantDeniedSorted  bool
	}{
		{
			name:        "nil tools returns nil (allow all)",
			tools:       nil,
			wantAllowed: nil,
			wantDenied:  nil,
		},
		{
			name:        "empty tools returns empty allowed slice",
			tools:       map[string]bool{},
			wantAllowed: []string{},
			wantDenied:  nil,
		},
		{
			name:             "default allow all with specific deny",
			tools:            map[string]bool{"*": true, "SpawnAgent": false, "Bash": false},
			wantAllowed:      nil,
			wantDenied:       []string{"Bash", "SpawnAgent"},
			wantDeniedSorted: true,
		},
		{
			name:              "default deny all with specific allow",
			tools:             map[string]bool{"*": false, "Read": true, "Write": true},
			wantAllowed:       []string{"Read", "Write"},
			wantAllowedSorted: true,
			wantDenied:        nil,
		},
		{
			name:        "default deny all with no specific allow",
			tools:       map[string]bool{"*": false},
			wantAllowed: []string{},
			wantDenied:  nil,
		},
		{
			name:              "allowlist without default marker",
			tools:             map[string]bool{"Read": true, "Write": true, "Bash": false},
			wantAllowed:       []string{"Read", "Write"},
			wantAllowedSorted: true,
			wantDenied:        nil,
		},
		{
			name:        "all disabled without default marker",
			tools:       map[string]bool{"Read": false, "Write": false},
			wantAllowed: []string{},
			wantDenied:  nil,
		},
		{
			name:        "default allow all with no specific deny",
			tools:       map[string]bool{"*": true},
			wantAllowed: nil,
			wantDenied:  nil,
		},
		{
			name:             "mixed tools with default allow",
			tools:            map[string]bool{"*": true, "Read": true, "Bash": false, "Grep": false},
			wantAllowed:      nil,
			wantDenied:       []string{"Bash", "Grep"},
			wantDeniedSorted: true,
		},
		{
			name:              "mixed tools with default deny",
			tools:             map[string]bool{"*": false, "Read": true, "Write": false, "Grep": true},
			wantAllowed:       []string{"Grep", "Read"},
			wantAllowedSorted: true,
			wantDenied:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAllowed, gotDenied := buildSubAgentTools(tt.tools)

			if tt.wantAllowedSorted {
				sort.Strings(gotAllowed)
			}

			if tt.wantDeniedSorted {
				sort.Strings(gotDenied)
			}

			assert.Equal(t, tt.wantAllowed, gotAllowed)
			assert.Equal(t, tt.wantDenied, gotDenied)
		})
	}
}
