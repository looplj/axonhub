package gql

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestSkipMutationTransaction(t *testing.T) {
	tests := []struct {
		name string
		op   *ast.OperationDefinition
		want bool
	}{
		{
			name: "named test channel operation",
			op:   &ast.OperationDefinition{Name: "TestChannel", Operation: ast.Mutation},
			want: true,
		},
		{
			name: "custom operation name with bulk import field",
			op: &ast.OperationDefinition{
				Name:         "ImportFromBackup",
				Operation:    ast.Mutation,
				SelectionSet: ast.SelectionSet{&ast.Field{Name: "bulkImportChannels"}},
			},
			want: true,
		},
		{
			name: "anonymous operation with bulk import field",
			op: &ast.OperationDefinition{
				Operation:    ast.Mutation,
				SelectionSet: ast.SelectionSet{&ast.Field{Name: "bulkImportChannels"}},
			},
			want: true,
		},
		{
			name: "aliased bulk import field",
			op: &ast.OperationDefinition{
				Name:         "ImportAliased",
				Operation:    ast.Mutation,
				SelectionSet: ast.SelectionSet{&ast.Field{Alias: "imported", Name: "bulkImportChannels"}},
			},
			want: true,
		},
		{
			name: "bulk import in inline fragment",
			op: &ast.OperationDefinition{
				Name:      "ImportInline",
				Operation: ast.Mutation,
				SelectionSet: ast.SelectionSet{&ast.InlineFragment{
					SelectionSet: ast.SelectionSet{&ast.Field{Name: "bulkImportChannels"}},
				}},
			},
			want: true,
		},
		{
			name: "bulk import in fragment spread",
			op: &ast.OperationDefinition{
				Name:      "ImportFragment",
				Operation: ast.Mutation,
				SelectionSet: ast.SelectionSet{&ast.FragmentSpread{
					Definition: &ast.FragmentDefinition{
						SelectionSet: ast.SelectionSet{&ast.Field{Name: "bulkImportChannels"}},
					},
				}},
			},
			want: true,
		},
		{
			name: "ordinary mutation",
			op: &ast.OperationDefinition{
				Name:         "CreateChannel",
				Operation:    ast.Mutation,
				SelectionSet: ast.SelectionSet{&ast.Field{Name: "createChannel"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, skipMutationTransaction(tt.op))
		})
	}
}
