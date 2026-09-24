//go:build ignore

package main

import (
	"log"
	"path/filepath"

	"entgo.io/contrib/entgql"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/looplj/axonhub/internal/pkg/xfile"
)

func main() {
	customPaginationTemplate, err := gen.NewTemplate(entgql.PaginationTemplate.Name()).
		Funcs(entgql.TemplateFuncs).
		ParseFiles(filepath.Join(xfile.CurDir(), "template", "pagination.tmpl"))
	if err != nil {
		log.Fatalf("creating custom entgql pagination template: %v", err)
	}

	templates := make([]*gen.Template, len(entgql.AllTemplates))
	copy(templates, entgql.AllTemplates)
	for i, tmpl := range templates {
		if tmpl.Name() == entgql.PaginationTemplate.Name() {
			templates[i] = customPaginationTemplate
			break
		}
	}

	ex, err := entgql.NewExtension(
		entgql.WithTemplates(templates...),
		// entgql.WithConfigPath("../graph/gqlgen.yml"),
		// entgql.WithConfigPath("./graph/gqlgen.yml"),
		entgql.WithConfigPath("gqlgen.yml"),
		entgql.WithSchemaGenerator(),
		// entgql.WithSchemaPath("../graph/ent.graphql"),
		// entgql.WithSchemaPath("./graph/ent.graphql"),
		entgql.WithSchemaPath("ent.graphql"),
		entgql.WithWhereInputs(true),
		entgql.WithNodeDescriptor(true),
		entgql.WithRelaySpec(true),
	)
	if err != nil {
		log.Fatalf("creating entgql extension: %v", err)
	}
	opts := []entc.Option{
		entc.FeatureNames("intercept", "schema/snapshot", "sql/upsert", "sql/modifier", "entql", "privacy"),
		entc.Extensions(ex),
	}
	// rt := reflect.TypeOf(objects.GUID{})
	if err := entc.Generate(filepath.Join(xfile.CurDir(), "schema"), &gen.Config{
		IDType: field.Int("id").Annotations(
			entgql.Skip(entgql.SkipWhereInput),
		).Descriptor().Info,
		// IDType: &field.TypeInfo{
		// 	Type:    field.TypeUUID,
		// 	Ident:   rt.String(),
		// 	PkgPath: rt.PkgPath(),
		// },
	}, opts...); err != nil {
		log.Fatalf("running ent codegen: %v", err)
	}
}
