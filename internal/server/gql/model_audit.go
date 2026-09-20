package gql

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// A hyphen cannot appear in a GraphQL alias, so user-selected, filtered execution
// pages cannot overwrite the private full-set audit edge.
const modelAuditExecutionsEdge = "model-audit"

func withModelAuditExecutions(query *ent.RequestQuery) *ent.RequestQuery {
	return query.WithNamedExecutions(modelAuditExecutionsEdge, func(executions *ent.RequestExecutionQuery) {
		executions.Select(
			requestexecution.FieldID, requestexecution.FieldRequestID, requestexecution.FieldProjectID,
			requestexecution.FieldStatus,
			requestexecution.FieldOutboundModelID, requestexecution.FieldUpstreamModelID, requestexecution.FieldUpstreamModelIds,
		).Order(ent.Desc(requestexecution.FieldID)).Where(func(s *sql.Selector) {
			// The parent request query enforces authorization. Keep the execution in
			// that request's project too, including for cross-project admin lists.
			parent := sql.Table(request.Table)
			s.Where(sql.Exists(sql.Select(parent.C(request.FieldID)).From(parent).Where(sql.And(
				sql.ColumnsEQ(parent.C(request.FieldID), s.C(requestexecution.FieldRequestID)),
				sql.ColumnsEQ(parent.C(request.FieldProjectID), s.C(requestexecution.FieldProjectID)),
			))))
		})
	})
}

// Respect fragments, aliases and @skip/@include while only loading audit
// metadata when the requests connection actually selects modelAudit.
func requestModelAuditSelected(ctx context.Context) bool {
	field := graphql.GetFieldContext(ctx)
	if field == nil {
		return false
	}
	operation := graphql.GetOperationContext(ctx)
	for _, edge := range graphql.CollectFields(operation, field.Field.Selections, []string{"RequestConnection"}) {
		if edge.Name != "edges" {
			continue
		}
		for _, node := range graphql.CollectFields(operation, edge.Selections, []string{"RequestEdge"}) {
			if node.Name != "node" {
				continue
			}
			for _, selected := range graphql.CollectFields(operation, node.Selections, []string{"Request", "Node"}) {
				if selected.Name == "modelAudit" {
					return true
				}
			}
		}
	}
	return false
}

func (r *requestResolver) resolveRequestModelAudit(ctx context.Context, obj *ent.Request) (*objects.RequestModelAudit, error) {
	executions, err := obj.NamedExecutions(modelAuditExecutionsEdge)
	if ent.IsNotLoaded(err) {
		// Node/detail queries may not have loaded the private edge. Query through
		// Request again so its project and personal-key privacy rules still apply.
		parent, loadErr := withModelAuditExecutions(r.client.Request.Query().
			Where(request.ID(obj.ID))).Select(request.FieldID, request.FieldProjectID).Only(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		executions, err = parent.NamedExecutions(modelAuditExecutionsEdge)
	}
	if err != nil {
		return nil, err
	}
	return biz.AuditRequestModels(executions), nil
}
