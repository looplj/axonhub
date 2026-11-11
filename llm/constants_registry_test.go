package llm

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

// Inspect the actual enum declarations, not another hand-maintained expectation.
// Adding a constant to constants.go without registering it must fail this test.
func TestEnumRegistriesCoverDeclaredConstants(t *testing.T) {
	fset := token.NewFileSet()
	source, err := parser.ParseFile(fset, "constants.go", nil, 0)
	require.NoError(t, err)
	pkg, err := (&types.Config{}).Check("llm", fset, []*ast.File{source}, nil)
	require.NoError(t, err)
	checkEnumRegistry(t, pkg, "RequestType", AllRequestTypes())
	checkEnumRegistry(t, pkg, "APIFormat", AllAPIFormats())
}

func checkEnumRegistry[T ~string](t *testing.T, pkg *types.Package, typeName string, registered []T) {
	t.Helper()
	enumType := pkg.Scope().Lookup(typeName).Type()
	declared := map[string]bool{}
	for _, name := range pkg.Scope().Names() {
		c, ok := pkg.Scope().Lookup(name).(*types.Const)
		if ok && types.Identical(c.Type(), enumType) {
			declared[constant.StringVal(c.Val())] = true
		}
	}
	seen := map[string]bool{}
	for _, value := range registered {
		require.False(t, seen[string(value)], "duplicate %s value %q", typeName, value)
		require.True(t, declared[string(value)], "undeclared %s value %q", typeName, value)
		seen[string(value)] = true
	}
	require.Equal(t, declared, seen, "update All%ss() when changing constants.go", typeName)
}

func TestEnumRegistriesReturnIndependentSlices(t *testing.T) {
	requests := AllRequestTypes()
	requests[0] = "changed"
	formats := AllAPIFormats()
	formats[0] = "changed"
	require.Equal(t, RequestTypeChat, AllRequestTypes()[0])
	require.Equal(t, APIFormatOpenAIChatCompletion, AllAPIFormats()[0])
}
