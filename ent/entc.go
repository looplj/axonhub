//go:build ignore

package main

import (
	"log"

	"entgo.io/contrib/entgql"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
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
	if err := entc.Generate("../ent/schema", &gen.Config{
		// IDType: field.Int("id").Descriptor().Info,
	}, opts...); err != nil {
		log.Fatalf("running ent codegen: %v", err)
	}
}
