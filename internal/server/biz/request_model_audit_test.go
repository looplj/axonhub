package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
)

func TestAuditRequestModels(t *testing.T) {
	tests := []struct {
		name                              string
		executions                        []*ent.RequestExecution
		status                            objects.ModelAuditStatus
		compared, unknown, conflicts      int
		upstream, mismatched, conflicting []string
	}{
		{name: "no executions", status: objects.ModelAuditUnknown},
		{name: "routing name cannot substitute for final sent name", executions: []*ent.RequestExecution{
			{ModelID: "routed", UpstreamModelID: "routed"},
		}, status: objects.ModelAuditUnknown, unknown: 1, upstream: []string{"routed"}},
		{name: "override matches actual sent name", executions: []*ent.RequestExecution{
			{ModelID: "routed", OutboundModelID: "sent", UpstreamModelID: "sent"},
		}, status: objects.ModelAuditMatched, compared: 1, upstream: []string{"sent"}},
		{name: "override reveals false match", executions: []*ent.RequestExecution{
			{ModelID: "routed", OutboundModelID: "sent", UpstreamModelID: "routed"},
		}, status: objects.ModelAuditMismatched, compared: 1, upstream: []string{"routed"}, mismatched: []string{"routed"}},
		{name: "retry cross matching and partial unknown", executions: []*ent.RequestExecution{
			{OutboundModelID: "a", UpstreamModelID: "a"},
			{OutboundModelID: "b", UpstreamModelID: "a"},
			{OutboundModelID: "c"},
		}, status: objects.ModelAuditMismatched, compared: 2, unknown: 1, upstream: []string{"a"}, mismatched: []string{"a"}},
		{name: "partial metadata cannot establish all matched", executions: []*ent.RequestExecution{
			{OutboundModelID: "a", UpstreamModelID: "a"},
			{OutboundModelID: "a", UpstreamModelID: "   "},
		}, status: objects.ModelAuditUnknown, compared: 1, unknown: 1, upstream: []string{"a"}},
		{name: "exact names retain whitespace case and version", executions: []*ent.RequestExecution{
			{OutboundModelID: "a", UpstreamModelID: "A"},
			{OutboundModelID: "a", UpstreamModelID: " a "},
			{OutboundModelID: "a", UpstreamModelID: "a-2026-09-19"},
		}, status: objects.ModelAuditMismatched, compared: 3, upstream: []string{"A", " a ", "a-2026-09-19"}, mismatched: []string{"A", " a ", "a-2026-09-19"}},
		{
			name: "stream changes preserve conflict and other mismatches", executions: []*ent.RequestExecution{
				{OutboundModelID: "a", UpstreamModelID: "a", UpstreamModelIds: []string{"a", "b"}},
				{OutboundModelID: "c", UpstreamModelID: "d"},
				{ModelID: "historical"},
			}, status: objects.ModelAuditConflicting, compared: 2, unknown: 1, conflicts: 1,
			upstream: []string{"a", "b", "d"}, mismatched: []string{"b", "d"}, conflicting: []string{"a", "b"},
		},
		{name: "stream conflict is known even when sent name is missing", executions: []*ent.RequestExecution{
			{UpstreamModelIds: []string{"a", "b"}},
		}, status: objects.ModelAuditConflicting, unknown: 1, conflicts: 1, upstream: []string{"a", "b"}, conflicting: []string{"a", "b"}},
		{name: "different retries and repeated metadata are not stream conflicts", executions: []*ent.RequestExecution{
			{OutboundModelID: "a", UpstreamModelID: "a", UpstreamModelIds: []string{"a", "a"}},
			{OutboundModelID: "b", UpstreamModelIds: []string{"b"}},
		}, status: objects.ModelAuditMatched, compared: 2, upstream: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audit := AuditRequestModels(tt.executions)
			require.Equal(t, tt.status, audit.Status)
			require.Equal(t, tt.compared, audit.ComparedCount)
			require.Equal(t, tt.unknown, audit.UnknownCount)
			require.Equal(t, tt.conflicts, audit.ConflictCount)
			require.ElementsMatch(t, tt.upstream, audit.UpstreamModelIds)
			require.ElementsMatch(t, tt.mismatched, audit.MismatchedModelIds)
			require.ElementsMatch(t, tt.conflicting, audit.ConflictingModelIds)
			require.NotNil(t, audit.UpstreamModelIds)
			require.NotNil(t, audit.MatchedUpstreamIds)
			require.NotNil(t, audit.MismatchedModelIds)
			require.NotNil(t, audit.ConflictingModelIds)
		})
	}
}

func TestAuditRequestModelsSuccessfulRetries(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status requestexecution.Status
		want   objects.ModelAuditStatus
	}{
		{"failed retry", requestexecution.StatusFailed, objects.ModelAuditMatched},
		{"canceled retry", requestexecution.StatusCanceled, objects.ModelAuditMatched},
		{"successful execution with missing metadata", requestexecution.StatusCompleted, objects.ModelAuditUnknown},
		{"pending execution", requestexecution.StatusPending, objects.ModelAuditUnknown},
		{"active execution", requestexecution.StatusProcessing, objects.ModelAuditUnknown},
		{"legacy execution without status", "", objects.ModelAuditUnknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			matched := &ent.RequestExecution{Status: requestexecution.StatusCompleted, OutboundModelID: "sent", UpstreamModelID: "sent"}
			unknown := &ent.RequestExecution{Status: tt.status, OutboundModelID: "sent"}
			for _, executions := range [][]*ent.RequestExecution{{unknown, matched}, {matched, unknown}} {
				audit := AuditRequestModels(executions)
				require.Equal(t, tt.want, audit.Status)
				require.Equal(t, 1, audit.UnknownCount, "retry evidence stays available")
				require.Equal(t, 1, audit.ComparedCount)
			}
		})
	}
	for _, tt := range []struct {
		name       string
		executions []*ent.RequestExecution
		want       objects.ModelAuditStatus
	}{
		{"no successful execution", []*ent.RequestExecution{
			{Status: requestexecution.StatusFailed, OutboundModelID: "sent", UpstreamModelID: "sent"},
			{Status: requestexecution.StatusFailed},
		}, objects.ModelAuditUnknown},
		{"final success is unknown", []*ent.RequestExecution{
			{Status: requestexecution.StatusFailed, OutboundModelID: "sent", UpstreamModelID: "sent"},
			{Status: requestexecution.StatusCompleted, OutboundModelID: "sent"},
		}, objects.ModelAuditUnknown},
		{"successful retry preserves known mismatch", []*ent.RequestExecution{
			{Status: requestexecution.StatusFailed, OutboundModelID: "sent", UpstreamModelID: "different"},
			{Status: requestexecution.StatusCompleted, OutboundModelID: "sent", UpstreamModelID: "sent"},
		}, objects.ModelAuditMismatched},
		{"successful retry preserves stream conflict", []*ent.RequestExecution{
			{Status: requestexecution.StatusFailed, UpstreamModelIds: []string{"sent", "different"}},
			{Status: requestexecution.StatusCompleted, OutboundModelID: "sent", UpstreamModelID: "sent"},
		}, objects.ModelAuditConflicting},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, AuditRequestModels(tt.executions).Status)
		})
	}
}

func TestAuditRequestModelsMatchedUpstreamIds(t *testing.T) {
	for _, status := range []requestexecution.Status{
		requestexecution.StatusFailed, requestexecution.StatusCanceled,
		requestexecution.StatusPending, requestexecution.StatusProcessing, "",
	} {
		t.Run(string(status), func(t *testing.T) {
			retry := &ent.RequestExecution{Status: status, OutboundModelID: "glm-5.3:free", UpstreamModelID: "glm-5.3:free"}
			success := &ent.RequestExecution{Status: requestexecution.StatusCompleted, OutboundModelID: "glm-5.3", UpstreamModelID: "glm-5.3", UpstreamModelIds: []string{"glm-5.3"}}
			for _, executions := range [][]*ent.RequestExecution{{retry, retry, retry, success}, {success, retry, retry, retry}} {
				audit := AuditRequestModels(executions)
				require.Equal(t, objects.ModelAuditMatched, audit.Status)
				require.Equal(t, []string{"glm-5.3"}, audit.MatchedUpstreamIds)
				require.ElementsMatch(t, []string{"glm-5.3", "glm-5.3:free"}, audit.UpstreamModelIds)
				require.Equal(t, 4, audit.ComparedCount)
			}
			require.Empty(t, AuditRequestModels([]*ent.RequestExecution{retry}).MatchedUpstreamIds)
		})
	}
	for _, execution := range []*ent.RequestExecution{
		{Status: requestexecution.StatusCompleted, OutboundModelID: "sent"},
		{Status: requestexecution.StatusCompleted, UpstreamModelID: "sent"},
		{Status: requestexecution.StatusCompleted, OutboundModelID: "sent", UpstreamModelID: "different"},
	} {
		require.Empty(t, AuditRequestModels([]*ent.RequestExecution{execution}).MatchedUpstreamIds)
	}
}
