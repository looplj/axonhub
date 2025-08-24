package scopes

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
)

type (
	// QueryPolicy will deny if the query rule returns nil or privacy.Skip.
	QueryPolicy []privacy.QueryRule

	// MutationPolicy will deny if the mutation rule returns nil or privacy.Skip.
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
	for idx, policy := range policies {
		decision := policy.EvalQuery(ctx, q)

		log.Debug(ctx, "query policy decision", log.Int("policy_index", idx), log.Any("decision", decision))

		switch {
		case decision == nil || errors.Is(decision, privacy.Skip):
			continue
		default:
			return decision
		}
	}

	return privacy.Deny
}

// EvalMutation evaluates a mutation against a mutation policy.
// Like the ent privacy package, but will deny by default.
func (policies MutationPolicy) EvalMutation(ctx context.Context, m ent.Mutation) error {
	for idx, policy := range policies {
		decision := policy.EvalMutation(ctx, m)

		log.Debug(ctx, "mutation policy decision", log.Int("policy_index", idx), log.Any("decision", decision))

		switch {
		case decision == nil || errors.Is(decision, privacy.Skip):
			continue
		default:
			return decision
		}
	}

	return privacy.Deny
}
