package schema

import (
	"entgo.io/contrib/entgql"
	"github.com/vektah/gqlparser/v2/ast"
)

func forceResolver() entgql.Directive {
	return entgql.NewDirective("goField", &ast.Argument{
		Name: "forceResolver",
		Value: &ast.Value{
			Raw:  "true",
			Kind: ast.BooleanValue,
		},
	})
}
