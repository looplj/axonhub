package biz

import (
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
)

// AuditRequestModels compares each execution with its own final wire model.
// Callers must supply the complete execution set, not a display page. ModelID is
// deliberately not a fallback: historical routing names do not prove what was sent.
func AuditRequestModels(executions []*ent.RequestExecution) *objects.RequestModelAudit {
	audit := &objects.RequestModelAudit{
		Status:              objects.ModelAuditUnknown,
		MatchedUpstreamIds:  []string{},
		UpstreamModelIds:    []string{},
		MismatchedModelIds:  []string{},
		ConflictingModelIds: []string{},
	}
	hasCompletedComparison := false
	blockingUnknownCount := 0
	for _, execution := range executions {
		models := lo.Filter(lo.Uniq(append([]string{execution.UpstreamModelID}, execution.UpstreamModelIds...)), func(model string, _ int) bool {
			return strings.TrimSpace(model) != ""
		})
		audit.UpstreamModelIds = append(audit.UpstreamModelIds, models...)
		if len(models) > 1 {
			audit.ConflictCount++
			audit.ConflictingModelIds = append(audit.ConflictingModelIds, models...)
		}
		if strings.TrimSpace(execution.OutboundModelID) == "" || len(models) == 0 {
			audit.UnknownCount++
			if execution.Status != requestexecution.StatusFailed && execution.Status != requestexecution.StatusCanceled {
				blockingUnknownCount++
			}
			continue
		}
		audit.ComparedCount++
		hasCompletedComparison = hasCompletedComparison || execution.Status == requestexecution.StatusCompleted
		for _, model := range models {
			if model == execution.OutboundModelID {
				if execution.Status == requestexecution.StatusCompleted {
					audit.MatchedUpstreamIds = append(audit.MatchedUpstreamIds, model)
				}
			} else {
				audit.MismatchedModelIds = append(audit.MismatchedModelIds, model)
			}
		}
	}
	audit.UpstreamModelIds = lo.Uniq(audit.UpstreamModelIds)
	audit.MatchedUpstreamIds = lo.Uniq(audit.MatchedUpstreamIds)
	audit.MismatchedModelIds = lo.Uniq(audit.MismatchedModelIds)
	audit.ConflictingModelIds = lo.Uniq(audit.ConflictingModelIds)
	switch {
	case audit.ConflictCount > 0:
		audit.Status = objects.ModelAuditConflicting
	case len(audit.MismatchedModelIds) > 0:
		audit.Status = objects.ModelAuditMismatched
	case len(executions) > 0 && (audit.UnknownCount == 0 || (hasCompletedComparison && blockingUnknownCount == 0)):
		// Failed/canceled retries often have no model metadata. They must not
		// obscure a successful match, but their counts and any known anomalies
		// remain visible. Successful, active and legacy unknowns still block it.
		audit.Status = objects.ModelAuditMatched
	}
	return audit
}
