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
	ex, err := entgql.NewExtension(
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
		entc.FeatureNames("intercept", "schema/snapshot", "sql/upsert"),
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
