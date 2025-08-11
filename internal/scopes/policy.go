package scopes

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

type (
	QueryPolicy    []privacy.QueryRule
	MutationPolicy []privacy.MutationRule
)

// Policy groups query and mutation policies.
type Policy struct {
	Query    QueryPolicy
	Mutation MutationPolicy
}

// EvalQuery evaluates a query against the policy's query rules.
func (p Policy) EvalQuery(ctx context.Context, q ent.Query) error {
	return p.Query.EvalQuery(ctx, q)
}

// EvalMutation evaluates a mutation against the policy's mutation rules.
func (p Policy) EvalMutation(ctx context.Context, m ent.Mutation) error {
	return p.Mutation.EvalMutation(ctx, m)
}

// EvalQuery evaluates a query against a query policy.
// Like the ent privacy package, but will deny by default.
func (policies QueryPolicy) EvalQuery(ctx context.Context, q ent.Query) error {
	for _, policy := range policies {
		switch decision := policy.EvalQuery(ctx, q); {
		case decision == nil || errors.Is(decision, privacy.Skip):
		default:
			return decision
		}
	}

	return privacy.Deny
}

// EvalMutation evaluates a mutation against a mutation policy.
// Like the ent privacy package, but will deny by default.
func (policies MutationPolicy) EvalMutation(ctx context.Context, m ent.Mutation) error {
	for _, policy := range policies {
		switch decision := policy.EvalMutation(ctx, m); {
		case decision == nil || errors.Is(decision, privacy.Skip):
		default:
			return decision
		}
	}

	return privacy.Deny
}
