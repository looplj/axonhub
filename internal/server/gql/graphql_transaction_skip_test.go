package gql

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

func mutationOp(name string, fields ...string) *ast.OperationDefinition {
	op := &ast.OperationDefinition{
		Operation: "mutation",
		Name:      name,
	}
	for _, f := range fields {
		op.SelectionSet = append(op.SelectionSet, &ast.Field{Name: f})
	}
	return op
}

func TestSkipGraphqlTransaction(t *testing.T) {
	cases := []struct {
		name     string
		op       *ast.OperationDefinition
		wantSkip bool
	}{
		{
			name:     "checkProviderQuotas skips under any client-chosen operation name",
			op:       mutationOp("AnythingIWant", "checkProviderQuotas"),
			wantSkip: true,
		},
		{
			name:     "checkProviderQuotas with the canonical name",
			op:       mutationOp("CheckProviderQuotas", "checkProviderQuotas"),
			wantSkip: true,
		},
		{
			name:     "resetChannelQuotaNow skips",
			op:       mutationOp("Whatever", "resetChannelQuotaNow"),
			wantSkip: true,
		},
		{
			name:     "quota mutation nested among other fields",
			op:       mutationOp("Mixed", "disableChannelAPIKey", "checkProviderQuotas"),
			wantSkip: true,
		},
		{
			name:     "ordinary channel mutation does not skip",
			op:       mutationOp("DisableChannelAPIKey", "disableChannelAPIKey"),
			wantSkip: false,
		},
		{
			name:     "TestChannel operation skips via name match",
			op:       mutationOp("TestChannel", "testChannel"),
			wantSkip: true,
		},
		{
			name:     "bulkImportChannels skips",
			op:       mutationOp("Import", "bulkImportChannels"),
			wantSkip: true,
		},
		{
			name:     "empty selection set does not skip",
			op:       mutationOp("Empty"),
			wantSkip: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := skipGraphqlTransaction(tc.op); got != tc.wantSkip {
				t.Fatalf("skipGraphqlTransaction(%s) = %v, want %v", tc.op.Name, got, tc.wantSkip)
			}
		})
	}
}
