package gql

import (
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
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
			name:     "manual plugin schedules claim before runtime starts",
			op:       mutationOp("RunWithAnyName", "runPluginTask"),
			wantSkip: true,
		},
		{
			name:     "plugin enable persists table migration intents independently",
			op:       mutationOp("Enable", "enablePlugin"),
			wantSkip: true,
		},
		{
			name:     "plugin upload persists table migration intents independently",
			op:       mutationOp("Upload", "installPlugin"),
			wantSkip: true,
		},
		{
			name:     "market install persists table migration intents independently",
			op:       mutationOp("Market", "installPluginFromMarket"),
			wantSkip: true,
		},
		{
			name:     "plugin delete queues table cleanup in the enclosing transaction",
			op:       mutationOp("Delete", "deletePlugin"),
			wantSkip: false,
		},
		{
			name:     "syncChannelModels uses the enclosing transaction",
			op:       mutationOp("UpdateModels", "syncChannelModels"),
			wantSkip: false,
		},
		{
			name:     "syncChannelModels and an ordinary mutation share the transaction",
			op:       mutationOp("MixedModelSync", "syncChannelModels", "disableChannelAPIKey"),
			wantSkip: false,
		},
		{
			name:     "empty selection set does not skip",
			op:       mutationOp("Empty"),
			wantSkip: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := skipGraphqlTransaction(&graphql.OperationContext{Operation: tc.op}); got != tc.wantSkip {
				t.Fatalf("skipGraphqlTransaction(%s) = %v, want %v", tc.op.Name, got, tc.wantSkip)
			}
		})
	}
}

func TestCatalogImportFragmentTransactions(t *testing.T) {
	for _, test := range []struct {
		fields  string
		enabled bool
		skip    bool
	}{
		{"applied: applyCatalogImport", true, true},
		{"... on Mutation { applyCatalogImport }", true, true},
		{"...Import @include(if:$enabled)", true, true},
		{"...Import @include(if:$enabled)", false, false},
		{"applyCatalogImport @skip(if:$enabled) updateModel", true, false},
	} {
		doc, err := parser.ParseQuery(&ast.Source{Input: "mutation Test($enabled:Boolean!) { " + test.fields + " } fragment Import on Mutation { applyCatalogImport }"})
		require.NoError(t, err)
		oc := &graphql.OperationContext{Doc: doc, Operation: doc.Operations[0], Variables: map[string]any{"enabled": test.enabled}}
		require.Equal(t, test.skip, skipGraphqlTransaction(oc), test.fields)
	}
}
