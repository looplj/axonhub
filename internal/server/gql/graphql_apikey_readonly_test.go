package gql

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/looplj/axonhub/internal/authz"
)

func TestAPIKeyReadOnly(t *testing.T) {
	tests := []struct {
		name       string
		principal  authz.PrincipalType
		operation  ast.Operation
		wantCalled bool
	}{
		{"user query allowed", authz.PrincipalTypeUser, ast.Query, true},
		{"user mutation allowed", authz.PrincipalTypeUser, ast.Mutation, true},
		{"system mutation allowed", authz.PrincipalTypeSystem, ast.Mutation, true},
		{"api key query allowed", authz.PrincipalTypeAPIKey, ast.Query, true},
		{"api key mutation denied", authz.PrincipalTypeAPIKey, ast.Mutation, false},
		{"api key subscription denied", authz.PrincipalTypeAPIKey, ast.Subscription, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, err := authz.WithPrincipal(context.Background(), authz.Principal{Type: tc.principal})
			require.NoError(t, err)
			ctx = graphql.WithOperationContext(ctx, &graphql.OperationContext{
				Operation: &ast.OperationDefinition{Operation: tc.operation},
			})

			called := false
			next := func(context.Context) graphql.ResponseHandler {
				called = true

				return graphql.OneShot(&graphql.Response{Data: []byte(`{}`)})
			}

			response := apiKeyReadOnly(ctx, next)
			require.Equal(t, tc.wantCalled, called)

			if !tc.wantCalled {
				require.NotEmpty(t, response(ctx).Errors)
			}
		})
	}
}
